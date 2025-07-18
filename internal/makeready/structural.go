package makeready

import (
	"fmt"

	"github.com/glinskevxer/downguy/internal/clearance"
	"github.com/glinskevxer/downguy/internal/loading"
	"github.com/glinskevxer/downguy/internal/nesc"
	"github.com/glinskevxer/downguy/internal/plant"
)

// Structural is the decision about the pole itself: does it carry the load
// with the new attachment on, and if not, is the answer a guy or a new pole.
type Structural struct {
	Pole        string
	Before      float64
	After       float64
	Passes      bool
	Guyable     bool
	Replacement loading.Replacement
	Items       []Item
	Findings    clearance.Findings
}

// AssessStructure runs the groundline check with and without the applicant's
// facility and works out what has to happen. The order of the questions is the
// point: a guy is cheaper than a pole, so it is asked about first, and a pole
// is only recommended when the guy will not do.
func (p *Planner) AssessStructure(pole plant.Pole, before, after []loading.Wire, pulls []loading.Pull, d nesc.District, g nesc.Grade) (*Structural, error) {
	beforeAnalysis, err := loading.Analyse(pole, before, d, g)
	if err != nil {
		return nil, err
	}
	afterAnalysis, err := loading.Analyse(pole, after, d, g)
	if err != nil {
		return nil, err
	}
	s := &Structural{
		Pole:   string(pole.ID),
		Before: beforeAnalysis.Utilisation,
		After:  afterAnalysis.Utilisation,
		Passes: afterAnalysis.Passes(),
	}
	if s.Passes {
		return s, nil
	}

	rule := nesc.MustRule("NESC-250B")
	s.Findings = clearance.Findings{{
		RuleID: rule.ID, Severity: rule.Severity,
		Pole: s.Pole, Subject: s.Pole, Blame: clearance.BlameUnattributed,
		Detail: fmt.Sprintf("groundline utilisation goes from %d%% to %d%% under grade %s in the %s district",
			beforeAnalysis.UtilisationPercent(), afterAnalysis.UtilisationPercent(), g, d),
	}}

	guyable, err := loading.NeedsGuying(pole, after, pulls, d, g)
	if err != nil {
		return nil, err
	}
	s.Guyable = guyable
	if guyable {
		s.Items = append(s.Items, Item{
			ID: p.id(), Action: ActionAddGuy, Pole: s.Pole,
			Owner: pole.Owner, Because: []string{s.Findings[0].Key()},
			Detail: "guy the unbalanced pull off the structure",
		})
		return s, nil
	}

	replacement, err := loading.FindReplacement(pole, after, d, g)
	if err != nil {
		return nil, err
	}
	s.Replacement = replacement
	if !replacement.Found {
		return s, nil
	}
	s.Items = append(s.Items, Item{
		ID: p.id(), Action: ActionReplacePole, Pole: s.Pole,
		Owner: pole.Owner, Because: []string{s.Findings[0].Key()},
		Detail: replacement.String(),
	})
	for _, w := range after {
		s.Items = append(s.Items, Item{
			ID: p.id(), Action: ActionTransfer, Pole: s.Pole,
			Subject: w.AttachmentID, Owner: w.Owner,
			FromHeight: w.Height, ToHeight: w.Height,
			Because: []string{s.Findings[0].Key()},
			Detail:  fmt.Sprintf("transfer to the new %s", replacement),
		})
	}
	return s, nil
}

// Worsened returns how much of the utilisation the applicant added.
func (s Structural) Worsened() float64 { return s.After - s.Before }

// AlreadyOver reports whether the pole was overloaded before the applicant
// arrived. It is the structural version of a pre existing violation, and it is
// the difference between the applicant paying for a pole and the owner paying
// for one.
func (s Structural) AlreadyOver() bool { return s.Before > 1 }

// Summary writes the sentence the structural section of a report leads with.
func (s Structural) Summary() string {
	if s.Passes {
		return fmt.Sprintf("%s carries the new attachment at %d%%", s.Pole, percent(s.After))
	}
	switch {
	case s.Guyable:
		return fmt.Sprintf("%s goes to %d%% and needs guying", s.Pole, percent(s.After))
	case s.Replacement.Found:
		return fmt.Sprintf("%s goes to %d%% and needs replacing with a %s", s.Pole, percent(s.After), s.Replacement)
	default:
		return fmt.Sprintf("%s goes to %d%% and nothing in stock carries it", s.Pole, percent(s.After))
	}
}

func percent(u float64) int {
	pct := u * 100
	whole := int(pct)
	if pct > float64(whole) {
		whole++
	}
	return whole
}
