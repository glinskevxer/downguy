package tests

import (
	"testing"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/permit"
)

func walk(t *testing.T, a *permit.Application, steps [][2]string) {
	t.Helper()
	for _, s := range steps {
		state, err := permit.ParseState(s[0])
		if err != nil {
			t.Fatal(err)
		}
		on, err := calendar.ParseDate(s[1])
		if err != nil {
			t.Fatal(err)
		}
		if err := a.MoveTo(state, on, "j.aldana", ""); err != nil {
			t.Fatalf("moving to %s on %s: %v", state, on, err)
		}
	}
}

func TestAssessAPromptApplication(t *testing.T) {
	a := newApplication(t)
	walk(t, a, [][2]string{
		{"submitted", "2025-03-05"},
		{"accepted", "2025-03-12"},
		{"surveying", "2025-03-20"},
		{"engineered", "2025-04-28"},
		{"estimated", "2025-05-12"},
		{"authorised", "2025-05-23"},
	})
	got, err := permit.Assess(a, calendar.MustNew(2025, 6, 1), holidays(t, 2025))
	if err != nil {
		t.Fatal(err)
	}
	if !got.OnTime() {
		for _, in := range got.Late {
			t.Errorf("late at %s by %d days", in.State, in.Overrun)
		}
	}
	if len(got.Intervals) != 7 {
		t.Errorf("intervals = %d", len(got.Intervals))
	}
	if got.DaysWith(permit.HolderUtility) <= 0 {
		t.Errorf("the utility held it for %d days", got.DaysWith(permit.HolderUtility))
	}
	if got.DaysWith(permit.HolderAttacher) <= 0 {
		t.Errorf("the attacher held it for %d days", got.DaysWith(permit.HolderAttacher))
	}
	if _, ok := got.WorstOverrun(); ok {
		t.Error("nothing overran")
	}
	if got.Summary() == "" {
		t.Error("Summary should say something")
	}
}

func TestAssessCatchesASlowSurvey(t *testing.T) {
	a := newApplication(t)
	walk(t, a, [][2]string{
		{"submitted", "2025-03-05"},
		{"accepted", "2025-03-12"},
		{"surveying", "2025-03-20"},
		{"engineered", "2025-07-01"},
	})
	got, err := permit.Assess(a, calendar.MustNew(2025, 7, 15), holidays(t, 2025))
	if err != nil {
		t.Fatal(err)
	}
	if got.OnTime() {
		t.Fatal("a survey running from March to July misses the forty five day clock")
	}
	worst, ok := got.WorstOverrun()
	if !ok || worst.State != permit.StateSurveying {
		t.Errorf("worst = %+v", worst)
	}
	if worst.Overrun <= 0 {
		t.Errorf("overrun = %d", worst.Overrun)
	}
	if worst.Holder != permit.HolderUtility {
		t.Errorf("the survey clock is the utility's, got %s", worst.Holder)
	}
	if len(got.LateFor(permit.HolderUtility)) == 0 {
		t.Error("the overrun should be charged to the utility")
	}
	if len(got.LateFor(permit.HolderAttacher)) != 0 {
		t.Errorf("the attacher was not late, got %v", got.LateFor(permit.HolderAttacher))
	}
}

func TestAssessCatchesASlowAttacher(t *testing.T) {
	a := newApplication(t)
	walk(t, a, [][2]string{
		{"submitted", "2025-03-05"},
		{"accepted", "2025-03-12"},
		{"surveying", "2025-03-20"},
		{"engineered", "2025-04-28"},
		{"estimated", "2025-05-12"},
		{"authorised", "2025-08-01"},
	})
	got, err := permit.Assess(a, calendar.MustNew(2025, 8, 15), holidays(t, 2025))
	if err != nil {
		t.Fatal(err)
	}
	late := got.LateFor(permit.HolderAttacher)
	if len(late) != 1 || late[0].State != permit.StateEstimated {
		t.Errorf("late for the attacher = %v", late)
	}
}

func TestTheOpenIntervalRunsToTheDateAsked(t *testing.T) {
	a := newApplication(t)
	walk(t, a, [][2]string{{"submitted", "2025-03-05"}})
	got, err := permit.Assess(a, calendar.MustNew(2025, 3, 10), holidays(t, 2025))
	if err != nil {
		t.Fatal(err)
	}
	current, open := got.Current()
	if !open {
		t.Fatal("the application is still sitting submitted")
	}
	if current.State != permit.StateSubmitted || current.Left.String() != "2025-03-10" {
		t.Errorf("current = %+v", current)
	}
	if current.Late() {
		t.Error("five days is inside the ten business day clock")
	}
	later, err := permit.Assess(a, calendar.MustNew(2025, 4, 10), holidays(t, 2025))
	if err != nil {
		t.Fatal(err)
	}
	if later.OnTime() {
		t.Error("a month sitting submitted misses it")
	}
	empty := &permit.Compliance{}
	if _, ok := empty.Current(); ok {
		t.Error("an empty assessment has no current interval")
	}
}

func TestAnUntimedStateStillCountsDays(t *testing.T) {
	a := newApplication(t)
	got, err := permit.Assess(a, calendar.MustNew(2025, 3, 20), holidays(t, 2025))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Intervals) != 1 {
		t.Fatalf("intervals = %v", got.Intervals)
	}
	in := got.Intervals[0]
	if in.Timed {
		t.Error("nobody is timed on a draft")
	}
	if in.Used != 17 {
		t.Errorf("used = %d", in.Used)
	}
	if in.Late() {
		t.Error("an untimed interval is never late")
	}
	if got.DaysWith(permit.HolderAttacher) != 17 {
		t.Errorf("days with the attacher = %d", got.DaysWith(permit.HolderAttacher))
	}
}

func TestAssessRejectsBadInput(t *testing.T) {
	a := newApplication(t)
	h := holidays(t, 2025)
	if _, err := permit.Assess(nil, calendar.MustNew(2025, 3, 10), h); err == nil {
		t.Error("no application should be rejected")
	}
	if _, err := permit.Assess(a, calendar.Zero, h); err == nil {
		t.Error("no date should be rejected")
	}
	if _, err := permit.Assess(&permit.Application{ID: "APP-0"}, calendar.MustNew(2025, 3, 10), h); err == nil {
		t.Error("an application with no history should be rejected")
	}
}

func TestAssessClampsADateBeforeTheApplication(t *testing.T) {
	a := newApplication(t)
	got, err := permit.Assess(a, calendar.MustNew(2025, 1, 1), holidays(t, 2025))
	if err != nil {
		t.Fatal(err)
	}
	in := got.Intervals[0]
	if in.Left.Before(in.Entered) {
		t.Errorf("an interval cannot end before it starts: %+v", in)
	}
	if in.Used != 0 {
		t.Errorf("used = %d", in.Used)
	}
}
