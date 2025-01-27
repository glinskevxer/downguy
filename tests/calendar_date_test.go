package tests

import (
	"errors"
	"testing"
	"time"

	"github.com/glinskevxer/downguy/internal/calendar"
)

func TestNewRejectsImpossibleDays(t *testing.T) {
	bad := [][3]int{{2025, 0, 1}, {2025, 13, 1}, {2025, 2, 29}, {2025, 4, 31}, {2025, 1, 0}}
	for _, b := range bad {
		if _, err := calendar.New(b[0], b[1], b[2]); err == nil {
			t.Errorf("New%v should have failed", b)
		} else if !errors.Is(err, calendar.ErrBadDate) {
			t.Errorf("New%v gave %v", b, err)
		}
	}
	if _, err := calendar.New(2024, 2, 29); err != nil {
		t.Errorf("2024-02-29 is a real day: %v", err)
	}
}

func TestParseDateRoundTrips(t *testing.T) {
	for _, s := range []string{"2025-01-20", "2024-02-29", "1999-12-31"} {
		d, err := calendar.ParseDate(s)
		if err != nil {
			t.Errorf("ParseDate(%q): %v", s, err)
			continue
		}
		if d.String() != s {
			t.Errorf("round trip of %q gave %q", s, d.String())
		}
	}
}

func TestParseDateRejectsJunk(t *testing.T) {
	for _, s := range []string{"", "2025-1-2", "20250120", "2025/01/20", "abcd-ef-gh", "2025-01-20T00:00:00"} {
		if _, err := calendar.ParseDate(s); err == nil {
			t.Errorf("ParseDate(%q) should have failed", s)
		}
	}
}

func TestDateComparison(t *testing.T) {
	a := calendar.MustNew(2025, 3, 1)
	b := calendar.MustNew(2025, 3, 2)
	c := calendar.MustNew(2026, 1, 1)
	if !a.Before(b) || !b.After(a) || !a.Equal(a) {
		t.Fatal("ordering is wrong")
	}
	if a.Compare(c) >= 0 {
		t.Error("2025 should sort before 2026")
	}
	if b.Compare(a) <= 0 {
		t.Error("later day should compare greater")
	}
}

func TestAddMonthsClampsToMonthEnd(t *testing.T) {
	cases := []struct {
		from string
		n    int
		want string
	}{
		{"2025-01-31", 1, "2025-02-28"},
		{"2024-01-31", 1, "2024-02-29"},
		{"2025-01-15", 12, "2026-01-15"},
		{"2025-03-31", -1, "2025-02-28"},
		{"2025-01-15", -13, "2023-12-15"},
		{"2025-12-31", 1, "2026-01-31"},
	}
	for _, c := range cases {
		from, err := calendar.ParseDate(c.from)
		if err != nil {
			t.Fatal(err)
		}
		if got := from.AddMonths(c.n).String(); got != c.want {
			t.Errorf("%s +%d months = %s, want %s", c.from, c.n, got, c.want)
		}
	}
}

func TestAddDaysAndDaysUntil(t *testing.T) {
	d := calendar.MustNew(2025, 2, 27)
	if got := d.AddDays(2).String(); got != "2025-03-01" {
		t.Errorf("add days = %s", got)
	}
	if got := d.AddDays(-58).String(); got != "2024-12-31" {
		t.Errorf("subtract days = %s", got)
	}
	a := calendar.MustNew(2025, 1, 1)
	b := calendar.MustNew(2026, 1, 1)
	if got := a.DaysUntil(b); got != 365 {
		t.Errorf("days until = %d, want 365", got)
	}
	if got := b.DaysUntil(a); got != -365 {
		t.Errorf("reverse days = %d", got)
	}
}

func TestAddYearsClampsLeapDay(t *testing.T) {
	d := calendar.MustNew(2024, 2, 29)
	if got := d.AddYears(1).String(); got != "2025-02-28" {
		t.Errorf("leap plus one year = %s", got)
	}
	if got := d.AddYears(4).String(); got != "2028-02-29" {
		t.Errorf("leap plus four years = %s", got)
	}
}

func TestCalendarHelpers(t *testing.T) {
	if !calendar.IsLeap(2024) || calendar.IsLeap(2025) || calendar.IsLeap(1900) || !calendar.IsLeap(2000) {
		t.Error("leap year rule is wrong")
	}
	if calendar.DaysInMonth(2025, 2) != 28 || calendar.DaysInMonth(2024, 2) != 29 {
		t.Error("february length is wrong")
	}
	if calendar.DaysInMonth(2025, 4) != 30 || calendar.DaysInMonth(2025, 7) != 31 {
		t.Error("month length is wrong")
	}
	if calendar.DaysInMonth(2025, 13) != 0 {
		t.Error("month 13 should measure zero")
	}
	if calendar.DaysInYear(2024) != 366 || calendar.DaysInYear(2025) != 365 {
		t.Error("year length is wrong")
	}
}

func TestWeekdayAndWeekend(t *testing.T) {
	sat := calendar.MustNew(2025, 1, 18)
	sun := calendar.MustNew(2025, 1, 19)
	mon := calendar.MustNew(2025, 1, 20)
	if sat.Weekday() != time.Saturday || !sat.IsWeekend() {
		t.Error("saturday")
	}
	if sun.Weekday() != time.Sunday || !sun.IsWeekend() {
		t.Error("sunday")
	}
	if mon.Weekday() != time.Monday || mon.IsWeekend() {
		t.Error("monday")
	}
}

func TestZeroDateIsUnboundedInMinMax(t *testing.T) {
	a := calendar.MustNew(2025, 6, 1)
	if !calendar.Min(calendar.Zero, a).Equal(a) {
		t.Error("min with zero should give the real date")
	}
	if !calendar.Max(calendar.Zero, a).Equal(a) {
		t.Error("max with zero should give the real date")
	}
	b := calendar.MustNew(2025, 7, 1)
	if !calendar.Min(a, b).Equal(a) || !calendar.Max(a, b).Equal(b) {
		t.Error("min or max of two real dates is wrong")
	}
	if !calendar.Zero.IsZero() || calendar.Zero.String() != "" {
		t.Error("zero date should print empty")
	}
	if !calendar.FromTime(a.Time()).Equal(a) {
		t.Error("time round trip")
	}
}
