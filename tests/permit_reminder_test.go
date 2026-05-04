package tests

import (
	"testing"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/permit"
	"github.com/glinskevxer/downguy/internal/plant"
)

func application(t *testing.T, id, applicant string, steps [][2]string) *permit.Application {
	t.Helper()
	app, err := permit.New(id, applicant, "SUMMIT VALLEY ELECTRIC",
		[]plant.PoleID{"NW-441829"}, calendar.MustNew(2025, 3, 3), "j.aldana")
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range steps {
		state, err := permit.ParseState(step[0])
		if err != nil {
			t.Fatal(err)
		}
		on, err := calendar.ParseDate(step[1])
		if err != nil {
			t.Fatal(err)
		}
		if err := app.MoveTo(state, on, "d.orme", ""); err != nil {
			t.Fatal(err)
		}
	}
	return app
}

func TestUrgencyOfDaysRemaining(t *testing.T) {
	t.Parallel()
	cases := map[int]permit.Urgency{
		-9: permit.UrgencyOverdue,
		-1: permit.UrgencyOverdue,
		0:  permit.UrgencyToday,
		1:  permit.UrgencySoon,
		5:  permit.UrgencySoon,
		6:  permit.UrgencyLater,
		40: permit.UrgencyLater,
	}
	for remaining, want := range cases {
		if got := permit.UrgencyOf(remaining); got != want {
			t.Errorf("UrgencyOf(%d) = %q, want %q", remaining, got, want)
		}
	}
	if !permit.UrgencyOverdue.MoreUrgentThan(permit.UrgencyToday) {
		t.Error("overdue beats today")
	}
	if !permit.UrgencyToday.MoreUrgentThan(permit.UrgencyLater) {
		t.Error("today beats later")
	}
	if permit.UrgencyLater.MoreUrgentThan(permit.UrgencyOverdue) {
		t.Error("later beats nothing")
	}
	if permit.Urgency("panic").MoreUrgentThan(permit.UrgencyLater) {
		t.Error("an unknown urgency beats nothing")
	}
	if !permit.KnownUrgency(permit.UrgencySoon) || permit.KnownUrgency(permit.Urgency("panic")) {
		t.Error("known urgency check")
	}
}

func TestRemindSortsTheWorstFirst(t *testing.T) {
	t.Parallel()
	h := holidays(t, 2025)
	late := application(t, "APP-1042", "MERIDIAN FIBER", [][2]string{{"submitted", "2025-03-05"}})
	fresh := application(t, "APP-1043", "CASCADE BROADBAND", [][2]string{{"submitted", "2025-05-01"}})
	got, err := permit.Remind([]*permit.Application{fresh, late}, calendar.MustNew(2025, 5, 6), h)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("reminders = %v", got)
	}
	if got[0].Application != "APP-1042" || got[0].Urgency != permit.UrgencyOverdue {
		t.Errorf("the overdue one comes first, got %+v", got[0])
	}
	if got[0].Remaining >= 0 {
		t.Errorf("an overdue clock runs negative, got %d", got[0].Remaining)
	}
	if got[0].Holder != permit.HolderUtility || got[0].Party != "SUMMIT VALLEY ELECTRIC" {
		t.Errorf("a submitted application is with the utility, got %+v", got[0])
	}
	if got[0].Cite == "" || got[0].Due.IsZero() {
		t.Errorf("reminder = %+v", got[0])
	}
	if got[0].String() == "" || got[1].String() == "" {
		t.Error("String should say something")
	}
}

func TestRemindNamesTheAttacherWhenTheClockIsTheirs(t *testing.T) {
	t.Parallel()
	h := holidays(t, 2025)
	app := application(t, "APP-1042", "MERIDIAN FIBER", [][2]string{
		{"submitted", "2025-03-05"},
		{"accepted", "2025-03-12"},
		{"surveying", "2025-03-20"},
		{"engineered", "2025-04-01"},
		{"estimated", "2025-04-10"},
	})
	got, err := permit.Remind([]*permit.Application{app}, calendar.MustNew(2025, 4, 14), h)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("reminders = %v", got)
	}
	if got[0].Holder != permit.HolderAttacher || got[0].Party != "MERIDIAN FIBER" {
		t.Errorf("the estimate clock is the attacher's, got %+v", got[0])
	}
	if got[0].Urgency == permit.UrgencyOverdue {
		t.Errorf("four days in is not overdue on a fourteen business day clock, got %+v", got[0])
	}
}

func TestRemindSkipsWhatHasNoClock(t *testing.T) {
	t.Parallel()
	h := holidays(t, 2025)
	draft := application(t, "APP-1042", "MERIDIAN FIBER", nil)
	closed := application(t, "APP-1043", "CASCADE BROADBAND", [][2]string{{"withdrawn", "2025-04-01"}})
	got, err := permit.Remind([]*permit.Application{draft, closed}, calendar.MustNew(2025, 5, 6), h)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("nobody is timed on a draft and a withdrawn one is finished, got %v", got)
	}
}

func TestChasingAndGrouping(t *testing.T) {
	t.Parallel()
	h := holidays(t, 2025)
	late := application(t, "APP-1042", "MERIDIAN FIBER", [][2]string{{"submitted", "2025-03-05"}})
	fresh := application(t, "APP-1043", "CASCADE BROADBAND", [][2]string{{"submitted", "2025-05-05"}})
	got, err := permit.Remind([]*permit.Application{late, fresh}, calendar.MustNew(2025, 5, 6), h)
	if err != nil {
		t.Fatal(err)
	}
	chasing := permit.Chasing(got)
	if len(chasing) != 1 || chasing[0].Application != "APP-1042" {
		t.Errorf("chasing = %v", chasing)
	}
	parties := permit.Parties(got)
	if len(parties) != 1 || parties[0] != "SUMMIT VALLEY ELECTRIC" {
		t.Errorf("both clocks are the utility's, got %v", parties)
	}
	if len(permit.ForParty(got, "SUMMIT VALLEY ELECTRIC")) != 2 {
		t.Errorf("for party = %v", permit.ForParty(got, "SUMMIT VALLEY ELECTRIC"))
	}
	if len(permit.ForParty(got, "NOBODY")) != 0 {
		t.Error("nobody has nothing to do")
	}
}

func TestRemindRejectsBadInput(t *testing.T) {
	t.Parallel()
	h := holidays(t, 2025)
	if _, err := permit.Remind(nil, calendar.Zero, h); err == nil {
		t.Error("no date should be rejected")
	}
	if _, err := permit.Remind([]*permit.Application{nil}, calendar.MustNew(2025, 5, 6), h); err == nil {
		t.Error("a nil application should be rejected")
	}
	got, err := permit.Remind(nil, calendar.MustNew(2025, 5, 6), h)
	if err != nil || got != nil {
		t.Errorf("no applications, no reminders, got %v %v", got, err)
	}
}
