package makeready

import (
	"fmt"
	"sort"

	"github.com/glinskevxer/downguy/internal/clearance"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/unit"
)

// Plan is the ordered work for one application.
type Plan struct {
	Application string
	Items       []Item
	Unresolved  []clearance.Finding
}

// Planner turns findings into work. It holds the surveyed assemblies so that a
// remedy can be worked out against real heights rather than against the
// deficit alone: raising a cable by the deficit is only correct if there is
// room above it, and the planner is the thing that knows whether there is.
type Planner struct {
	assemblies  map[string]*plant.Assembly
	nextID      int
	application string
}

// NewPlanner starts a plan for one application.
func NewPlanner(application string) *Planner {
	return &Planner{assemblies: map[string]*plant.Assembly{}, application: application}
}

// Know registers a surveyed assembly so the planner can reason about the pole.
func (p *Planner) Know(a *plant.Assembly) {
	if a == nil {
		return
	}
	p.assemblies[string(a.Pole.ID)] = a
}

func (p *Planner) id() string {
	p.nextID++
	return fmt.Sprintf("MR-%04d", p.nextID)
}

// Plan works every finding into an item, and lists the ones it could not.
func (p *Planner) Plan(findings clearance.Findings) *Plan {
	out := &Plan{Application: p.application}
	for _, f := range findings {
		item, ok := p.remedyFor(f)
		if !ok {
			out.Unresolved = append(out.Unresolved, f)
			continue
		}
		out.Items = append(out.Items, item)
	}
	out.Items = Order(out.Items)
	return out
}

// remedyFor picks the work that answers one finding.
func (p *Planner) remedyFor(f clearance.Finding) (Item, bool) {
	switch f.RuleID {
	case "NESC-235C1", "NESC-235C2", "NESC-238A":
		return p.lowerOrRaise(f)
	case "NESC-232A", "NESC-232B", "NESC-234C", "NESC-235E":
		return p.raiseInSpan(f)
	case "NESC-236A":
		return Item{
			ID: p.id(), Action: ActionRelocateEquipment, Pole: f.Pole,
			Subject: f.Subject, Owner: f.Owner, Because: []string{f.Key()},
			Detail: "move off the climbing face",
		}, true
	case "NESC-239E":
		return Item{
			ID: p.id(), Action: ActionRelocateEquipment, Pole: f.Pole,
			Subject: f.Subject, Owner: f.Owner, Because: []string{f.Key()},
			Detail: "move the vertical run clear of the facility it passes",
		}, true
	case "NESC-261A":
		return Item{
			ID: p.id(), Action: ActionAddGuy, Pole: f.Pole,
			Owner: f.Owner, Because: []string{f.Key()},
			Detail: "set an anchor and guy against the unbalanced pull",
		}, true
	}
	return Item{}, false
}

// lowerOrRaise answers a separation finding on a pole. The facility below the
// gap goes down if there is room under it, and the one above goes up if there
// is not.
func (p *Planner) lowerOrRaise(f clearance.Finding) (Item, bool) {
	a, ok := p.assemblies[f.Pole]
	if !ok {
		return Item{}, false
	}
	subject, ok := a.Find(f.Subject)
	if !ok {
		return Item{}, false
	}
	deficit := f.Deficit()
	if deficit <= 0 {
		return Item{}, false
	}
	if room, ok := p.roomBelow(a, subject); ok && room >= deficit {
		return Item{
			ID: p.id(), Action: ActionLower, Pole: f.Pole,
			Subject: f.Subject, Owner: f.Owner,
			FromHeight: subject.Height, ToHeight: subject.Height.Sub(deficit),
			Because: []string{f.Key()},
			Detail:  fmt.Sprintf("open %s under %s", deficit, f.Against),
		}, true
	}
	against, ok := a.Find(f.Against)
	if !ok {
		return Item{}, false
	}
	return Item{
		ID: p.id(), Action: ActionRaise, Pole: f.Pole,
		Subject: f.Against, Owner: against.Owner,
		FromHeight: against.Height, ToHeight: against.Height.Add(deficit),
		Because: []string{f.Key()},
		Detail:  fmt.Sprintf("no room under %s, so lift %s instead", f.Subject, f.Against),
	}, true
}

// raiseInSpan answers a ground clearance or mid span finding, which is always
// answered by lifting the facility rather than by moving what is above it.
func (p *Planner) raiseInSpan(f clearance.Finding) (Item, bool) {
	deficit := f.Deficit()
	if deficit <= 0 {
		return Item{}, false
	}
	from := f.Measured
	if a, ok := p.assemblies[f.Pole]; ok {
		if att, ok := a.Find(f.Subject); ok {
			from = att.Height
		}
	}
	return Item{
		ID: p.id(), Action: ActionRaise, Pole: f.Pole, Span: f.Span,
		Subject: f.Subject, Owner: f.Owner,
		FromHeight: from, ToHeight: from.Add(deficit),
		Because: []string{f.Key()},
		Detail:  fmt.Sprintf("lift %s by %s", f.Subject, deficit),
	}, true
}

// MinimumSeparation is the gap the planner leaves between a facility it moves
// down and whatever is under it.
const MinimumSeparation = unit.Height(10)

// roomBelow returns how far a facility could drop before it runs into the next
// one down, and reports whether there is anything under it at all.
func (p *Planner) roomBelow(a *plant.Assembly, subject plant.Attachment) (unit.Height, bool) {
	below, ok := a.Highest(func(other plant.Attachment) bool {
		return other.ID != subject.ID && other.Height < subject.Height
	})
	if !ok {
		return 0, false
	}
	gap := subject.Height.Sub(below.Height)
	if gap <= MinimumSeparation {
		return 0, true
	}
	return gap.Sub(MinimumSeparation), true
}

// Order sorts work items into the sequence a crew does them in: by pole, then
// by stage, then by id so that the list is stable.
func Order(items []Item) []Item {
	out := make([]Item, len(items))
	copy(out, items)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Pole != out[j].Pole {
			return out[i].Pole < out[j].Pole
		}
		si, _ := Stage(out[i].Action)
		sj, _ := Stage(out[j].Action)
		if si != sj {
			return si < sj
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// Validate checks every item in the plan.
func (p *Plan) Validate() error {
	seen := map[string]bool{}
	for _, i := range p.Items {
		if err := i.Validate(); err != nil {
			return err
		}
		if seen[i.ID] {
			return fmt.Errorf("%w: %s appears twice", ErrBadRemedy, i.ID)
		}
		seen[i.ID] = true
	}
	return nil
}

// Poles returns every pole the plan touches, sorted.
func (p *Plan) Poles() []string {
	seen := map[string]bool{}
	var out []string
	for _, i := range p.Items {
		if i.Pole == "" || seen[i.Pole] {
			continue
		}
		seen[i.Pole] = true
		out = append(out, i.Pole)
	}
	sort.Strings(out)
	return out
}

// Owners returns every attacher with work to do, sorted.
func (p *Plan) Owners() []string {
	seen := map[string]bool{}
	var out []string
	for _, i := range p.Items {
		if i.Owner == "" || seen[i.Owner] {
			continue
		}
		seen[i.Owner] = true
		out = append(out, i.Owner)
	}
	sort.Strings(out)
	return out
}

// Empty reports whether there is no work at all.
func (p *Plan) Empty() bool { return len(p.Items) == 0 }
