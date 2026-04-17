package tests

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/glinskevxer/downguy/internal/cliapp"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/store"
)

func TestTallyCountsEveryTable(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := openStore(t)
	empty, err := store.Tally(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if empty.Total() != 0 {
		t.Errorf("an empty database counts %d", empty.Total())
	}

	loader := store.NewPlant(db)
	a := sampleAssembly()
	a.Anchors = []plant.Anchor{sampleAnchor()}
	if err := loader.SaveAssembly(ctx, a); err != nil {
		t.Fatal(err)
	}
	got, err := store.Tally(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if got.Poles != 1 || got.Attachments != 4 || got.Anchors != 1 || got.Guys != 1 {
		t.Errorf("counts = %+v", got)
	}
	if got.Total() != 7 {
		t.Errorf("total = %d", got.Total())
	}
	if _, err := store.Tally(ctx, nil); err == nil {
		t.Error("no database should be rejected")
	}
}

func TestCheckFindsNothingOnACleanDatabase(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := openStore(t)
	loader := store.NewPlant(db)
	if err := loader.SaveAssembly(ctx, sampleAssembly()); err != nil {
		t.Fatal(err)
	}
	got, err := store.Check(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("problems = %v", got)
	}
	if _, err := store.Check(ctx, nil); err == nil {
		t.Error("no database should be rejected")
	}
}

func TestCheckCatchesWhatAForeignKeyCannot(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := openStore(t)
	loader := store.NewPlant(db)
	if err := loader.SaveAssembly(ctx, sampleAssembly()); err != nil {
		t.Fatal(err)
	}

	// An attachment at the very top of its own pole. Nothing in the schema
	// forbids it, and the record type would have refused it, so it can only
	// get here through a direct write.
	if _, err := db.ExecContext(ctx,
		`UPDATE attachments SET height = 400 WHERE id = 'A4'`); err != nil {
		t.Fatal(err)
	}
	// An application covering a pole nobody has.
	if _, err := db.ExecContext(ctx,
		`INSERT INTO applications (id, applicant, owner, state) VALUES ('APP-1','X','Y','draft')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO application_poles (application_id, pole_id) VALUES ('APP-1','NW-999999')`); err != nil {
		t.Fatal(err)
	}

	got, err := store.Check(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, p := range got {
		found[p.Check] = true
		if p.Detail == "" {
			t.Errorf("problem = %+v", p)
		}
	}
	for _, want := range []string{"attachment_above_pole", "orphan_application_poles", "application_without_history"} {
		if !found[want] {
			t.Errorf("%s was not reported, got %v", want, got)
		}
	}
	for i := 1; i < len(got); i++ {
		if got[i].Check < got[i-1].Check {
			t.Fatalf("problems are not sorted: %v", got)
		}
	}
}

func TestCompactAndPageCount(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := openStore(t)
	before, err := store.PageCount(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if before <= 0 {
		t.Errorf("pages = %d", before)
	}
	if err := store.Compact(ctx, db); err != nil {
		t.Fatal(err)
	}
	after, err := store.PageCount(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if after <= 0 {
		t.Errorf("pages = %d", after)
	}
	if err := store.Compact(ctx, nil); err == nil {
		t.Error("no database should be rejected")
	}
}

func TestMaintainCommand(t *testing.T) {
	t.Parallel()
	path := loadedDB(t)
	got := cli("maintain", "-db", path)
	if got.Code != cliapp.CodeOK {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	for _, want := range []string{"schema version", "poles", "attachments", "found nothing"} {
		if !strings.Contains(got.Stdout, want) {
			t.Errorf("stdout does not mention %q:\n%s", want, got.Stdout)
		}
	}
}

func TestMaintainAsJSONAndCompacting(t *testing.T) {
	t.Parallel()
	path := loadedDB(t)
	got := cli("maintain", "-db", path, "-json", "-compact")
	if got.Code != cliapp.CodeOK {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	var back cliapp.MaintainJSON
	if err := json.Unmarshal([]byte(got.Stdout), &back); err != nil {
		t.Fatalf("%v\n%s", err, got.Stdout)
	}
	if back.Schema != store.LatestVersion() {
		t.Errorf("schema = %d", back.Schema)
	}
	if back.Counts.Poles != 3 || back.Counts.Attachments != 3 {
		t.Errorf("counts = %+v", back.Counts)
	}
	if back.PagesBefore == 0 || back.Pages == 0 {
		t.Errorf("a compaction should report both page counts, got %+v", back)
	}
	if len(back.Problems) != 0 {
		t.Errorf("problems = %v", back.Problems)
	}
}

func TestMaintainRejectsBadUse(t *testing.T) {
	t.Parallel()
	path := loadedDB(t)
	for _, args := range [][]string{
		{"maintain", "-db", path, "extra"},
		{"maintain", "-nonsense"},
	} {
		if got := cli(args...); got.Code != cliapp.CodeUsage {
			t.Errorf("%v gave code %d", args, got.Code)
		}
	}
}

func TestTheTallyCoversEveryTableTheMigrationsCreate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := openStore(t)
	tables, err := store.Tables(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	counted := map[string]bool{}
	for _, name := range store.TalliedTables() {
		counted[name] = true
	}
	// The child tables and the migration ledger are deliberately not separate
	// counts on the summary, so they are named here rather than left to be
	// noticed as a gap.
	notCounted := map[string]bool{
		"schema_migrations": true,
		"application_poles": true, "application_events": true,
		"survey_measurements": true, "rate_years": true,
	}
	for _, name := range tables {
		if counted[name] || notCounted[name] {
			continue
		}
		t.Errorf("the schema creates %s and nothing counts it", name)
	}
	for _, name := range store.TalliedTables() {
		found := false
		for _, real := range tables {
			if real == name {
				found = true
			}
		}
		if !found {
			t.Errorf("the tally counts %s, which the schema does not create", name)
		}
	}
}
