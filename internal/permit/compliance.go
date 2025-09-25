package permit

import (
	"fmt"
	"sort"

	"github.com/glinskevxer/downguy/internal/calendar"
)

// Interval is one stretch the application sat at one state, with the clock
// that applied and what actually happened to it.
type Interval struct {
	State    State
	Holder   Holder
	Entered  calendar.Date
	Left     calendar.Date
	Open     bool
	Deadline Deadline
	Timed    bool
	Due      calendar.Date
	Used     int
	Overrun  int
}

// Late reports whether the clock ran out on this interval.
func (i Interval) Late() bool { return i.Timed && i.Overrun > 0 }

// Compliance is the whole timing picture for one application.
type Compliance struct {
	Application string
	AsOf        calendar.Date
	Intervals   []Interval
	Late        []Interval
	ByHolder    map[Holder]int
}

// Assess walks an application's history and works out where the time went.
// Intervals run from the day a state was entered to the day it was left; the
// state the application is in now runs to the day being asked about.
func Assess(a *Application, asOf calendar.Date, h Holidays) (*Compliance, error) {
	if a == nil {
		return nil, fmt.Errorf("%w: no application", ErrBadTransition)
	}
	if asOf.IsZero() {
		return nil, fmt.Errorf("%w: %s needs a date to be assessed against", ErrBadTransition, a.ID)
	}
	if len(a.History) == 0 {
		return nil, fmt.Errorf("%w: %s has no history", ErrBadTransition, a.ID)
	}
	c := &Compliance{Application: a.ID, AsOf: asOf, ByHolder: map[Holder]int{}}

	for i, e := range a.History {
		entered := e.On
		left := asOf
		open := true
		if i+1 < len(a.History) {
			left = a.History[i+1].On
			open = false
		}
		if left.Before(entered) {
			left = entered
		}
		in := Interval{
			State: e.To, Holder: e.To.HeldBy(),
			Entered: entered, Left: left, Open: open,
		}
		if d, ok := DeadlineFor(e.To); ok {
			in.Deadline = d
			in.Timed = true
			in.Due = d.Add(entered, h)
			in.Used = d.Elapsed(entered, left, h)
			if in.Used > d.Days {
				in.Overrun = in.Used - d.Days
			}
		} else {
			in.Used = entered.DaysUntil(left)
		}
		c.Intervals = append(c.Intervals, in)
		c.ByHolder[in.Holder] += entered.DaysUntil(left)
		if in.Late() {
			c.Late = append(c.Late, in)
		}
	}
	sort.SliceStable(c.Late, func(i, j int) bool { return c.Late[i].Overrun > c.Late[j].Overrun })
	return c, nil
}

// OnTime reports whether every clock was met.
func (c *Compliance) OnTime() bool { return len(c.Late) == 0 }

// WorstOverrun returns the interval that ran furthest over, and reports
// whether anything did.
func (c *Compliance) WorstOverrun() (Interval, bool) {
	if len(c.Late) == 0 {
		return Interval{}, false
	}
	return c.Late[0], true
}

// DaysWith returns how many calendar days one side has held the application.
func (c *Compliance) DaysWith(h Holder) int { return c.ByHolder[h] }

// LateFor returns the overruns charged to one side, worst first.
func (c *Compliance) LateFor(h Holder) []Interval {
	var out []Interval
	for _, in := range c.Late {
		if in.Holder == h {
			out = append(out, in)
		}
	}
	return out
}

// Current returns the interval the application is sitting in now, and reports
// whether it is still open.
func (c *Compliance) Current() (Interval, bool) {
	if len(c.Intervals) == 0 {
		return Interval{}, false
	}
	last := c.Intervals[len(c.Intervals)-1]
	return last, last.Open
}

// Summary writes the line a delay report opens with.
func (c *Compliance) Summary() string {
	verdict := "on time"
	if !c.OnTime() {
		verdict = fmt.Sprintf("%d clocks missed", len(c.Late))
	}
	return fmt.Sprintf("%s as of %s: %s, %d days with the utility and %d with the attacher",
		c.Application, c.AsOf, verdict,
		c.DaysWith(HolderUtility), c.DaysWith(HolderAttacher))
}
