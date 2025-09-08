// Package permit runs the application through its lifecycle.
//
// A joint use application is a regulated process with clocks on it. The owner
// has so many days to acknowledge, so many to survey, so many to return an
// estimate; the attacher has so many to accept it and so many to pay. Missing
// one of those dates has consequences, so the state machine and the clocks are
// the same object rather than two.
package permit

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ErrBadTransition is returned when a state change is not allowed.
var ErrBadTransition = errors.New("permit: transition not allowed")

// State is where an application has reached.
type State string

// The states an application passes through.
const (
	StateDraft       State = "draft"
	StateSubmitted   State = "submitted"
	StateAccepted    State = "accepted"
	StateSurveying   State = "surveying"
	StateEngineered  State = "engineered"
	StateEstimated   State = "estimated"
	StateAuthorised  State = "authorised"
	StateScheduled   State = "scheduled"
	StateConstructed State = "constructed"
	StateInspected   State = "inspected"
	StateClosed      State = "closed"
	StateRejected    State = "rejected"
	StateWithdrawn   State = "withdrawn"
)

var transitions = map[State][]State{
	StateDraft:       {StateSubmitted, StateWithdrawn},
	StateSubmitted:   {StateAccepted, StateRejected, StateWithdrawn},
	StateAccepted:    {StateSurveying, StateWithdrawn},
	StateSurveying:   {StateEngineered, StateRejected, StateWithdrawn},
	StateEngineered:  {StateEstimated, StateRejected, StateWithdrawn},
	StateEstimated:   {StateAuthorised, StateWithdrawn},
	StateAuthorised:  {StateScheduled, StateWithdrawn},
	StateScheduled:   {StateConstructed, StateWithdrawn},
	StateConstructed: {StateInspected},
	StateInspected:   {StateClosed, StateScheduled},
	StateClosed:      {},
	StateRejected:    {},
	StateWithdrawn:   {},
}

// KnownState reports whether the state is one the machine has.
func KnownState(s State) bool {
	_, ok := transitions[s]
	return ok
}

// States returns every state, sorted.
func States() []State {
	out := make([]State, 0, len(transitions))
	for s := range transitions {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Terminal reports whether nothing follows the state.
func (s State) Terminal() bool {
	next, ok := transitions[s]
	return ok && len(next) == 0
}

// Live reports whether the application is still being worked.
func (s State) Live() bool { return KnownState(s) && !s.Terminal() }

// Next returns the states that may follow, sorted.
func (s State) Next() []State {
	next, ok := transitions[s]
	if !ok {
		return nil
	}
	out := make([]State, len(next))
	copy(out, next)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// CanMoveTo reports whether the transition is allowed.
func (s State) CanMoveTo(to State) bool {
	for _, candidate := range transitions[s] {
		if candidate == to {
			return true
		}
	}
	return false
}

// ParseState reads a state name.
func ParseState(text string) (State, error) {
	t := State(strings.ToLower(strings.TrimSpace(text)))
	if KnownState(t) {
		return t, nil
	}
	return "", fmt.Errorf("%w: unknown state %q", ErrBadTransition, text)
}

// Owner says which side of the agreement is holding the application at a
// state. It is the field the delay reports are built from, because "the
// application has been sitting for forty days" only means something once
// somebody can say whose forty days they were.
type Holder string

// The two sides, and the one state nobody is holding.
const (
	HolderUtility  Holder = "utility"
	HolderAttacher Holder = "attacher"
	HolderNobody   Holder = "nobody"
)

var holders = map[State]Holder{
	StateDraft:       HolderAttacher,
	StateSubmitted:   HolderUtility,
	StateAccepted:    HolderUtility,
	StateSurveying:   HolderUtility,
	StateEngineered:  HolderUtility,
	StateEstimated:   HolderAttacher,
	StateAuthorised:  HolderUtility,
	StateScheduled:   HolderUtility,
	StateConstructed: HolderUtility,
	StateInspected:   HolderUtility,
	StateClosed:      HolderNobody,
	StateRejected:    HolderNobody,
	StateWithdrawn:   HolderNobody,
}

// HeldBy returns who is holding the application at a state.
func (s State) HeldBy() Holder {
	if h, ok := holders[s]; ok {
		return h
	}
	return HolderNobody
}

// Path returns the shortest sequence of states from one to another, and
// reports whether there is one. It is used to check that a state a caller
// wants to jump to is reachable at all before anything is written down.
func Path(from, to State) ([]State, bool) {
	if !KnownState(from) || !KnownState(to) {
		return nil, false
	}
	if from == to {
		return []State{from}, true
	}
	previous := map[State]State{}
	seen := map[State]bool{from: true}
	queue := []State{from}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, next := range current.Next() {
			if seen[next] {
				continue
			}
			seen[next] = true
			previous[next] = current
			if next == to {
				return buildPath(previous, from, to), true
			}
			queue = append(queue, next)
		}
	}
	return nil, false
}

func buildPath(previous map[State]State, from, to State) []State {
	var reversed []State
	for at := to; at != from; at = previous[at] {
		reversed = append(reversed, at)
	}
	reversed = append(reversed, from)
	out := make([]State, len(reversed))
	for i, s := range reversed {
		out[len(reversed)-1-i] = s
	}
	return out
}
