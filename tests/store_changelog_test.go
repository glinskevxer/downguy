package tests

import (
	"context"
	"testing"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/store"
)

func changeLog(t *testing.T) (*store.ChangeLog, context.Context) {
	t.Helper()
	return store.NewChangeLog(openStore(t)), context.Background()
}

func change(id string, subject store.Subject, action store.Action, day int) store.Change {
	return store.Change{
		At: calendar.MustNew(2025, 5, day), Subject: subject, ID: id,
		Action: action, Actor: "j.aldana", Detail: "imported from the mapping export",
	}
}

func TestSubjectsAndActions(t *testing.T) {
	t.Parallel()
	subjects := store.Subjects()
	if len(subjects) != 7 {
		t.Fatalf("subjects = %v", subjects)
	}
	for i := 1; i < len(subjects); i++ {
		if subjects[i] < subjects[i-1] {
			t.Fatalf("subjects are not sorted: %v", subjects)
		}
	}
	for _, s := range subjects {
		if !store.KnownSubject(s) {
			t.Errorf("%s should be known", s)
		}
	}
	if store.KnownSubject(store.Subject("weather")) {
		t.Error("the weather is not a subject")
	}
	for _, a := range []store.Action{store.ActionCreated, store.ActionUpdated,
		store.ActionRemoved, store.ActionImported, store.ActionBilled} {
		if !store.KnownAction(a) {
			t.Errorf("%s should be known", a)
		}
	}
	if store.KnownAction(store.Action("mangled")) {
		t.Error("that is not an action")
	}
}

func TestChangeValidate(t *testing.T) {
	t.Parallel()
	good := change("NW-441829", store.SubjectPole, store.ActionImported, 6)
	if err := good.Validate(); err != nil {
		t.Fatalf("a good change failed: %v", err)
	}
	cases := map[string]func(*store.Change){
		"no date":     func(c *store.Change) { c.At = calendar.Zero },
		"bad subject": func(c *store.Change) { c.Subject = store.Subject("weather") },
		"no id":       func(c *store.Change) { c.ID = "  " },
		"bad action":  func(c *store.Change) { c.Action = store.Action("mangled") },
		"no actor":    func(c *store.Change) { c.Actor = "" },
	}
	for name, mutate := range cases {
		c := good
		mutate(&c)
		if err := c.Validate(); err == nil {
			t.Errorf("%s should have been rejected", name)
		}
	}
	if good.String() == "" {
		t.Error("String should say something")
	}
	bare := good
	bare.Detail = ""
	if len(bare.String()) >= len(good.String()) {
		t.Error("a change with no detail prints shorter")
	}
}

func TestAppendAndReadBack(t *testing.T) {
	t.Parallel()
	log, ctx := changeLog(t)
	first, err := log.Append(ctx, change("NW-441829", store.SubjectPole, store.ActionImported, 6))
	if err != nil {
		t.Fatal(err)
	}
	second, err := log.Append(ctx, change("NW-441829", store.SubjectPole, store.ActionUpdated, 8))
	if err != nil {
		t.Fatal(err)
	}
	if second <= first {
		t.Errorf("sequence numbers should climb, got %d then %d", first, second)
	}
	got, err := log.For(ctx, store.SubjectPole, "NW-441829")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("history = %v", got)
	}
	if got[0].Action != store.ActionImported || got[1].Action != store.ActionUpdated {
		t.Errorf("the history should read oldest first, got %v", got)
	}
	if got[0].At.String() != "2025-05-06" || got[0].Actor != "j.aldana" {
		t.Errorf("change = %+v", got[0])
	}
	if got[0].Detail == "" {
		t.Errorf("change = %+v", got[0])
	}
	if other, err := log.For(ctx, store.SubjectSpan, "NW-441829"); err != nil || len(other) != 0 {
		t.Errorf("a different subject with the same id is a different record, got %v %v", other, err)
	}
}

func TestAppendRefusesABadChange(t *testing.T) {
	t.Parallel()
	log, ctx := changeLog(t)
	bad := change("NW-441829", store.SubjectPole, store.ActionImported, 6)
	bad.Actor = ""
	if _, err := log.Append(ctx, bad); err == nil {
		t.Error("a change nobody made should be rejected")
	}
	count, err := log.Count(ctx)
	if err != nil || count != 0 {
		t.Errorf("count = %d %v", count, err)
	}
}

func TestAppendAllIsOneTransaction(t *testing.T) {
	t.Parallel()
	log, ctx := changeLog(t)
	batch := []store.Change{
		change("NW-441829", store.SubjectPole, store.ActionImported, 6),
		change("NW-441830", store.SubjectPole, store.ActionImported, 6),
	}
	if err := log.AppendAll(ctx, batch); err != nil {
		t.Fatal(err)
	}
	count, err := log.Count(ctx)
	if err != nil || count != 2 {
		t.Errorf("count = %d %v", count, err)
	}
	broken := append([]store.Change{}, batch...)
	broken = append(broken, store.Change{})
	if err := log.AppendAll(ctx, broken); err == nil {
		t.Error("a batch with a bad entry should be refused whole")
	}
	count, err = log.Count(ctx)
	if err != nil || count != 2 {
		t.Errorf("the rolled back batch should have written nothing, got %d", count)
	}
}

func TestSinceAndByActor(t *testing.T) {
	t.Parallel()
	log, ctx := changeLog(t)
	early := change("NW-441829", store.SubjectPole, store.ActionImported, 1)
	late := change("NW-441830", store.SubjectPole, store.ActionImported, 20)
	late.Actor = "d.orme"
	if err := log.AppendAll(ctx, []store.Change{early, late}); err != nil {
		t.Fatal(err)
	}
	got, err := log.Since(ctx, calendar.MustNew(2025, 5, 10))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "NW-441830" {
		t.Errorf("since = %v", got)
	}
	all, err := log.Since(ctx, calendar.MustNew(2020, 1, 1))
	if err != nil || len(all) != 2 {
		t.Errorf("since = %v %v", all, err)
	}
	mine, err := log.ByActor(ctx, "d.orme")
	if err != nil {
		t.Fatal(err)
	}
	if len(mine) != 1 || mine[0].ID != "NW-441830" {
		t.Errorf("by actor = %v", mine)
	}
	if none, err := log.ByActor(ctx, "nobody"); err != nil || len(none) != 0 {
		t.Errorf("by actor = %v %v", none, err)
	}
}
