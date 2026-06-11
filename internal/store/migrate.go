// Package store keeps the records on disk.
//
// The schema is a list of numbered migrations that only ever gets appended to,
// and the database records which of them it has run. A schema that is built by
// comparing the code against the database cannot tell a column somebody has
// not added yet from one somebody removed on purpose, which is how a joint use
// system loses an attachment.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
)

// ErrMigration is returned when the schema cannot be brought up to date.
var ErrMigration = errors.New("store: migration failed")

// Migration is one numbered step of the schema.
type Migration struct {
	Version int
	Name    string
	Up      string
}

var migrations = []Migration{
	{Version: 1, Name: "poles", Up: `
CREATE TABLE poles (
    id            TEXT PRIMARY KEY,
    district      TEXT NOT NULL,
    length_ft     INTEGER NOT NULL,
    class         TEXT NOT NULL,
    species       TEXT NOT NULL,
    set_depth     INTEGER NOT NULL,
    installed_in  INTEGER NOT NULL DEFAULT 0,
    owner         TEXT NOT NULL,
    condition     TEXT NOT NULL,
    latitude      REAL NOT NULL DEFAULT 0,
    longitude     REAL NOT NULL DEFAULT 0
);
CREATE INDEX poles_by_owner ON poles (owner);
CREATE INDEX poles_by_district ON poles (district);
`},
	{Version: 2, Name: "attachments", Up: `
CREATE TABLE attachments (
    id            TEXT PRIMARY KEY,
    pole_id       TEXT NOT NULL REFERENCES poles (id),
    owner         TEXT NOT NULL,
    kind          TEXT NOT NULL,
    height        INTEGER NOT NULL,
    authorized    INTEGER NOT NULL DEFAULT 0,
    in_force_from TEXT NOT NULL DEFAULT '',
    in_force_to   TEXT NOT NULL DEFAULT '',
    voltage       INTEGER NOT NULL DEFAULT 0,
    face          TEXT NOT NULL DEFAULT '',
    note          TEXT NOT NULL DEFAULT ''
);
CREATE INDEX attachments_by_pole ON attachments (pole_id);
CREATE INDEX attachments_by_owner ON attachments (owner);
`},
	{Version: 3, Name: "spans", Up: `
CREATE TABLE spans (
    id         TEXT PRIMARY KEY,
    from_pole  TEXT NOT NULL REFERENCES poles (id),
    to_pole    TEXT NOT NULL REFERENCES poles (id),
    length_ft  INTEGER NOT NULL,
    cover      TEXT NOT NULL,
    drop_tenths INTEGER NOT NULL DEFAULT 0,
    dead_end   INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX spans_by_from ON spans (from_pole);
CREATE INDEX spans_by_to ON spans (to_pole);
`},
	{Version: 4, Name: "anchors", Up: `
CREATE TABLE anchors (
    id       TEXT PRIMARY KEY,
    pole_id  TEXT NOT NULL REFERENCES poles (id),
    lead_ft  INTEGER NOT NULL,
    bearing  INTEGER NOT NULL
);
CREATE TABLE guys (
    id        TEXT NOT NULL,
    anchor_id TEXT NOT NULL REFERENCES anchors (id),
    height    INTEGER NOT NULL,
    size      TEXT NOT NULL,
    PRIMARY KEY (anchor_id, id)
);
CREATE INDEX anchors_by_pole ON anchors (pole_id);
`},
	{Version: 5, Name: "applications", Up: `
CREATE TABLE applications (
    id        TEXT PRIMARY KEY,
    applicant TEXT NOT NULL,
    owner     TEXT NOT NULL,
    state     TEXT NOT NULL
);
CREATE TABLE application_poles (
    application_id TEXT NOT NULL REFERENCES applications (id),
    pole_id        TEXT NOT NULL,
    PRIMARY KEY (application_id, pole_id)
);
CREATE TABLE application_events (
    application_id TEXT NOT NULL REFERENCES applications (id),
    seq            INTEGER NOT NULL,
    from_state     TEXT NOT NULL,
    to_state       TEXT NOT NULL,
    on_date        TEXT NOT NULL,
    by_who         TEXT NOT NULL,
    note           TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (application_id, seq)
);
CREATE INDEX applications_by_applicant ON applications (applicant);
`},
	{Version: 6, Name: "surveys", Up: `
CREATE TABLE pole_surveys (
    id          TEXT PRIMARY KEY,
    pole_id     TEXT NOT NULL REFERENCES poles (id),
    taken       TEXT NOT NULL,
    crew        TEXT NOT NULL,
    condition   TEXT NOT NULL,
    ground_line INTEGER NOT NULL DEFAULT 0,
    note        TEXT NOT NULL DEFAULT ''
);
CREATE TABLE survey_measurements (
    survey_id TEXT NOT NULL REFERENCES pole_surveys (id),
    subject   TEXT NOT NULL,
    height    INTEGER NOT NULL,
    method    TEXT NOT NULL,
    crew      TEXT NOT NULL,
    note      TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (survey_id, subject)
);
CREATE INDEX pole_surveys_by_pole ON pole_surveys (pole_id);
`},
	{Version: 7, Name: "billing", Up: `
CREATE TABLE rate_years (
    year          INTEGER PRIMARY KEY,
    formula       TEXT NOT NULL,
    annual_cents  INTEGER NOT NULL,
    published_on  TEXT NOT NULL
);
CREATE TABLE rental_bills (
    number    TEXT PRIMARY KEY,
    year      INTEGER NOT NULL,
    owner     TEXT NOT NULL,
    formula   TEXT NOT NULL,
    total_cents INTEGER NOT NULL,
    issued    TEXT NOT NULL,
    due       TEXT NOT NULL
);
CREATE INDEX rental_bills_by_owner ON rental_bills (owner, year);
`},
	{Version: 8, Name: "change_log", Up: `
CREATE TABLE change_log (
    seq       INTEGER PRIMARY KEY AUTOINCREMENT,
    at        TEXT NOT NULL,
    subject   TEXT NOT NULL,
    subject_id TEXT NOT NULL,
    action    TEXT NOT NULL,
    actor     TEXT NOT NULL,
    detail    TEXT NOT NULL DEFAULT ''
);
CREATE INDEX change_log_by_subject ON change_log (subject, subject_id, seq);
CREATE INDEX change_log_by_at ON change_log (at);
`},
}

