package tests

import (
	"testing"

	"github.com/glinskevxer/downguy/internal/calendar"
)

func rng(t *testing.T, from, to string) calendar.Range {
	t.Helper()
	var r calendar.Range
	if from != "" {
		d, err := calendar.ParseDate(from)
		if err != nil {
			t.Fatal(err)
		}
		r.From = d
	}
	if to != "" {
		d, err := calendar.ParseDate(to)
		if err != nil {
			t.Fatal(err)
		}
		r.To = d
	}
	return r
}

func TestRangeValidity(t *testing.T) {
	if _, err := calendar.NewRange(calendar.MustNew(2025, 5, 1), calendar.MustNew(2025, 4, 1)); err == nil {
		t.Error("backwards range should be rejected")
	}
	if _, err := calendar.NewRange(calendar.MustNew(2025, 5, 1), calendar.MustNew(2025, 5, 1)); err != nil {
		t.Errorf("single day range: %v", err)
	}
	if !calendar.Always.Valid() || !calendar.Always.Open() {
		t.Error("the unbounded range should be valid and open")
	}
}

func TestRangeContains(t *testing.T) {
	r := rng(t, "2025-03-01", "2025-03-31")
	if !r.Contains(calendar.MustNew(2025, 3, 1)) || !r.Contains(calendar.MustNew(2025, 3, 31)) {
		t.Error("both ends are inside the range")
	}
	if r.Contains(calendar.MustNew(2025, 2, 28)) || r.Contains(calendar.MustNew(2025, 4, 1)) {
		t.Error("outside days should not be contained")
	}
	open := rng(t, "2025-03-01", "")
	if !open.Contains(calendar.MustNew(2030, 1, 1)) {
		t.Error("an open range should still be in force later")
	}
	if open.Contains(calendar.MustNew(2024, 1, 1)) {
		t.Error("an open range does not reach back before its start")
	}
}

func TestRangeDaysIsInclusive(t *testing.T) {
	if got := rng(t, "2025-03-01", "2025-03-01").Days(); got != 1 {
		t.Errorf("one day range = %d", got)
	}
	if got := rng(t, "2025-01-01", "2025-12-31").Days(); got != 365 {
		t.Errorf("year = %d", got)
	}
	if got := rng(t, "2024-01-01", "2024-12-31").Days(); got != 366 {
		t.Errorf("leap year = %d", got)
	}
	if got := rng(t, "2025-01-01", "").Days(); got != 0 {
		t.Errorf("open range should measure zero, got %d", got)
	}
}

func TestRangeOverlapAndIntersect(t *testing.T) {
	a := rng(t, "2025-01-01", "2025-06-30")
	b := rng(t, "2025-06-30", "2025-12-31")
	c := rng(t, "2025-07-01", "2025-12-31")
	if !a.Overlaps(b) {
		t.Error("ranges sharing one day overlap")
	}
	if a.Overlaps(c) {
		t.Error("adjacent ranges do not overlap")
	}
	got, ok := a.Intersect(b)
	if !ok || got.Days() != 1 || got.From.String() != "2025-06-30" {
		t.Errorf("intersect = %v ok=%v", got, ok)
	}
	if _, ok := a.Intersect(c); ok {
		t.Error("no intersection expected")
	}
	if got := a.String(); got != "2025-01-01 to 2025-06-30" {
		t.Errorf("String = %q", got)
	}
	if got := rng(t, "", "").String(); got != "open to open" {
		t.Errorf("open String = %q", got)
	}
}

func TestTimelineResolvesAsOf(t *testing.T) {
	tl := calendar.NewTimeline([]calendar.Effective[int]{
		{From: calendar.MustNew(2026, 1, 1), Value: 30},
		{From: calendar.MustNew(2024, 1, 1), Value: 10},
		{From: calendar.MustNew(2025, 1, 1), Value: 20},
	})
	if tl.Len() != 3 {
		t.Fatalf("len = %d", tl.Len())
	}
	if _, ok := tl.AsOf(calendar.MustNew(2023, 12, 31)); ok {
		t.Error("nothing is in force before the first entry")
	}
	cases := []struct {
		day  string
		want int
	}{
		{"2024-01-01", 10},
		{"2024-12-31", 10},
		{"2025-01-01", 20},
		{"2025-12-31", 20},
		{"2026-01-01", 30},
		{"2030-06-15", 30},
	}
	for _, c := range cases {
		d, err := calendar.ParseDate(c.day)
		if err != nil {
			t.Fatal(err)
		}
		got, ok := tl.AsOf(d)
		if !ok || got != c.want {
			t.Errorf("as of %s = %d ok=%v, want %d", c.day, got, ok, c.want)
		}
	}
	entries := tl.Entries()
	if entries[0].Value != 10 || entries[2].Value != 30 {
		t.Error("entries should come back in effective date order")
	}
	entries[0].Value = 99
	if again, _ := tl.AsOf(calendar.MustNew(2024, 6, 1)); again != 10 {
		t.Error("Entries should hand back a copy")
	}
}

func TestTimelineAddKeepsOrder(t *testing.T) {
	tl := calendar.NewTimeline([]calendar.Effective[string]{})
	tl.Add(calendar.MustNew(2025, 6, 1), "second")
	tl.Add(calendar.MustNew(2025, 1, 1), "first")
	if got, _ := tl.AsOf(calendar.MustNew(2025, 3, 1)); got != "first" {
		t.Errorf("as of march = %q", got)
	}
	if got, _ := tl.AsOf(calendar.MustNew(2025, 8, 1)); got != "second" {
		t.Errorf("as of august = %q", got)
	}
}

func TestRateYearAndProration(t *testing.T) {
	y := calendar.RateYear(2025)
	if y.Days() != 365 || y.String() != "2025" {
		t.Errorf("rate year = %d days %q", y.Days(), y.String())
	}
	if calendar.RateYear(2024).Days() != 366 {
		t.Error("2024 is a leap year")
	}
	if calendar.RateYearOf(calendar.MustNew(2025, 7, 4)) != 2025 {
		t.Error("rate year of a date")
	}
	num, den := calendar.ProrationOf(y, rng(t, "2025-01-01", ""))
	if num != 365 || den != 365 {
		t.Errorf("full year proration = %d/%d", num, den)
	}
	num, den = calendar.ProrationOf(y, rng(t, "2025-07-01", "2025-12-31"))
	if num != 184 || den != 365 {
		t.Errorf("half year proration = %d/%d", num, den)
	}
	num, den = calendar.ProrationOf(y, rng(t, "2026-01-01", "2026-12-31"))
	if num != 0 || den != 365 {
		t.Errorf("out of year proration = %d/%d", num, den)
	}
	num, _ = calendar.ProrationOf(calendar.RateYear(2024), rng(t, "2024-01-01", "2024-12-31"))
	if num != 366 {
		t.Errorf("leap year proration numerator = %d", num)
	}
}
