package tests

import (
	"context"
	"errors"
	"testing"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/store"
	"github.com/glinskevxer/downguy/internal/survey"
)

func surveyRepo(t *testing.T) (*store.Surveys, context.Context) {
	t.Helper()
	db := openStore(t)
	ctx := context.Background()
	poles := store.NewPoles(db)
	for _, id := range []plant.PoleID{"NW-441829", "NW-441830"} {
		p := samplePole()
		p.ID = id
		if err := poles.Put(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	return store.NewSurveys(db), ctx
}

func TestPutAndGetASurvey(t *testing.T) {
	t.Parallel()
	repo, ctx := surveyRepo(t)
	sheet := sampleSheet()
	sheet.Notes = "guy guard missing"
	if err := repo.Put(ctx, sheet); err != nil {
		t.Fatal(err)
	}
	got, err := repo.Get(ctx, "SV-2201")
	if err != nil {
		t.Fatal(err)
	}
	if got.Pole != "NW-441829" || got.Crew != "CREW 41" {
		t.Errorf("sheet = %+v", got)
	}
	if got.Taken.String() != "2025-05-06" {
		t.Errorf("taken %s", got.Taken)
	}
	if got.Condition != plant.ConditionSound || got.Notes != "guy guard missing" {
		t.Errorf("sheet = %+v", got)
	}
	if len(got.Measurements) != 4 {
		t.Fatalf("readings = %v", got.Measurements)
	}
	if got.Measurements[0].Subject != "A1" {
		t.Errorf("readings should come back highest first, got %v", got.Measurements)
	}
	if got.Measurements[0].Method != survey.MethodLaser {
		t.Errorf("method = %s", got.Measurements[0].Method)
	}
	if err := got.Validate(); err != nil {
		t.Errorf("what came back does not validate: %v", err)
	}
}

func TestPutRefusesAnInvalidSurvey(t *testing.T) {
	t.Parallel()
	repo, ctx := surveyRepo(t)
	bad := sampleSheet()
	bad.Crew = ""
	if err := repo.Put(ctx, bad); err == nil {
		t.Error("a sheet with no crew should not reach the table")
	}
	count, err := repo.Count(ctx)
	if err != nil || count != 0 {
		t.Errorf("count = %d %v", count, err)
	}
	orphan := sampleSheet()
	orphan.Pole = "NW-999999"
	if err := repo.Put(ctx, orphan); err == nil {
		t.Error("a survey of a pole that does not exist should be refused")
	}
}

func TestPutRewritesTheReadings(t *testing.T) {
	t.Parallel()
	repo, ctx := surveyRepo(t)
	if err := repo.Put(ctx, sampleSheet()); err != nil {
		t.Fatal(err)
	}
	trimmed := sampleSheet()
	trimmed.Measurements = trimmed.Measurements[:2]
	if err := repo.Put(ctx, trimmed); err != nil {
		t.Fatal(err)
	}
	got, err := repo.Get(ctx, "SV-2201")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Measurements) != 2 {
		t.Errorf("readings = %v", got.Measurements)
	}
}

func TestLatestSurveyOfAPole(t *testing.T) {
	t.Parallel()
	repo, ctx := surveyRepo(t)
	older := sampleSheet()
	older.ID = "SV-2100"
	older.Taken = calendar.MustNew(2024, 8, 1)
	newer := sampleSheet()
	for _, s := range []survey.PoleSheet{older, newer} {
		if err := repo.Put(ctx, s); err != nil {
			t.Fatal(err)
		}
	}
	got, err := repo.Latest(ctx, "NW-441829")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "SV-2201" {
		t.Errorf("latest = %s", got.ID)
	}
	if len(got.Measurements) == 0 {
		t.Error("the latest survey should come back with its readings")
	}
	if _, err := repo.Latest(ctx, "NW-441830"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("error = %v", err)
	}
}

func TestSurveysOnAPoleComeBackNewestFirst(t *testing.T) {
	t.Parallel()
	repo, ctx := surveyRepo(t)
	older := sampleSheet()
	older.ID = "SV-2100"
	older.Taken = calendar.MustNew(2024, 8, 1)
	for _, s := range []survey.PoleSheet{older, sampleSheet()} {
		if err := repo.Put(ctx, s); err != nil {
			t.Fatal(err)
		}
	}
	got, err := repo.OnPole(ctx, "NW-441829")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "SV-2201" {
		t.Errorf("surveys = %v", got)
	}
	for _, s := range got {
		if len(s.Measurements) == 0 {
			t.Errorf("%s came back with no readings", s.ID)
		}
	}
	if empty, err := repo.OnPole(ctx, "NW-441830"); err != nil || len(empty) != 0 {
		t.Errorf("an unsurveyed pole = %v %v", empty, err)
	}
}

func TestStaleSurveys(t *testing.T) {
	t.Parallel()
	repo, ctx := surveyRepo(t)
	old := sampleSheet()
	old.ID = "SV-2100"
	old.Taken = calendar.MustNew(2024, 8, 1)
	fresh := sampleSheet()
	fresh.ID = "SV-2300"
	fresh.Pole = "NW-441830"
	for _, s := range []survey.PoleSheet{old, fresh} {
		if err := repo.Put(ctx, s); err != nil {
			t.Fatal(err)
		}
	}
	got, err := repo.StaleAsOf(ctx, calendar.MustNew(2025, 6, 1))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "NW-441829" {
		t.Errorf("stale = %v", got)
	}
	// A newer survey of the same pole clears it.
	if err := repo.Put(ctx, sampleSheet()); err != nil {
		t.Fatal(err)
	}
	got, err = repo.StaleAsOf(ctx, calendar.MustNew(2025, 6, 1))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("nothing should be stale now, got %v", got)
	}
}

func TestGetAMissingSurvey(t *testing.T) {
	t.Parallel()
	repo, ctx := surveyRepo(t)
	if _, err := repo.Get(ctx, "SV-0"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("error = %v", err)
	}
}
