package loading

import (
	"fmt"
	"sort"

	"github.com/glinskevxer/downguy/internal/nesc"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/sagten"
	"github.com/glinskevxer/downguy/internal/unit"
)

// conditionFactor scales a pole's remaining strength by what the last
// groundline inspection found. A pole that has been in the ground since 1974
// does not carry what the tables say a new one of its class carries, and
// analysing it as though it does is the commonest way a joint use application
// gets approved onto a pole that then fails.
var conditionFactor = map[plant.Condition]float64{
	plant.ConditionNew:        1.00,
	plant.ConditionSound:      1.00,
	plant.ConditionServiceabe: 0.85,
	plant.ConditionRestorable: 0.70,
	plant.ConditionReject:     0.00,
	plant.ConditionUnknown:    0.75,
}

// ConditionFactor returns the strength multiplier for a pole condition and
// reports whether the condition is one the table covers.
func ConditionFactor(c plant.Condition) (float64, bool) {
	f, ok := conditionFactor[c]
	return f, ok
}

// Wire is one span carrying facility as the loading calculation sees it: what
// it is, how much wire the pole carries the wind on, and how high up it is.
type Wire struct {
	AttachmentID string
	Owner        string
	Conductor    sagten.Conductor
	// WindSpanFeet is the length of wire whose wind this pole carries.
	WindSpanFeet float64
	// SpanFeet is the span the sag is worked out over, and RulingSpanFeet is
	// the section's ruling span, which fixes the tension. They are different
	// numbers and using one for the other is the classic way to get a sag that
	// looks plausible and is wrong.
	SpanFeet       float64
	RulingSpanFeet float64
	Height         unit.Height
	Tension        float64
	AngleDegrees   float64
}

// Sag returns the sag of the wire at a temperature under a load, in feet,
// having solved the change of state from its design condition.
func (w Wire) Sag(d nesc.District, loadPerFoot float64, temp unit.Temperature) (float64, error) {
	if w.RulingSpanFeet <= 0 || w.SpanFeet <= 0 {
		return 0, fmt.Errorf("%w: %s has no span to sag over", ErrBadLoad, w.AttachmentID)
	}
	start, err := sagten.InitialCondition(w.Conductor, w.RulingSpanFeet, d)
	if err != nil {
		return 0, err
	}
	sag, _, err := sagten.SagAt(w.Conductor, w.RulingSpanFeet, w.SpanFeet, start, loadPerFoot, temp)
	return sag, err
}

// MaxSag returns the deepest sag the wire ever reaches: at its maximum
// operating temperature under its own bare weight, or under the district
// loading case, whichever hangs lower.
func (w Wire) MaxSag(d nesc.District) (float64, error) {
	hot, err := w.Sag(d, sagten.BareLoad(w.Conductor), unit.TempMaxOperating)
	if err != nil {
		return 0, err
	}
	load, temp, err := sagten.DistrictCondition(w.Conductor, d)
	if err != nil {
		return 0, err
	}
	iced, err := w.Sag(d, load, temp)
	if err != nil {
		return 0, err
	}
	if iced > hot {
		return iced, nil
	}
	return hot, nil
}

// Component is one contribution to the groundline moment, kept separately so a
// report can say which attachment is responsible for what.
type Component struct {
	Source   string
	Kind     string
	Moment   float64
	Factored float64
}

// Analysis is the finished loading check on one pole.
type Analysis struct {
	Pole            plant.Pole
	District        nesc.District
	Grade           nesc.Grade
	Components      []Component
	FactoredMoment  float64
	Capacity        float64
	FactoredCapacty float64
	Utilisation     float64
}

// Passes reports whether the factored moment stays under the factored
// capacity.
func (a Analysis) Passes() bool {
	return a.FactoredCapacty > 0 && a.FactoredMoment <= a.FactoredCapacty
}

// Headroom returns the moment still available, in foot pounds. A negative
// figure is the overload.
func (a Analysis) Headroom() float64 { return a.FactoredCapacty - a.FactoredMoment }

// Worst returns the component contributing the most factored moment.
func (a Analysis) Worst() (Component, bool) {
	if len(a.Components) == 0 {
		return Component{}, false
	}
	worst := a.Components[0]
	for _, c := range a.Components[1:] {
		if c.Factored > worst.Factored {
			worst = c
		}
	}
	return worst, true
}

// Analyse works out the groundline check for a pole carrying a set of wires,
// under a district loading case and a grade of construction.
func Analyse(p plant.Pole, wires []Wire, d nesc.District, g nesc.Grade) (*Analysis, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	l, err := nesc.LoadingFor(d)
	if err != nil {
		return nil, err
	}
	f, err := nesc.FactorsFor(g)
	if err != nil {
		return nil, err
	}
	cf, ok := ConditionFactor(p.Condition)
	if !ok {
		return nil, fmt.Errorf("%w: %s is in condition %q", ErrBadLoad, p.ID, p.Condition)
	}
	capacity, err := plant.GroundlineCapacity(p)
	if err != nil {
		return nil, err
	}

	a := &Analysis{Pole: p, District: d, Grade: g, Capacity: capacity}

	poleMoment, err := PoleMoment(p, l)
	if err != nil {
		return nil, err
	}
	a.Components = append(a.Components, Component{
		Source: string(p.ID), Kind: "pole wind",
		Moment: poleMoment, Factored: poleMoment * f.TransverseWind,
	})

	for _, w := range wires {
		m, err := WireMoment(w.Conductor, l, w.WindSpanFeet, w.Height)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", w.AttachmentID, err)
		}
		a.Components = append(a.Components, Component{
			Source: w.AttachmentID, Kind: "wire wind",
			Moment: m, Factored: m * f.TransverseWind,
		})
		if w.AngleDegrees > 0 {
			am, err := AngleMoment(w.Tension, w.AngleDegrees, w.Height)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", w.AttachmentID, err)
			}
			a.Components = append(a.Components, Component{
				Source: w.AttachmentID, Kind: "line angle",
				Moment: am, Factored: am * f.TransverseWire,
			})
		}
	}

	for _, c := range a.Components {
		a.FactoredMoment += c.Factored
	}
	a.FactoredCapacty = capacity * f.WoodStrength * cf
	if a.FactoredCapacty > 0 {
		a.Utilisation = a.FactoredMoment / a.FactoredCapacty
	}
	sort.SliceStable(a.Components, func(i, j int) bool {
		return a.Components[i].Factored > a.Components[j].Factored
	})
	return a, nil
}

// UtilisationPercent returns the utilisation as a whole percent, rounded up,
// which is the form a work order carries.
func (a Analysis) UtilisationPercent() int {
	pct := a.Utilisation * 100
	whole := int(pct)
	if pct > float64(whole) {
		whole++
	}
	return whole
}

// String prints the one line summary a report leads with.
func (a Analysis) String() string {
	verdict := "over"
	if a.Passes() {
		verdict = "ok"
	}
	return fmt.Sprintf("%s grade %s in the %s district: %d%% %s",
		a.Pole.ID, a.Grade, a.District, a.UtilisationPercent(), verdict)
}
