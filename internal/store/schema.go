package store

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/go-sql-driver/mysql"
)

// SchemaRevision is the database schema revision this build expects. It is the
// single number the whole database contract is expressed with: the baseline
// (database/baseline.sql) writes it, and startup refuses a database that
// carries anything else.
//
// The project is not deployed yet, so there is exactly one revision and no
// upgrade path: a database that does not match is rebuilt from the baseline.
const SchemaRevision uint32 = 1

// SchemaRevisionError reports a database this build cannot use.
type SchemaRevisionError struct {
	Have uint32
	Want uint32
}

func (e *SchemaRevisionError) Error() string {
	if e.Have == 0 {
		return fmt.Sprintf("database schema revision unknown (no schema_version row): this build needs revision %d; rebuild it with \"bash database/migrate.sh --force\"", e.Want)
	}
	return fmt.Sprintf("database schema revision %d, this build needs %d; rebuild it with \"bash database/migrate.sh --force\"", e.Have, e.Want)
}

// Revision reports schema_version.revision, or 0 when the table or row is
// missing (a database created before schema_version existed).
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
		// 1146: table doesn't exist.
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1146 {
			return 0, nil
		}
		return 0, err
	}
	return revision, nil
}

// CheckSchema compares the database revision with the one this build requires.
// It replaces the old contract that pinned fourteen rows in schema_migrations.
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
