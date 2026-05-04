package permit

import (
	"fmt"
	"sort"

	"github.com/glinskevxer/downguy/internal/calendar"
)

// A clock that has already run out is a report. A clock about to run out is a
// phone call, and the phone call is worth more. What this produces is the list
// somebody works through on a Monday morning: who to chase, about what, and
// how many days are left.

// Urgency is how soon something has to happen.
type Urgency string

// The urgencies, from a clock already blown to one nobody has to think about.
const (
	UrgencyOverdue Urgency = "overdue"
	UrgencyToday   Urgency = "today"
	UrgencySoon    Urgency = "soon"
	UrgencyLater   Urgency = "later"
)

var urgencyRank = map[Urgency]int{
	UrgencyOverdue: 0,
	UrgencyToday:   1,
	UrgencySoon:    2,
	UrgencyLater:   3,
}

// SoonDays is how far ahead a clock counts as soon.
const SoonDays = 5

// KnownUrgency reports whether the urgency is one the list handles.
func KnownUrgency(u Urgency) bool {
	_, ok := urgencyRank[u]
	return ok
}

// MoreUrgentThan reports whether u needs attention before other.
func (u Urgency) MoreUrgentThan(other Urgency) bool {
	a, oka := urgencyRank[u]
	b, okb := urgencyRank[other]
	if !oka || !okb {
		return false
	}
	return a < b
}

// UrgencyOf turns days remaining into an urgency.
func UrgencyOf(remaining int) Urgency {
	switch {
	case remaining < 0:
		return UrgencyOverdue
	case remaining == 0:
		return UrgencyToday
	case remaining <= SoonDays:
		return UrgencySoon
	default:
		return UrgencyLater
	}
}

// Reminder is one thing somebody has to do, and when.
type Reminder struct {
	Application string
	State       State
	Holder      Holder
	Party       string
	Due         calendar.Date
	Remaining   int
	Urgency     Urgency
	Cite        string
}

// String prints the reminder the way the list reads.
func (r Reminder) String() string {
	switch r.Urgency {
	case UrgencyOverdue:
		return fmt.Sprintf("%s: %s is %d days past due with the %s", r.Application, r.State, -r.Remaining, r.Holder)
	case UrgencyToday:
		return fmt.Sprintf("%s: %s is due today with the %s", r.Application, r.State, r.Holder)
	default:
		return fmt.Sprintf("%s: %s is due in %d days with the %s", r.Application, r.State, r.Remaining, r.Holder)
	}
}

// Remind builds the list for a set of applications as of a day. An
// application at a state nobody is timed on produces nothing, because there is
// no clock to chase against.
func Remind(apps []*Application, asOf calendar.Date, h Holidays) ([]Reminder, error) {
	if asOf.IsZero() {
		return nil, fmt.Errorf("%w: a reminder list needs a date", ErrBadTransition)
	}
	var out []Reminder
	for _, app := range apps {
		if app == nil {
			return nil, fmt.Errorf("%w: a nil application in the list", ErrBadTransition)
		}
		if !app.Live() {
			continue
		}
		deadline, ok := DeadlineFor(app.State)
		if !ok {
			continue
		}
		entered, ok := app.EnteredAt(app.State)
		if !ok {
			continue
		}
		remaining := deadline.Remaining(entered, asOf, h)
		party := app.Owner
		if deadline.Holder == HolderAttacher {
			party = app.Applicant
		}
		out = append(out, Reminder{
			Application: app.ID, State: app.State, Holder: deadline.Holder,
			Party: party, Due: deadline.Add(entered, h),
			Remaining: remaining, Urgency: UrgencyOf(remaining), Cite: deadline.Cite,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Urgency != out[j].Urgency {
			return out[i].Urgency.MoreUrgentThan(out[j].Urgency)
		}
		if out[i].Remaining != out[j].Remaining {
			return out[i].Remaining < out[j].Remaining
		}
		return out[i].Application < out[j].Application
	})
	return out, nil
}

// Chasing returns the reminders that need somebody to do something now, which
// is everything overdue or due today.
func Chasing(reminders []Reminder) []Reminder {
	var out []Reminder
	for _, r := range reminders {
		if r.Urgency == UrgencyOverdue || r.Urgency == UrgencyToday {
			out = append(out, r)
		}
	}
	return out
}

// ForParty returns the reminders one company is on the hook for.
func ForParty(reminders []Reminder, party string) []Reminder {
	var out []Reminder
	for _, r := range reminders {
		if r.Party == party {
			out = append(out, r)
		}
	}
	return out
}

// Parties returns every company with something to do, sorted.
func Parties(reminders []Reminder) []string {
	seen := map[string]bool{}
	var out []string
	for _, r := range reminders {
		if r.Party == "" || seen[r.Party] {
			continue
		}
		seen[r.Party] = true
		out = append(out, r.Party)
	}
	sort.Strings(out)
	return out
}
