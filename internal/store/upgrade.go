package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

// SchemaRevision is the database schema revision this build expects. Bump it
// together with a new file in database/upgrades: the startup check refuses a
// database whose revision differs, and --auto-upgrade applies the missing steps
// before the check runs again.
const SchemaRevision uint32 = 3

// SchemaMinRevision is the oldest revision an automatic upgrade may start from.
// Revision 1 is a database built by the fourteen historical migrations (or one
// that predates schema_version entirely).
const SchemaMinRevision uint32 = 1

// SchemaRevisionError reports a database this build cannot use.
type SchemaRevisionError struct {
	Have uint32
	Want uint32
}

func (e *SchemaRevisionError) Error() string {
	if e.Have == 0 {
		return fmt.Sprintf("database schema revision unknown (no schema_version row): this build needs revision %d; run \"bash database/upgrade.sh\"", e.Want)
	}
	return fmt.Sprintf("database schema revision %d, this build needs %d; run \"bash database/upgrade.sh\"", e.Have, e.Want)
}

// Revision reports schema_version.revision, or 0 when the table or row is
// missing (a database built by the fourteen historical migrations).
func (db *DB) Revision() (uint32, error) {
	ctx, cancel := db.ctx()
	defer cancel()
	var revision uint32
	err := db.sql.QueryRowContext(ctx, "SELECT revision FROM schema_version WHERE id = 1").Scan(&revision)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		var mysqlErr *mysql.MySQLError
		// 1146: table doesn't exist. A legacy database has no schema_version yet.
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1146 {
			return 0, nil
		}
		return 0, err
	}
	return revision, nil
}

// CheckSchema compares the database revision with the one this build requires.
// It replaces the old contract that pinned fourteen rows in schema_migrations:
// the database now carries a single revision number, which is what the binary
// is built against.
func (db *DB) CheckSchema() error {
	revision, err := db.Revision()
	if err != nil {
		return err
	}
	if revision != SchemaRevision {
		return &SchemaRevisionError{Have: revision, Want: SchemaRevision}
	}
	return nil
}

// UpgradeStep is one file in database/upgrades.
type UpgradeStep struct {
	Number int
	Target uint32
	Path   string
}

// targetRevisionPattern matches the marker every upgrade file carries, e.g.
// "-- target-revision: 3".
var targetRevisionPattern = regexp.MustCompile(`(?m)^--\s*target-revision:\s*(\d+)\s*$`)

// upgradeFilePattern matches "NNN-description.sql".
var upgradeFilePattern = regexp.MustCompile(`^(\d+)-.+\.sql$`)

// UpgradeSteps lists the upgrade scripts in dir, ordered by their file number.
// Every file must declare its target revision; a file without one is an error
// rather than a silently skipped step.
func UpgradeSteps(dir string) ([]UpgradeStep, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var steps []UpgradeStep
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		match := upgradeFilePattern.FindStringSubmatch(entry.Name())
		if match == nil {
			continue
		}
		number, err := strconv.Atoi(match[1])
		if err != nil {
			return nil, fmt.Errorf("upgrade file %s has an unreadable number: %w", entry.Name(), err)
		}
		path := filepath.Join(dir, entry.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		target := targetRevisionPattern.FindSubmatch(content)
		if target == nil {
			return nil, fmt.Errorf("upgrade file %s has no \"-- target-revision: N\" marker", entry.Name())
		}
		value, err := strconv.Atoi(string(target[1]))
		if err != nil || value <= 0 {
			return nil, fmt.Errorf("upgrade file %s declares an invalid target revision %q", entry.Name(), target[1])
		}
		steps = append(steps, UpgradeStep{Number: number, Target: uint32(value), Path: path})
	}
	sort.Slice(steps, func(i, j int) bool { return steps[i].Number < steps[j].Number })
	for i := 1; i < len(steps); i++ {
		if steps[i].Target <= steps[i-1].Target {
			return nil, fmt.Errorf("upgrade files %d and %d declare non-increasing target revisions",
				steps[i-1].Number, steps[i].Number)
		}
	}
	return steps, nil
}