// Migrations returns the schema steps in version order.
func Migrations() []Migration {
	out := make([]Migration, len(migrations))
	copy(out, migrations)
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out
}

// LatestVersion returns the highest migration number the code carries.
func LatestVersion() int {
	latest := 0
	for _, m := range migrations {
		if m.Version > latest {
			latest = m.Version
		}
	}
	return latest
}

// CurrentVersion returns the highest migration the database has run.
func CurrentVersion(ctx context.Context, db *sql.DB) (int, error) {
	if err := ensureSchemaTable(ctx, db); err != nil {
		return 0, err
	}
	var version sql.NullInt64
	err := db.QueryRowContext(ctx, `SELECT MAX(version) FROM schema_migrations`).Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("%w: reading the schema version: %v", ErrMigration, err)
	}
	if !version.Valid {
		return 0, nil
	}
	return int(version.Int64), nil
}

func ensureSchemaTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    name    TEXT NOT NULL,
    run_at  TEXT NOT NULL DEFAULT (datetime('now'))
)`)
	if err != nil {
		return fmt.Errorf("%w: creating the migration table: %v", ErrMigration, err)
	}
	return nil
}

// Migrate brings a database up to the latest schema and returns how many steps
// it ran. Each step is one transaction, so a failure halfway leaves the
// database at the last version that worked rather than somewhere in between.
func Migrate(ctx context.Context, db *sql.DB) (int, error) {
	if db == nil {
		return 0, fmt.Errorf("%w: no database", ErrMigration)
	}
	current, err := CurrentVersion(ctx, db)
	if err != nil {
		return 0, err
	}
	ran := 0
	for _, m := range Migrations() {
		if m.Version <= current {
			continue
		}
		if err := runMigration(ctx, db, m); err != nil {
			return ran, err
		}
		ran++
	}
	return ran, nil
}

func runMigration(ctx context.Context, db *sql.DB, m Migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("%w: starting %d %s: %v", ErrMigration, m.Version, m.Name, err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, m.Up); err != nil {
		return fmt.Errorf("%w: running %d %s: %v", ErrMigration, m.Version, m.Name, err)
	}
	_, err = tx.ExecContext(ctx,
		`INSERT INTO schema_migrations (version, name) VALUES (?, ?)`, m.Version, m.Name)
	if err != nil {
		return fmt.Errorf("%w: recording %d %s: %v", ErrMigration, m.Version, m.Name, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("%w: committing %d %s: %v", ErrMigration, m.Version, m.Name, err)
	}
	return nil
}
