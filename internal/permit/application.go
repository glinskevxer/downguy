package permit

import (
	"fmt"
	"sort"
	"strings"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/plant"
)

// Event is one step in an application's life. The history is append only:
// nothing edits a past event, because the dates on them are what a regulator
// reads when an attacher complains about a delay.
type Event struct {
	From State
	To   State
	On   calendar.Date
	By   string
	Note string
}

// Application is one attacher's request to go up on a set of poles.
type Application struct {
	ID        string
	Applicant string
	Owner     string
	Poles     []plant.PoleID
	State     State
	History   []Event
}

// New starts an application in draft.
func New(id, applicant, owner string, poles []plant.PoleID, on calendar.Date, by string) (*Application, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("%w: an application with no id", ErrBadTransition)
	}
	if strings.TrimSpace(applicant) == "" {
		return nil, fmt.Errorf("%w: %s has no applicant", ErrBadTransition, id)
	}
	if strings.TrimSpace(owner) == "" {
		return nil, fmt.Errorf("%w: %s has no pole owner", ErrBadTransition, id)
	}
	if len(poles) == 0 {
		return nil, fmt.Errorf("%w: %s names no poles", ErrBadTransition, id)
	}
	if on.IsZero() {
		return nil, fmt.Errorf("%w: %s has no start date", ErrBadTransition, id)
	}
	seen := map[plant.PoleID]bool{}
	ordered := make([]plant.PoleID, 0, len(poles))
	for _, p := range poles {
		if _, err := plant.ParsePoleID(string(p)); err != nil {
			return nil, err
		}
		if seen[p] {
			continue
		}
		seen[p] = true
		ordered = append(ordered, p)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	return &Application{
		ID: id, Applicant: applicant, Owner: owner, Poles: ordered,
		State: StateDraft,
		History: []Event{{
			From: StateDraft, To: StateDraft, On: on, By: by,
			Note: "application opened",
		}},
	}, nil
}

// MoveTo records a transition. It refuses a move the machine does not allow
// and a date earlier than the last event, because a history that goes
// backwards is worse than no history.
func (a *Application) MoveTo(to State, on calendar.Date, by, note string) error {
	if !KnownState(to) {
		return fmt.Errorf("%w: %q is not a state", ErrBadTransition, to)
	}
	if !a.State.CanMoveTo(to) {
		return fmt.Errorf("%w: %s cannot go from %s to %s", ErrBadTransition, a.ID, a.State, to)
	}
	if on.IsZero() {
		return fmt.Errorf("%w: %s moving to %s has no date", ErrBadTransition, a.ID, to)
	}
	if last, ok := a.Last(); ok && on.Before(last.On) {
		return fmt.Errorf("%w: %s reached %s on %s, so it cannot reach %s on %s",
			ErrBadTransition, a.ID, last.To, last.On, to, on)
	}
	if strings.TrimSpace(by) == "" {
		return fmt.Errorf("%w: %s moving to %s names nobody", ErrBadTransition, a.ID, to)
	}
	a.History = append(a.History, Event{From: a.State, To: to, On: on, By: by, Note: note})
	a.State = to
	return nil
}

// Last returns the most recent event.
func (a *Application) Last() (Event, bool) {
	if len(a.History) == 0 {
		return Event{}, false
	}
	return a.History[len(a.History)-1], true
}

// EnteredAt returns when the application reached a state, and reports whether
// it ever did. A state reached twice reports the most recent time.
func (a *Application) EnteredAt(s State) (calendar.Date, bool) {
	for i := len(a.History) - 1; i >= 0; i-- {
		if a.History[i].To == s {
			return a.History[i].On, true
		}
	}
	return calendar.Zero, false
}

// Opened returns the date the application was started.
func (a *Application) Opened() calendar.Date {
	if len(a.History) == 0 {
		return calendar.Zero
	}
	return a.History[0].On
}

// Age returns how many days the application has been open as of a date.
func (a *Application) Age(asOf calendar.Date) int {
	opened := a.Opened()
	if opened.IsZero() || asOf.IsZero() {
		return 0
	}
	return opened.DaysUntil(asOf)
}

// Live reports whether the application is still being worked.
func (a *Application) Live() bool { return a.State.Live() }

// HeldBy returns which side is holding it now.
func (a *Application) HeldBy() Holder { return a.State.HeldBy() }

// PoleCount returns how many poles the application covers.
func (a *Application) PoleCount() int { return len(a.Poles) }

// Covers reports whether a pole is on the application.
func (a *Application) Covers(id plant.PoleID) bool {
	for _, p := range a.Poles {
		if p == id {
			return true
		}
	}
	return false
}

// Path returns the states the application has actually passed through, in
// order.
func (a *Application) Path() []State {
	if len(a.History) == 0 {
		return nil
	}
	out := []State{a.History[0].To}
	for _, e := range a.History[1:] {
		out = append(out, e.To)
	}
	return out
}

// String prints the application header.
func (a *Application) String() string {
	return fmt.Sprintf("%s %s on %d poles: %s, held by the %s",
		a.ID, a.Applicant, len(a.Poles), a.State, a.HeldBy())
}
