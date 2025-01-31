package calendar

import (
	"errors"
	"fmt"
	"sort"
)

// Range is a span of calendar days, inclusive at both ends. A zero From means
// the range has always been in force and a zero To means it still is, which is
// how a tariff sheet is actually written.
type Range struct {
	From Date
	To   Date
}

// ErrBadRange is returned when a range ends before it starts.
var ErrBadRange = errors.New("calendar: range ends before it starts")

// Always is the range with no bounds at either end.
var Always = Range{}

// NewRange builds a range and checks the order of its bounds.
func NewRange(from, to Date) (Range, error) {
	r := Range{From: from, To: to}
	if !r.Valid() {
		return Range{}, fmt.Errorf("%w: %s to %s", ErrBadRange, from, to)
	}
	return r, nil
}

// Valid reports whether the bounds are in order.
func (r Range) Valid() bool {
	if r.From.IsZero() || r.To.IsZero() {
		return true
	}
	return !r.From.After(r.To)
}

// Contains reports whether the day falls inside the range.
func (r Range) Contains(d Date) bool {
	if !r.From.IsZero() && d.Before(r.From) {
		return false
	}
	if !r.To.IsZero() && d.After(r.To) {
		return false
	}
	return true
}

// Open reports whether the range has no end.
func (r Range) Open() bool { return r.To.IsZero() }

// Days returns the inclusive length of the range, or zero when either bound is
// open. A one day range measures one day, not nothing.
func (r Range) Days() int {
	if r.From.IsZero() || r.To.IsZero() {
		return 0
	}
	return r.From.DaysUntil(r.To) + 1
}

// Overlaps reports whether the two ranges share at least one day.
func (r Range) Overlaps(o Range) bool {
	if !r.To.IsZero() && !o.From.IsZero() && r.To.Before(o.From) {
		return false
	}
	if !o.To.IsZero() && !r.From.IsZero() && o.To.Before(r.From) {
		return false
	}
	return true
}

// Intersect returns the shared part of two ranges and reports whether there is
// one.
func (r Range) Intersect(o Range) (Range, bool) {
	if !r.Overlaps(o) {
		return Range{}, false
	}
	return Range{From: Max(r.From, o.From), To: Min(r.To, o.To)}, true
}

// String prints the range the way a tariff sheet writes it.
func (r Range) String() string {
	from := r.From.String()
	if from == "" {
		from = "open"
	}
	to := r.To.String()
	if to == "" {
		to = "open"
	}
	return from + " to " + to
}

// Effective pairs a value with the day it takes effect. A timeline of these is
// how every rate, factor and table in this system is versioned.
type Effective[T any] struct {
	From  Date
	Value T
}

// Timeline resolves an effective dated series as of a given day. Entries are
// kept sorted by their effective date and the one in force is the latest whose
// date is not after the day asked about.
type Timeline[T any] struct {
	entries []Effective[T]
}

// NewTimeline builds a timeline from entries in any order.
func NewTimeline[T any](entries []Effective[T]) *Timeline[T] {
	t := &Timeline[T]{entries: make([]Effective[T], len(entries))}
	copy(t.entries, entries)
	sort.SliceStable(t.entries, func(i, j int) bool {
		return t.entries[i].From.Before(t.entries[j].From)
	})
	return t
}

// Add inserts an entry and keeps the series ordered.
func (t *Timeline[T]) Add(from Date, value T) {
	t.entries = append(t.entries, Effective[T]{From: from, Value: value})
	sort.SliceStable(t.entries, func(i, j int) bool {
		return t.entries[i].From.Before(t.entries[j].From)
	})
}

// AsOf returns the value in force on the day, and reports whether there was
// one. A day before the first entry has no value at all, which is different
// from having a zero one.
func (t *Timeline[T]) AsOf(d Date) (T, bool) {
	var zero T
	idx := -1
	for i, e := range t.entries {
		if e.From.After(d) {
			break
		}
		idx = i
	}
	if idx < 0 {
		return zero, false
	}
	return t.entries[idx].Value, true
}

// Len returns the number of entries.
func (t *Timeline[T]) Len() int { return len(t.entries) }

// Entries returns a copy of the series in effective date order.
func (t *Timeline[T]) Entries() []Effective[T] {
	out := make([]Effective[T], len(t.entries))
	copy(out, t.entries)
	return out
}

// RateYear is the annual period a joint use rental is billed for. Utilities
// bill on a calendar year, so the year number is the whole key.
type RateYear int

// Range returns the calendar range the rate year covers.
func (y RateYear) Range() Range {
	return Range{From: MustNew(int(y), 1, 1), To: MustNew(int(y), 12, 31)}
}

// Days returns the length of the rate year.
func (y RateYear) Days() int { return DaysInYear(int(y)) }

// String prints the year.
func (y RateYear) String() string { return fmt.Sprintf("%d", int(y)) }

// RateYearOf returns the rate year a date falls in.
func RateYearOf(d Date) RateYear { return RateYear(d.Year()) }

// ProrationOf returns the numerator and denominator of the share of a rate
// year that an attachment was actually on the pole for. The denominator is the
// length of the year, so a leap year prorates on 366 days.
func ProrationOf(year RateYear, active Range) (int, int) {
	den := year.Days()
	shared, ok := year.Range().Intersect(active)
	if !ok {
		return 0, den
	}
	return shared.Days(), den
}
