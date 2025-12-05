package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Memory is the data source name for a private in memory database, which is
// what the tests and the dry run mode use.
const Memory = ":memory:"

// OpenRaw connects to a database without migrating it. It is for a caller
// that has to know what version the database was at before anything touched
// it, which is exactly one caller and is not worth exporting the whole
// connection setup for.
func OpenRaw(dsn string) (*sql.DB, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, fmt.Errorf("%w: no data source given", ErrMigration)
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("%w: opening %s: %v", ErrMigration, dsn, err)
	}
	db.SetMaxOpenConns(1)
	return db, nil
}

// Open connects to a database, turns on the settings this system depends on,
// and migrates it to the latest schema.
func Open(ctx context.Context, dsn string) (*sql.DB, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, fmt.Errorf("%w: no data source given", ErrMigration)
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("%w: opening %s: %v", ErrMigration, dsn, err)
	}
	// An in memory database lives in its connection, so a pool of more than
	// one hands out connections that cannot see each other's tables.
	if dsn == Memory {
		db.SetMaxOpenConns(1)
	} else {
		db.SetMaxOpenConns(8)
	}
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(time.Hour)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("%w: reaching %s: %v", ErrMigration, dsn, err)
	}
	for _, pragma := range []string{
		`PRAGMA foreign_keys = ON`,
		`PRAGMA busy_timeout = 5000`,
	} {
		if _, err := db.ExecContext(ctx, pragma); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("%w: %s: %v", ErrMigration, pragma, err)
		}
	}
	if _, err := Migrate(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// ForeignKeysOn reports whether foreign key enforcement is actually on, which
// sqlite leaves off by default and silently ignores if the pragma is set on
// the wrong connection.
func ForeignKeysOn(ctx context.Context, db *sql.DB) (bool, error) {
	var on int
	if err := db.QueryRowContext(ctx, `PRAGMA foreign_keys`).Scan(&on); err != nil {
		return false, fmt.Errorf("%w: reading the foreign key pragma: %v", ErrMigration, err)
	}
	return on == 1, nil
}

// Tables returns the tables in the database, sorted.
func Tables(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT name FROM sqlite_master WHERE type = 'table' ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("%w: listing tables: %v", ErrMigration, err)
	}
	defer func() { _ = rows.Close() }()

	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("%w: reading a table name: %v", ErrMigration, err)
		}
		out = append(out, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%w: listing tables: %v", ErrMigration, err)
	}
	return out, nil
}
