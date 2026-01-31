package tests

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/glinskevxer/downguy/internal/store"
)

func openStore(t *testing.T) *sql.DB {
	t.Helper()
	db, err := store.Open(context.Background(), store.Memory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestMigrationsAreNumberedAndOrdered(t *testing.T) {
	t.Parallel()
	all := store.Migrations()
	if len(all) < 5 {
		t.Fatalf("migrations = %d", len(all))
	}
	seen := map[int]bool{}
	for i, m := range all {
		if m.Version <= 0 || m.Name == "" || m.Up == "" {
			t.Errorf("migration %d = %+v", i, m)
		}
		if seen[m.Version] {
			t.Errorf("version %d appears twice", m.Version)
		}
		seen[m.Version] = true
		if i > 0 && m.Version <= all[i-1].Version {
			t.Fatalf("migrations are not in order: %v", all)
		}
	}
	if store.LatestVersion() != all[len(all)-1].Version {
		t.Errorf("latest = %d", store.LatestVersion())
	}
}

func TestOpenMigratesToTheLatestVersion(t *testing.T) {
	t.Parallel()
	db := openStore(t)
	got, err := store.CurrentVersion(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	if got != store.LatestVersion() {
		t.Errorf("version = %d, want %d", got, store.LatestVersion())
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := openStore(t)
	ran, err := store.Migrate(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if ran != 0 {
		t.Errorf("a migrated database should run nothing, got %d", ran)
	}
	if _, err := store.Migrate(ctx, nil); err == nil {
		t.Error("no database should be rejected")
	}
}

func TestMigrateRunsEveryStepOnAFreshDatabase(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db, err := sql.Open("sqlite", store.Memory)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	db.SetMaxOpenConns(1)
	ran, err := store.Migrate(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if ran != len(store.Migrations()) {
		t.Errorf("ran %d of %d", ran, len(store.Migrations()))
	}
}

func TestTheExpectedTablesExist(t *testing.T) {
	t.Parallel()
	got, err := store.Tables(context.Background(), openStore(t))
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, name := range got {
		have[name] = true
	}
	for _, want := range []string{
		"schema_migrations", "poles", "attachments", "spans", "anchors", "guys",
		"applications", "application_poles", "application_events",
		"pole_surveys", "survey_measurements", "rate_years", "rental_bills",
	} {
		if !have[want] {
			t.Errorf("no %s table, got %v", want, got)
		}
	}
	for i := 1; i < len(got); i++ {
		if got[i] < got[i-1] {
			t.Fatalf("tables are not sorted: %v", got)
		}
	}
}

func TestForeignKeysAreOn(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := openStore(t)
	on, err := store.ForeignKeysOn(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if !on {
		t.Fatal("foreign key enforcement is off")
	}
	_, err = db.ExecContext(ctx,
		`INSERT INTO attachments (id, pole_id, owner, kind, height) VALUES (?, ?, ?, ?, ?)`,
		"A1", "NW-000000", "X", "comm_cable", 200)
	if err == nil {
		t.Error("an attachment on a pole that does not exist should be refused")
	}
}

func TestOpenOnDisk(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "downguy.db")
	db, err := store.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO poles (id, district, length_ft, class, species, set_depth, owner, condition)
		 VALUES ('NW-441829', 'NW', 40, '4', 'SPF', 60, 'SUMMIT VALLEY ELECTRIC', 'sound')`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	again, err := store.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = again.Close() }()
	var count int
	if err := again.QueryRowContext(ctx, `SELECT COUNT(*) FROM poles`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("the pole did not survive a reopen, got %d", count)
	}
	ran, err := store.Migrate(ctx, again)
	if err != nil {
		t.Fatal(err)
	}
	if ran != 0 {
		t.Errorf("reopening should not re-run migrations, got %d", ran)
	}
}

func TestOpenRejectsAnEmptyDataSource(t *testing.T) {
	t.Parallel()
	if _, err := store.Open(context.Background(), "  "); err == nil {
		t.Error("an empty data source should be rejected")
	}
}
