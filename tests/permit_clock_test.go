package tests

import (
	"testing"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/permit"
)

func holidays(t *testing.T, year int) permit.Holidays {
	t.Helper()
	h, ok := permit.StandardHolidays(year)
	if !ok {
		t.Fatalf("no holiday list for %d", year)
	}
	return h
}

func TestTimedStatesAllCarryAWellFormedDeadline(t *testing.T) {
	states := permit.TimedStates()
	if len(states) != 9 {
		t.Fatalf("timed states = %v", states)
	}
	for i := 1; i < len(states); i++ {
		if states[i] < states[i-1] {
			t.Fatalf("timed states are not sorted: %v", states)
		}
	}
	for _, s := range states {
		d, ok := permit.DeadlineFor(s)
		if !ok {
			t.Errorf("%s is listed but has no deadline", s)
			continue
		}
		if d.State != s || d.Days <= 0 || d.Cite == "" {
			t.Errorf("%s = %+v", s, d)
		}
		if d.Holder != s.HeldBy() {
			t.Errorf("%s is held by %s but timed against %s", s, s.HeldBy(), d.Holder)
		}
		if d.String() == "" {
			t.Errorf("%s prints nothing", s)
		}
	}
	if _, ok := permit.DeadlineFor(permit.StateDraft); ok {
		t.Error("nobody is timed on a draft")
	}
}

func TestHolidayListsParseAndCover(t *testing.T) {
	years := permit.ObservedYears()
	if len(years) != 2 || years[0] != 2025 {
		t.Errorf("years = %v", years)
	}
	h := holidays(t, 2025)
	if !h.Contains(calendar.MustNew(2025, 7, 4)) {
		t.Error("the fourth of July is observed")
	}
	if h.Contains(calendar.MustNew(2025, 7, 5)) {
		t.Error("the fifth is not")
	}
	if _, ok := permit.StandardHolidays(1999); ok {
		t.Error("no list for 1999")
	}
	if _, err := permit.NewHolidays("not a date"); err == nil {
		t.Error("a bad date should be rejected")
	}
}

func TestWorkingDays(t *testing.T) {
	h := holidays(t, 2025)
	if h.Working(calendar.MustNew(2025, 7, 4)) {
		t.Error("a holiday is not a working day")
	}
	if h.Working(calendar.MustNew(2025, 7, 5)) {
		t.Error("a Saturday is not a working day")
	}
	if !h.Working(calendar.MustNew(2025, 7, 7)) {
		t.Error("the Monday is")
	}
	var none permit.Holidays
	if !none.Working(calendar.MustNew(2025, 7, 4)) {
		t.Error("with no list only the weekend is skipped")
	}
}

func TestCalendarDayDeadlineIsPlainArithmetic(t *testing.T) {
	d, ok := permit.DeadlineFor(permit.StateAccepted)
	if !ok {
		t.Fatal("no deadline on an accepted application")
	}
	from := calendar.MustNew(2025, 6, 2)
	got := d.Add(from, holidays(t, 2025))
	if got.String() != "2025-07-17" {
		t.Errorf("45 calendar days from 2025-06-02 = %s", got)
	}
	if n := d.Elapsed(from, got, nil); n != 45 {
		t.Errorf("elapsed = %d", n)
	}
}

func TestBusinessDayDeadlineSkipsWeekendsAndHolidays(t *testing.T) {
	d, ok := permit.DeadlineFor(permit.StateSubmitted)
	if !ok {
		t.Fatal("no deadline on a submitted application")
	}
	h := holidays(t, 2025)
	// Monday 30 June plus ten business days, with Friday 4 July observed.
	from := calendar.MustNew(2025, 6, 30)
	got := d.Add(from, h)
	if got.String() != "2025-07-15" {
		t.Errorf("ten business days from 2025-06-30 = %s", got)
	}
	// Without the holiday it would land a day earlier.
	plain := d.Add(from, nil)
	if !plain.Before(got) {
		t.Errorf("the holiday should push the date out: %s against %s", plain, got)
	}
}

func TestElapsedCountsTheSameWayTheDeadlineDoes(t *testing.T) {
	d, _ := permit.DeadlineFor(permit.StateSubmitted)
	h := holidays(t, 2025)
	from := calendar.MustNew(2025, 6, 30)
	due := d.Add(from, h)
	if n := d.Elapsed(from, due, h); n != d.Days {
		t.Errorf("elapsed to the due date = %d, want %d", n, d.Days)
	}
	if n := d.Elapsed(from, from, h); n != 0 {
		t.Errorf("nothing has elapsed on the first day, got %d", n)
	}
	if n := d.Elapsed(from, from.AddDays(-5), h); n != 0 {
		t.Errorf("a date before the start elapses nothing, got %d", n)
	}
}

func TestRemainingAndMissed(t *testing.T) {
	d, _ := permit.DeadlineFor(permit.StateSubmitted)
	h := holidays(t, 2025)
	from := calendar.MustNew(2025, 6, 30)
	due := d.Add(from, h)
	if got := d.Remaining(from, from, h); got != d.Days {
		t.Errorf("remaining on day one = %d", got)
	}
	if got := d.Remaining(from, due, h); got != 0 {
		t.Errorf("remaining on the due date = %d", got)
	}
	if d.Missed(from, due, h) {
		t.Error("the due date itself is not a miss")
	}
	if !d.Missed(from, due.AddDays(1), h) {
		t.Error("the day after is")
	}
	if got := d.Remaining(from, due.AddDays(7), h); got >= 0 {
		t.Errorf("a missed clock runs negative, got %d", got)
	}
}

func TestTheFullRunOfClocksIsLongerThanAnyOne(t *testing.T) {
	h := holidays(t, 2025)
	at := calendar.MustNew(2025, 3, 3)
	start := at
	for _, s := range []permit.State{
		permit.StateSubmitted, permit.StateAccepted, permit.StateSurveying,
		permit.StateEngineered, permit.StateEstimated, permit.StateAuthorised,
	} {
		d, ok := permit.DeadlineFor(s)
		if !ok {
			t.Fatalf("no deadline at %s", s)
		}
		at = d.Add(at, h)
	}
	if start.DaysUntil(at) < 120 {
		t.Errorf("the whole run takes %d days, which looks too short", start.DaysUntil(at))
	}
}
