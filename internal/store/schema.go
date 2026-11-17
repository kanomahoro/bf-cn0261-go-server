package store

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// The structural contract lives in schema_expected.go; regenerate it with
// tools/db-schema-expected.sh after an intentional schema change. The revision
// number the baseline writes into schema_version is a label only: startup never
// compares it, so data-only changes need no action.

// SchemaStructureError lists the ways the live database differs from the
// structure this build expects.
type SchemaStructureError struct {
	Problems []string
	Checked  int
}

func (e *SchemaStructureError) Error() string {
	const shownMax = 6
	shown := e.Problems
	if len(shown) > shownMax {
		shown = shown[:shownMax]
	}
	message := fmt.Sprintf("database structure does not match this build (%d problem(s) across %d expected columns): %s",
		len(e.Problems), e.Checked, strings.Join(shown, "; "))
	if len(e.Problems) > shownMax {
		message += fmt.Sprintf("; and %d more", len(e.Problems)-shownMax)
	}
	return message + "; rebuild it with \"bash database/migrate.sh --force\""
}

// integerDisplayWidth matches the "(N)" part of the integer types MariaDB and
// MySQL report differently across versions (int(10) vs int(11) vs plain int).
var integerDisplayWidth = regexp.MustCompile(`^(tinyint|smallint|mediumint|int|bigint)\(\d+\)(.*)$`)

// normalizeColumnType drops integer display widths so a newer server reporting
// int(11) instead of int(10), or MySQL dropping the width, is not treated as a
// structural change.
func normalizeColumnType(columnType string) string {
	columnType = strings.ToLower(strings.TrimSpace(columnType))
	if match := integerDisplayWidth.FindStringSubmatch(columnType); match != nil {
		return match[1] + match[2]
	}
	return columnType
}

// CheckSchema compares the live database structure with expectedSchema. Data is
// never inspected, so content updates need no migration. Extra tables and extra
// columns are tolerated (they do not break existing queries); a missing table or
// column, or a changed type, nullability, key or auto-increment flag, is not.
func (db *DB) CheckSchema() error {
	ctx, cancel := db.ctx()
	defer cancel()

	rows, err := db.sql.QueryContext(ctx,
		`SELECT TABLE_NAME, COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE, COLUMN_KEY, EXTRA
		   FROM information_schema.columns
		  WHERE table_schema = DATABASE()`)
	if err != nil {
		return err
	}
	defer rows.Close()

	actual := make(map[string]string)
	tables := make(map[string]bool)
	for rows.Next() {
		var table, column, columnType, nullable, key, extra string
		if err := rows.Scan(&table, &column, &columnType, &nullable, &key, &extra); err != nil {
			return err
		}
		tables[table] = true
		actual[table+"|"+column] = normalizeColumnType(columnType) + "|" + nullable + "|" + key + "|" + extra
	}
	if err := rows.Err(); err != nil {
		return err
	}

	var problems []string
	missingTables := make(map[string]bool)
	checked := 0
	for _, line := range strings.Split(expectedSchema, "\n") {
		fields := strings.Split(line, "|")
		if len(fields) != 6 {
			continue // blank line or stray comment
		}
		table, column := fields[0], fields[1]
		checked++
		if !tables[table] {
			if !missingTables[table] {
				missingTables[table] = true
				problems = append(problems, "missing table "+table)
			}
			continue
		}
		got, ok := actual[table+"|"+column]
		if !ok {
			problems = append(problems, "missing column "+table+"."+column)
			continue
		}
		want := normalizeColumnType(fields[2]) + "|" + fields[3] + "|" + fields[4] + "|" + fields[5]
		if got != want {
			problems = append(problems, fmt.Sprintf("column %s.%s: expected %s, found %s", table, column, want, got))
		}
	}

	if len(problems) > 0 {
		sort.Strings(problems)
		return &SchemaStructureError{Problems: problems, Checked: checked}
	}
	return nil
}
