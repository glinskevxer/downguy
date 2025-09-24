package tests

import (
	"testing"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/permit"
	"github.com/glinskevxer/downguy/internal/plant"
)

func newApplication(t *testing.T) *permit.Application {
	t.Helper()
	a, err := permit.New("APP-1042", "MERIDIAN FIBER", "SUMMIT VALLEY ELECTRIC",
		[]plant.PoleID{"NW-441830", "NW-441829", "NW-441829"},
		calendar.MustNew(2025, 3, 3), "j.aldana")
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestNewApplicationStartsInDraft(t *testing.T) {
	a := newApplication(t)
	if a.State != permit.StateDraft {
		t.Errorf("state = %s", a.State)
	}
	if a.PoleCount() != 2 {
		t.Errorf("duplicate poles should be folded, got %d", a.PoleCount())
	}
	if a.Poles[0] != "NW-441829" {
		t.Errorf("poles should be sorted, got %v", a.Poles)
	}
	if !a.Covers("NW-441830") || a.Covers("SE-1129043") {
		t.Error("Covers is wrong")
	}
	if a.Opened().String() != "2025-03-03" {
		t.Errorf("opened %s", a.Opened())
	}
	if !a.Live() || a.HeldBy() != permit.HolderAttacher {
		t.Errorf("a draft is live and with the attacher, got %v %s", a.Live(), a.HeldBy())
	}
	if a.String() == "" {
		t.Error("String should say something")
	}
}

func TestNewApplicationRejectsBadInput(t *testing.T) {
	poles := []plant.PoleID{"NW-441829"}
	on := calendar.MustNew(2025, 3, 3)
	cases := []struct {
		name  string
		id    string
		app   string
		owner string
		poles []plant.PoleID
		on    calendar.Date
	}{
		{"no id", " ", "A", "B", poles, on},
		{"no applicant", "APP-1", "", "B", poles, on},
		{"no owner", "APP-1", "A", " ", poles, on},
		{"no poles", "APP-1", "A", "B", nil, on},
		{"no date", "APP-1", "A", "B", poles, calendar.Zero},
		{"bad pole", "APP-1", "A", "B", []plant.PoleID{"nope"}, on},
	}
	for _, c := range cases {
		if _, err := permit.New(c.id, c.app, c.owner, c.poles, c.on, "j.aldana"); err == nil {
			t.Errorf("%s should have been rejected", c.name)
		}
	}
}

func TestMoveThroughTheHappyPath(t *testing.T) {
	a := newApplication(t)
	steps := []struct {
		to permit.State
		on string
	}{
		{permit.StateSubmitted, "2025-03-05"},
		{permit.StateAccepted, "2025-03-12"},
		{permit.StateSurveying, "2025-03-20"},
		{permit.StateEngineered, "2025-04-28"},
		{permit.StateEstimated, "2025-05-12"},
		{permit.StateAuthorised, "2025-05-23"},
		{permit.StateScheduled, "2025-06-10"},
		{permit.StateConstructed, "2025-07-22"},
		{permit.StateInspected, "2025-08-04"},
		{permit.StateClosed, "2025-08-11"},
	}
	for _, s := range steps {
		on, err := calendar.ParseDate(s.on)
		if err != nil {
			t.Fatal(err)
		}
		if err := a.MoveTo(s.to, on, "j.aldana", ""); err != nil {
			t.Fatalf("moving to %s: %v", s.to, err)
		}
	}
	if a.State != permit.StateClosed || a.Live() {
		t.Errorf("state = %s live = %v", a.State, a.Live())
	}
	if len(a.History) != len(steps)+1 {
		t.Errorf("history = %d events", len(a.History))
	}
	path := a.Path()
	if len(path) != len(steps)+1 || path[len(path)-1] != permit.StateClosed {
		t.Errorf("path = %v", path)
	}
	when, ok := a.EnteredAt(permit.StateEstimated)
	if !ok || when.String() != "2025-05-12" {
		t.Errorf("entered estimated %s ok=%v", when, ok)
	}
	if _, ok := a.EnteredAt(permit.StateRejected); ok {
		t.Error("it was never rejected")
	}
	if got := a.Age(calendar.MustNew(2025, 9, 1)); got != 182 {
		t.Errorf("age = %d", got)
	}
}

func TestMoveRefusesAnIllegalTransition(t *testing.T) {
	a := newApplication(t)
	on := calendar.MustNew(2025, 3, 5)
	if err := a.MoveTo(permit.StateClosed, on, "j.aldana", ""); err == nil {
		t.Error("a draft cannot be closed")
	}
	if err := a.MoveTo(permit.State("pending"), on, "j.aldana", ""); err == nil {
		t.Error("an unknown state should be rejected")
	}
	if a.State != permit.StateDraft {
		t.Errorf("a refused move should leave the state alone, got %s", a.State)
	}
	if len(a.History) != 1 {
		t.Errorf("a refused move should not be recorded, got %d events", len(a.History))
	}
}

func TestMoveRefusesABackwardsDate(t *testing.T) {
	a := newApplication(t)
	if err := a.MoveTo(permit.StateSubmitted, calendar.MustNew(2025, 3, 5), "j.aldana", ""); err != nil {
		t.Fatal(err)
	}
	if err := a.MoveTo(permit.StateAccepted, calendar.MustNew(2025, 3, 1), "j.aldana", ""); err == nil {
		t.Error("a history cannot go backwards")
	}
	if err := a.MoveTo(permit.StateAccepted, calendar.MustNew(2025, 3, 5), "j.aldana", ""); err != nil {
		t.Errorf("the same day is allowed: %v", err)
	}
}

func TestMoveRefusesAnUndatedOrUnsignedStep(t *testing.T) {
	a := newApplication(t)
	if err := a.MoveTo(permit.StateSubmitted, calendar.Zero, "j.aldana", ""); err == nil {
		t.Error("an undated move should be rejected")
	}
	if err := a.MoveTo(permit.StateSubmitted, calendar.MustNew(2025, 3, 5), "  ", ""); err == nil {
		t.Error("a move nobody signed should be rejected")
	}
}

func TestRejectionAndWithdrawalAreTerminal(t *testing.T) {
	a := newApplication(t)
	if err := a.MoveTo(permit.StateSubmitted, calendar.MustNew(2025, 3, 5), "j.aldana", ""); err != nil {
		t.Fatal(err)
	}
	if err := a.MoveTo(permit.StateRejected, calendar.MustNew(2025, 3, 9), "d.orme", "no capacity"); err != nil {
		t.Fatal(err)
	}
	if a.Live() || a.HeldBy() != permit.HolderNobody {
		t.Errorf("a rejected application is finished, got live=%v held=%s", a.Live(), a.HeldBy())
	}
	if err := a.MoveTo(permit.StateSubmitted, calendar.MustNew(2025, 4, 1), "j.aldana", ""); err == nil {
		t.Error("nothing follows a rejection")
	}
	last, ok := a.Last()
	if !ok || last.Note != "no capacity" {
		t.Errorf("last event = %+v", last)
	}
	empty := &permit.Application{}
	if _, ok := empty.Last(); ok {
		t.Error("an empty history has no last event")
	}
	if empty.Age(calendar.MustNew(2025, 1, 1)) != 0 {
		t.Error("nor an age")
	}
	if empty.Path() != nil {
		t.Error("nor a path")
	}
}
