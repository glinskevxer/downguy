// Package calendar holds the date type this system stores and the effective
// dating that hangs off it.
//
// Everything in joint use is dated. A tariff rate applies from a date. An
// attachment is authorised from a date. A make ready estimate is good until a
// date. The standard library time.Time carries a clock and a zone that none of
// those records have, and comparing two of them for equality is a trap, so the
// stored type is a plain calendar day.
package calendar

import (
	"errors"
	"fmt"
	"time"
)

// Date is a calendar day with no clock and no zone.
type Date struct {
	year  int
	month int
	day   int
}

// ErrBadDate is returned by ParseDate for anything it cannot read.
var ErrBadDate = errors.New("calendar: cannot read date")

// Zero is the empty date, used as an open ended bound.
var Zero = Date{}

// New builds a Date and reports whether the day exists.
func New(year, month, day int) (Date, error) {
	if month < 1 || month > 12 {
		return Zero, fmt.Errorf("%w: month %d", ErrBadDate, month)
	}
	if day < 1 || day > DaysInMonth(year, month) {
		return Zero, fmt.Errorf("%w: %d-%02d-%02d", ErrBadDate, year, month, day)
	}
	return Date{year: year, month: month, day: day}, nil
}

// MustNew builds a Date and panics if the day does not exist. It is for
// constants and fixtures, not for anything reading outside input.
func MustNew(year, month, day int) Date {
	d, err := New(year, month, day)
	if err != nil {
		panic(err)
	}
	return d
}

// ParseDate reads the ISO form the whole system writes.
func ParseDate(text string) (Date, error) {
	if len(text) != 10 || text[4] != '-' || text[7] != '-' {
		return Zero, fmt.Errorf("%w: %q is not yyyy-mm-dd", ErrBadDate, text)
	}
	y, ok := atoiN(text[0:4])
	if !ok {
		return Zero, fmt.Errorf("%w: year in %q", ErrBadDate, text)
	}
	m, ok := atoiN(text[5:7])
	if !ok {
		return Zero, fmt.Errorf("%w: month in %q", ErrBadDate, text)
	}
	d, ok := atoiN(text[8:10])
	if !ok {
		return Zero, fmt.Errorf("%w: day in %q", ErrBadDate, text)
	}
	return New(y, m, d)
}

func atoiN(s string) (int, bool) {
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, false
		}
		n = n*10 + int(s[i]-'0')
	}
	return n, true
}

// Year, Month and Day return the parts.
func (d Date) Year() int  { return d.year }
func (d Date) Month() int { return d.month }
func (d Date) Day() int   { return d.day }

// IsZero reports whether the date is the open ended bound.
func (d Date) IsZero() bool { return d == Zero }

// String prints the ISO form.
func (d Date) String() string {
	if d.IsZero() {
		return ""
	}
	return fmt.Sprintf("%04d-%02d-%02d", d.year, d.month, d.day)
}

// Compare returns a negative number when d is before other, zero when they are
// the same day, and a positive number when d is after.
func (d Date) Compare(other Date) int {
	if d.year != other.year {
		return d.year - other.year
	}
	if d.month != other.month {
		return d.month - other.month
	}
	return d.day - other.day
}

// Before, After and Equal are the readable forms of Compare.
func (d Date) Before(o Date) bool { return d.Compare(o) < 0 }
func (d Date) After(o Date) bool  { return d.Compare(o) > 0 }
func (d Date) Equal(o Date) bool  { return d.Compare(o) == 0 }

// Time converts to a UTC time at midnight, for the few places that have to
// hand a date to the standard library.
func (d Date) Time() time.Time {
	return time.Date(d.year, time.Month(d.month), d.day, 0, 0, 0, 0, time.UTC)
}

// FromTime takes the calendar day out of a time, in that time's own location.
func FromTime(t time.Time) Date {
	return Date{year: t.Year(), month: int(t.Month()), day: t.Day()}
}

// AddDays returns the date n days later, or earlier when n is negative.
func (d Date) AddDays(n int) Date { return FromTime(d.Time().AddDate(0, 0, n)) }

// AddMonths returns the date n months later, clamped to the end of the target
// month so that adding one month to 31 January lands on 28 February rather
// than wrapping into March.
func (d Date) AddMonths(n int) Date {
	total := d.year*12 + (d.month - 1) + n
	y := total / 12
	m := total%12 + 1
	if m < 1 {
		y--
		m += 12
	}
	day := d.day
	if max := DaysInMonth(y, m); day > max {
		day = max
	}
	return Date{year: y, month: m, day: day}
}

// AddYears returns the date n years later, clamping 29 February in a common
// year back to the 28th.
func (d Date) AddYears(n int) Date { return d.AddMonths(n * 12) }

// DaysUntil returns the whole days from d to other, negative when other is
// earlier.
func (d Date) DaysUntil(other Date) int {
	diff := other.Time().Sub(d.Time())
	return int(diff.Hours() / 24)
}

// Weekday returns the day of the week.
func (d Date) Weekday() time.Weekday { return d.Time().Weekday() }

// IsWeekend reports whether the date is a Saturday or a Sunday.
func (d Date) IsWeekend() bool {
	w := d.Weekday()
	return w == time.Saturday || w == time.Sunday
}

// IsLeap reports whether the year has a 29 February.
func IsLeap(year int) bool {
	return year%4 == 0 && (year%100 != 0 || year%400 == 0)
}

// DaysInMonth returns the length of the month.
func DaysInMonth(year, month int) int {
	switch month {
	case 1, 3, 5, 7, 8, 10, 12:
		return 31
	case 4, 6, 9, 11:
		return 30
	case 2:
		if IsLeap(year) {
			return 29
		}
		return 28
	}
	return 0
}

// DaysInYear returns 365 or 366.
func DaysInYear(year int) int {
	if IsLeap(year) {
		return 366
	}
	return 365
}

// Min and Max pick the earlier and the later of two dates, treating the zero
// date as unbounded in the direction that makes it a bound.
func Min(a, b Date) Date {
	if a.IsZero() {
		return b
	}
	if b.IsZero() {
		return a
	}
	if a.Before(b) {
		return a
	}
	return b
}

// Max returns the later of two dates.
func Max(a, b Date) Date {
	if a.IsZero() {
		return b
	}
	if b.IsZero() {
		return a
	}
	if a.After(b) {
		return a
	}
	return b
}
