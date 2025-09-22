package permit

import (
	"fmt"
	"sort"

	"github.com/glinskevxer/downguy/internal/calendar"
)

// Deadline is how long one side has at one state. The regulator writes these
// in business days, which is why the countdown skips weekends and the holiday
// list is a field rather than a constant.
type Deadline struct {
	State    State
	Holder   Holder
	Days     int
	Business bool
	Cite     string
}

var deadlines = map[State]Deadline{
	StateSubmitted:   {State: StateSubmitted, Holder: HolderUtility, Days: 10, Business: true, Cite: "1.1411(c)(1)"},
	StateAccepted:    {State: StateAccepted, Holder: HolderUtility, Days: 45, Business: false, Cite: "1.1411(c)(1)"},
	StateSurveying:   {State: StateSurveying, Holder: HolderUtility, Days: 45, Business: false, Cite: "1.1411(c)(1)"},
	StateEngineered:  {State: StateEngineered, Holder: HolderUtility, Days: 14, Business: true, Cite: "1.1411(c)(2)"},
	StateEstimated:   {State: StateEstimated, Holder: HolderAttacher, Days: 14, Business: true, Cite: "1.1411(c)(3)"},
	StateAuthorised:  {State: StateAuthorised, Holder: HolderUtility, Days: 15, Business: true, Cite: "1.1411(d)"},
	StateScheduled:   {State: StateScheduled, Holder: HolderUtility, Days: 60, Business: false, Cite: "1.1411(e)"},
	StateConstructed: {State: StateConstructed, Holder: HolderUtility, Days: 30, Business: false, Cite: "1.1411(f)"},
	StateInspected:   {State: StateInspected, Holder: HolderUtility, Days: 10, Business: true, Cite: "1.1411(f)"},
}

// DeadlineFor returns the clock that runs at a state, and reports whether one
// does. A state with no deadline is one nobody is being timed at.
func DeadlineFor(s State) (Deadline, bool) {
	d, ok := deadlines[s]
	return d, ok
}

// TimedStates returns the states that carry a clock, sorted.
func TimedStates() []State {
	out := make([]State, 0, len(deadlines))
	for s := range deadlines {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Holidays is the set of days that do not count against a business day clock.
type Holidays map[string]bool

// NewHolidays builds a holiday set from ISO dates.
func NewHolidays(dates ...string) (Holidays, error) {
	h := Holidays{}
	for _, d := range dates {
		parsed, err := calendar.ParseDate(d)
		if err != nil {
			return nil, err
		}
		h[parsed.String()] = true
	}
	return h, nil
}

// Contains reports whether a day is a holiday.
func (h Holidays) Contains(d calendar.Date) bool { return h[d.String()] }

// Working reports whether a day counts against a business day clock.
func (h Holidays) Working(d calendar.Date) bool { return !d.IsWeekend() && !h.Contains(d) }

// Add returns the date n days after the start under this deadline, counting
// business days when the deadline is written in them. The start day itself is
// not counted, which is how every one of these rules is written.
func (d Deadline) Add(from calendar.Date, h Holidays) calendar.Date {
	if !d.Business {
		return from.AddDays(d.Days)
	}
	at := from
	for counted := 0; counted < d.Days; {
		at = at.AddDays(1)
		if h.Working(at) {
			counted++
		}
	}
	return at
}

// Elapsed returns how many days of the clock have run between two dates,
// counting the same way the deadline does.
func (d Deadline) Elapsed(from, to calendar.Date, h Holidays) int {
	if to.Before(from) {
		return 0
	}
	if !d.Business {
		return from.DaysUntil(to)
	}
	count := 0
	for at := from.AddDays(1); !at.After(to); at = at.AddDays(1) {
		if h.Working(at) {
			count++
		}
	}
	return count
}

// Remaining returns how many days are left on the clock as of a day, which
// goes negative once the deadline has passed.
func (d Deadline) Remaining(from, asOf calendar.Date, h Holidays) int {
	return d.Days - d.Elapsed(from, asOf, h)
}

// Missed reports whether the clock has run out as of a day.
func (d Deadline) Missed(from, asOf calendar.Date, h Holidays) bool {
	return asOf.After(d.Add(from, h))
}

// String prints the deadline the way the agreement writes it.
func (d Deadline) String() string {
	kind := "calendar"
	if d.Business {
		kind = "business"
	}
	return fmt.Sprintf("%s: %d %s days on the %s (%s)", d.State, d.Days, kind, d.Holder, d.Cite)
}

// StandardHolidays returns the days this utility observes in a year, and
// reports whether it has a list for that year.
func StandardHolidays(year int) (Holidays, bool) {
	dates, ok := observed[year]
	if !ok {
		return nil, false
	}
	h, err := NewHolidays(dates...)
	if err != nil {
		return nil, false
	}
	return h, true
}

var observed = map[int][]string{
	2025: {
		"2025-01-01", "2025-01-20", "2025-05-26", "2025-07-04",
		"2025-09-01", "2025-11-27", "2025-11-28", "2025-12-25",
	},
	2026: {
		"2026-01-01", "2026-01-19", "2026-05-25", "2026-07-03",
		"2026-09-07", "2026-11-26", "2026-11-27", "2026-12-25",
	},
}

// ObservedYears returns the years the holiday list covers, oldest first.
func ObservedYears() []int {
	out := make([]int, 0, len(observed))
	for y := range observed {
		out = append(out, y)
	}
	sort.Ints(out)
	return out
}