// ApplyUpgrades applies every step whose target revision is above the database's
// current revision. Statements run one by one; MariaDB commits DDL implicitly,
// so a failure leaves the work done so far in place and the same file can be
// re-run (every step is written to be idempotent).
func (db *DB) ApplyUpgrades(dir string, logf func(string, ...any)) error {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	steps, err := UpgradeSteps(filepath.Join(dir, "upgrades"))
	if err != nil {
		return err
	}
	current, err := db.Revision()
	if err != nil {
		return err
	}
	if current > SchemaRevision {
		return fmt.Errorf("database schema revision %d is newer than this build's %d; upgrade the server", current, SchemaRevision)
	}
	// A database built by the fourteen historical migrations has no
	// schema_version at all and reports 0; it is revision 1 for upgrade purposes.
	start := current
	if start == 0 {
		start = SchemaMinRevision
	}
	if start < SchemaMinRevision {
		return fmt.Errorf("database schema revision %d cannot be upgraded automatically (oldest supported is %d)", current, SchemaMinRevision)
	}
	for _, step := range steps {
		if step.Target <= start {
			continue
		}
		content, err := os.ReadFile(step.Path)
		if err != nil {
			return err
		}
		statements, err := SplitStatements(string(content))
		if err != nil {
			return fmt.Errorf("upgrade %s: %w", filepath.Base(step.Path), err)
		}
		logf("[Database] applying upgrade", "file", filepath.Base(step.Path),
			"targetRevision", step.Target, "statements", len(statements))
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		for i, statement := range statements {
			if _, err := db.sql.ExecContext(ctx, statement); err != nil {
				cancel()
				return fmt.Errorf("upgrade %s: statement %d of %d failed: %w",
					filepath.Base(step.Path), i+1, len(statements), err)
			}
		}
		cancel()
		start = step.Target
	}
	revision, err := db.Revision()
	if err != nil {
		return err
	}
	if revision != SchemaRevision {
		return &SchemaRevisionError{Have: revision, Want: SchemaRevision}
	}
	return nil
}

// SplitStatements splits a SQL script into executable statements. It understands
// single and double quoted strings (backslash and doubled-quote escapes) and
// strips -- line comments outside of them; a ';' inside a string or a comment
// therefore never ends a statement. The upgrade scripts contain no stored
// procedures, so there is no delimiter handling.
func SplitStatements(script string) ([]string, error) {
	var (
		statements []string
		current    strings.Builder
		inSingle   bool
		inDouble   bool
		inComment  bool
	)
	runes := []rune(script)
	for i := 0; i < len(runes); i++ {
		char := runes[i]
		if inComment {
			if char == '\n' {
				inComment = false
				current.WriteRune(char)
			}
			continue
		}
		if inSingle {
			current.WriteRune(char)
			if char == '\\' && i+1 < len(runes) {
				i++
				current.WriteRune(runes[i])
				continue
			}
			if char == '\'' {
				// A doubled quote is an escaped quote, not the end of the string.
				if i+1 < len(runes) && runes[i+1] == '\'' {
					i++
					current.WriteRune(runes[i])
					continue
				}
				inSingle = false
			}
			continue
		}
		if inDouble {
			current.WriteRune(char)
			if char == '\\' && i+1 < len(runes) {
				i++
				current.WriteRune(runes[i])
				continue
			}
			if char == '"' {
				inDouble = false
			}
			continue
		}
		switch {
		case char == '-' && i+1 < len(runes) && runes[i+1] == '-':
			inComment = true
			i++
		case char == '\'':
			inSingle = true
			current.WriteRune(char)
		case char == '"':
			inDouble = true
			current.WriteRune(char)
		case char == ';':
			statement := strings.TrimSpace(current.String())
			if statement != "" {
				statements = append(statements, statement)
			}
			current.Reset()
		default:
			current.WriteRune(char)
		}
	}
	if inSingle || inDouble {
		return nil, fmt.Errorf("script ends inside a quoted string")
	}
	if statement := strings.TrimSpace(current.String()); statement != "" {
		statements = append(statements, statement)
	}
	return statements, nil
}
