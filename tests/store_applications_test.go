package tests

import (
	"context"
	"errors"
	"testing"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/permit"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/store"
)

func applicationRepo(t *testing.T) (*store.Applications, context.Context) {
	t.Helper()
	return store.NewApplications(openStore(t)), context.Background()
}

func TestPutAndGetAnApplication(t *testing.T) {
	repo, ctx := applicationRepo(t)
	app := newApplication(t)
	walk(t, app, [][2]string{
		{"submitted", "2025-03-05"},
		{"accepted", "2025-03-12"},
	})
	if err := repo.Put(ctx, app); err != nil {
		t.Fatal(err)
	}
	got, err := repo.Get(ctx, "APP-1042")
	if err != nil {
		t.Fatal(err)
	}
	if got.Applicant != "MERIDIAN FIBER" || got.Owner != "SUMMIT VALLEY ELECTRIC" {
		t.Errorf("application = %+v", got)
	}
	if got.State != permit.StateAccepted {
		t.Errorf("state = %s", got.State)
	}
	if len(got.Poles) != 2 || got.Poles[0] != "NW-441829" {
		t.Errorf("poles = %v", got.Poles)
	}
	if len(got.History) != 3 {
		t.Fatalf("history = %v", got.History)
	}
	if got.History[1].To != permit.StateSubmitted || got.History[1].On.String() != "2025-03-05" {
		t.Errorf("event = %+v", got.History[1])
	}
	if got.History[1].By != "j.aldana" {
		t.Errorf("by = %q", got.History[1].By)
	}
	when, ok := got.EnteredAt(permit.StateAccepted)
	if !ok || when.String() != "2025-03-12" {
		t.Errorf("entered accepted %s ok=%v", when, ok)
	}
}

func TestPutRefusesABadApplication(t *testing.T) {
	repo, ctx := applicationRepo(t)
	if err := repo.Put(ctx, nil); err == nil {
		t.Error("no application should be rejected")
	}
	if err := repo.Put(ctx, &permit.Application{State: permit.StateDraft}); err == nil {
		t.Error("an application with no id should be rejected")
	}
	if err := repo.Put(ctx, &permit.Application{ID: "APP-1", State: permit.State("pending")}); err == nil {
		t.Error("an unknown state should be rejected")
	}
	count, err := repo.Count(ctx)
	if err != nil || count != 0 {
		t.Errorf("count = %d %v", count, err)
	}
}

func TestPutRewritesRatherThanAppends(t *testing.T) {
	repo, ctx := applicationRepo(t)
	app := newApplication(t)
	if err := repo.Put(ctx, app); err != nil {
		t.Fatal(err)
	}
	walk(t, app, [][2]string{{"submitted", "2025-03-05"}})
	if err := repo.Put(ctx, app); err != nil {
		t.Fatal(err)
	}
	got, err := repo.Get(ctx, "APP-1042")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.History) != 2 {
		t.Errorf("history = %v", got.History)
	}
	count, err := repo.Count(ctx)
	if err != nil || count != 1 {
		t.Errorf("count = %d %v", count, err)
	}
}

func TestGetAMissingApplication(t *testing.T) {
	repo, ctx := applicationRepo(t)
	if _, err := repo.Get(ctx, "APP-0"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("error = %v", err)
	}
}

func fileApplications(t *testing.T, repo *store.Applications, ctx context.Context) {
	t.Helper()
	first := newApplication(t)
	walk(t, first, [][2]string{{"submitted", "2025-03-05"}})

	second, err := permit.New("APP-1043", "CASCADE BROADBAND", "SUMMIT VALLEY ELECTRIC",
		[]plant.PoleID{"NW-441829"}, calendar.MustNew(2025, 4, 1), "r.beltre")
	if err != nil {
		t.Fatal(err)
	}
	third, err := permit.New("APP-1044", "MERIDIAN FIBER", "SUMMIT VALLEY ELECTRIC",
		[]plant.PoleID{"SE-1129043"}, calendar.MustNew(2025, 5, 1), "j.aldana")
	if err != nil {
		t.Fatal(err)
	}
	walk(t, third, [][2]string{{"withdrawn", "2025-05-20"}})

	for _, app := range []*permit.Application{first, second, third} {
		if err := repo.Put(ctx, app); err != nil {
			t.Fatal(err)
		}
	}
}

func TestListApplications(t *testing.T) {
	repo, ctx := applicationRepo(t)
	fileApplications(t, repo, ctx)

	submitted, err := repo.InState(ctx, permit.StateSubmitted)
	if err != nil {
		t.Fatal(err)
	}
	if len(submitted) != 1 || submitted[0].ID != "APP-1042" {
		t.Errorf("submitted = %v", submitted)
	}
	if got, err := repo.InState(ctx, permit.StateClosed); err != nil || len(got) != 0 {
		t.Errorf("closed = %v %v", got, err)
	}

	meridian, err := repo.ForApplicant(ctx, "MERIDIAN FIBER")
	if err != nil {
		t.Fatal(err)
	}
	if len(meridian) != 2 || meridian[0].ID != "APP-1042" {
		t.Errorf("meridian = %v", meridian)
	}

	live, err := repo.Live(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(live) != 2 {
		t.Errorf("the withdrawn one is not live, got %v", live)
	}
	for _, app := range live {
		if !app.Live() {
			t.Errorf("%s is not live", app.ID)
		}
	}
}

func TestApplicationsTouchingAPole(t *testing.T) {
	repo, ctx := applicationRepo(t)
	fileApplications(t, repo, ctx)
	got, err := repo.TouchingPole(ctx, "NW-441829")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "APP-1042" {
		t.Errorf("touching = %v", got)
	}
	if got, err := repo.TouchingPole(ctx, "NW-441899"); err != nil || len(got) != 0 {
		t.Errorf("nothing touches that pole, got %v %v", got, err)
	}
}
