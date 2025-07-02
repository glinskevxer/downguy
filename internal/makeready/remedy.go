// Package makeready turns clearance and loading findings into the work that
// has to happen before an attachment can go up.
//
// A finding says what is wrong. A remedy says what somebody has to do about
// it, who has to do it, and in what order. The order matters more than it
// looks: transferring a cable onto a pole that is about to be replaced is
// work done twice, and a planner that emits remedies in finding order will do
// exactly that.
package makeready

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/glinskevxer/downguy/internal/unit"
)

// ErrBadRemedy covers every rejected work item.
var ErrBadRemedy = errors.New("makeready: bad remedy")

// Action is what physically happens.
type Action string

// The actions this utility's make ready crews perform.
const (
	// ActionRaise lifts a facility on the same pole.
	ActionRaise Action = "raise"
	// ActionLower drops a facility on the same pole.
	ActionLower Action = "lower"
	// ActionTransfer moves a facility from one pole to its replacement.
	ActionTransfer Action = "transfer"
	// ActionReplacePole sets a new pole and moves everything to it.
	ActionReplacePole Action = "replace_pole"
	// ActionAddGuy sets an anchor and strings a guy.
	ActionAddGuy Action = "add_guy"
	// ActionRelocateEquipment moves a box off the climbing face.
	ActionRelocateEquipment Action = "relocate_equipment"
	// ActionRemove takes an unauthorised or abandoned facility down.
	ActionRemove Action = "remove"
	// ActionResag pulls a slack span back up to its design sag.
	ActionResag Action = "resag"
)

// stageOf is the order the work has to happen in. Everything that changes the
// pole itself comes before anything that hangs off it, because the second lot
// has to be done again otherwise.
var stageOf = map[Action]int{
	ActionRemove:            0,
	ActionReplacePole:       1,
	ActionAddGuy:            2,
	ActionTransfer:          3,
	ActionRelocateEquipment: 4,
	ActionRaise:             5,
	ActionLower:             5,
	ActionResag:             6,
}

// KnownAction reports whether the action is one the planner emits.
func KnownAction(a Action) bool {
	_, ok := stageOf[a]
	return ok
}

// Stage returns the order the action happens in, and reports whether the
// action is known.
func Stage(a Action) (int, bool) {
	s, ok := stageOf[a]
	return s, ok
}

// Actions returns every action in stage order, then alphabetically inside a
// stage so the list is stable.
func Actions() []Action {
	out := make([]Action, 0, len(stageOf))
	for a := range stageOf {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool {
		si, sj := stageOf[out[i]], stageOf[out[j]]
		if si != sj {
			return si < sj
		}
		return out[i] < out[j]
	})
	return out
}

// ParseAction reads an action written with a space, a dash or an underscore.
func ParseAction(text string) (Action, error) {
	t := strings.ToLower(strings.TrimSpace(text))
	t = strings.ReplaceAll(t, " ", "_")
	t = strings.ReplaceAll(t, "-", "_")
	if KnownAction(Action(t)) {
		return Action(t), nil
	}
	return "", fmt.Errorf("%w: unknown action %q", ErrBadRemedy, text)
}

// PoleWork reports whether the action changes the structure rather than what
// hangs on it. Pole work needs an outage window and a crew with a digger;
// everything else is a two person job off a ladder.
func (a Action) PoleWork() bool {
	switch a {
	case ActionReplacePole, ActionAddGuy:
		return true
	}
	return false
}

// Item is one piece of make ready work.
type Item struct {
	ID         string
	Action     Action
	Pole       string
	Span       string
	Subject    string
	Owner      string
	FromHeight unit.Height
	ToHeight   unit.Height
	Because    []string
	Detail     string
}

// Validate checks a work item.
func (i Item) Validate() error {
	if strings.TrimSpace(i.ID) == "" {
		return fmt.Errorf("%w: no id", ErrBadRemedy)
	}
	if !KnownAction(i.Action) {
		return fmt.Errorf("%w: %s is a %q", ErrBadRemedy, i.ID, i.Action)
	}
	if strings.TrimSpace(i.Pole) == "" {
		return fmt.Errorf("%w: %s names no pole", ErrBadRemedy, i.ID)
	}
	if len(i.Because) == 0 {
		return fmt.Errorf("%w: %s cites no finding", ErrBadRemedy, i.ID)
	}
	switch i.Action {
	case ActionRaise:
		if i.ToHeight <= i.FromHeight {
			return fmt.Errorf("%w: %s raises %s from %s to %s", ErrBadRemedy, i.ID, i.Subject, i.FromHeight, i.ToHeight)
		}
	case ActionLower:
		if i.ToHeight >= i.FromHeight {
			return fmt.Errorf("%w: %s lowers %s from %s to %s", ErrBadRemedy, i.ID, i.Subject, i.FromHeight, i.ToHeight)
		}
	}
	if i.Action != ActionReplacePole && i.Action != ActionAddGuy && strings.TrimSpace(i.Subject) == "" {
		return fmt.Errorf("%w: %s names nothing to work on", ErrBadRemedy, i.ID)
	}
	return nil
}

// Move returns how far the facility travels, which is zero for work that does
// not move anything.
func (i Item) Move() unit.Height { return i.ToHeight.Sub(i.FromHeight).Abs() }

// String prints the work item the way a crew sheet lists it.
func (i Item) String() string {
	switch i.Action {
	case ActionRaise, ActionLower:
		return fmt.Sprintf("%s %s %s on %s from %s to %s", i.ID, i.Action, i.Subject, i.Pole, i.FromHeight, i.ToHeight)
	case ActionReplacePole:
		return fmt.Sprintf("%s replace %s: %s", i.ID, i.Pole, i.Detail)
	default:
		return fmt.Sprintf("%s %s %s on %s", i.ID, i.Action, i.Subject, i.Pole)
	}
}
