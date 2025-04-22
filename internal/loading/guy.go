package loading

import (
	"fmt"
	"math"
	"sort"

	"github.com/glinskevxer/downguy/internal/nesc"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/unit"
)

// Pull is an unbalanced horizontal force on a pole, with the direction it acts
// in and the height it acts at. A dead end, a line angle and a wind all
// produce one, and the guy has to answer the resultant of the lot rather than
// any single one of them.
type Pull struct {
	Source  string
	Force   float64
	Bearing int
	Height  unit.Height
}

// Resultant returns the vector sum of a set of pulls: the total force in
// pounds and the bearing it acts along in whole degrees. Two equal pulls a
// hundred and eighty degrees apart cancel, which is exactly the case a
// straight line through a pole is in and the reason most poles need no guy at
// all.
func Resultant(pulls []Pull) (float64, int) {
	north, east := 0.0, 0.0
	for _, p := range pulls {
		rad := float64(p.Bearing) * math.Pi / 180
		north += p.Force * math.Cos(rad)
		east += p.Force * math.Sin(rad)
	}
	force := math.Hypot(north, east)
	if force < 1e-9 {
		return 0, 0
	}
	deg := math.Atan2(east, north) * 180 / math.Pi
	for deg < 0 {
		deg += 360
	}
	return force, int(math.Round(deg)) % 360
}

// MomentOf returns the groundline moment of a set of pulls, in foot pounds,
// taking each at its own height. It is not the resultant force times an
// average height: a small pull high up can outweigh a large one near the
// ground.
func MomentOf(pulls []Pull) float64 {
	total := 0.0
	for _, p := range pulls {
		total += p.Force * p.Height.Float()
	}
	return total
}

// GuyCheck is the finding on whether a pole's guying answers what is pulling
// on it.
type GuyCheck struct {
	Pole        plant.PoleID
	PullForce   float64
	PullBearing int
	PullMoment  float64
	GuyMoment   float64
	Anchors     []plant.Anchor
	Adequate    bool
	Note        string
}

// EffectiveBearing returns the bearing a guy resists along, which is the
// opposite of the direction it is anchored in.
func EffectiveBearing(anchorBearing int) int { return (anchorBearing + 180) % 360 }

// CheckGuying works out whether the anchors on a pole answer the resultant
// pull. Only the component of each anchor's holding capacity that lies along
// the pull direction counts, so an anchor set ninety degrees off does nothing
// at all for that pull.
func CheckGuying(p plant.Pole, pulls []Pull, anchors []plant.Anchor) (*GuyCheck, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	force, bearing := Resultant(pulls)
	check := &GuyCheck{
		Pole: p.ID, PullForce: force, PullBearing: bearing,
		PullMoment: MomentOf(pulls), Anchors: anchors,
	}
	if force < 1 {
		check.Adequate = true
		check.Note = "nothing unbalanced to guy against"
		return check, nil
	}
	for _, a := range anchors {
		if a.Pole != p.ID {
			return nil, fmt.Errorf("%w: anchor %s is filed on %s, not %s", ErrBadLoad, a.ID, a.Pole, p.ID)
		}
		capacity, err := a.HoldingCapacity()
		if err != nil {
			return nil, err
		}
		resist := EffectiveBearing(a.Bearing)
		// A guy set square to the pull contributes nothing. Cosine of ninety
		// degrees comes back as a fraction of a nanopound rather than zero, so
		// the cut is a tolerance and not a sign test.
		off := math.Cos(float64(resist-bearing) * math.Pi / 180)
		if off <= 1e-9 {
			continue
		}
		highest := unit.Height(0)
		for _, g := range a.Guys {
			if g.Height > highest {
				highest = g.Height
			}
		}
		check.GuyMoment += capacity * off * highest.Float()
	}
	check.Adequate = check.GuyMoment >= check.PullMoment
	if check.Adequate {
		check.Note = "guyed"
	} else if len(anchors) == 0 {
		check.Note = "unguyed and carrying an unbalanced pull"
	} else {
		check.Note = "guyed, but not enough for the pull"
	}
	return check, nil
}

// Deficit returns the guying moment still needed, in foot pounds. A guyed pole
// with margin returns zero rather than a negative number.
func (g GuyCheck) Deficit() float64 {
	if g.GuyMoment >= g.PullMoment {
		return 0
	}
	return g.PullMoment - g.GuyMoment
}

// Rule returns the catalogue rule an inadequate guying finding cites.
func (g GuyCheck) Rule() string { return "NESC-261A" }

// DeadEndPulls builds the unbalanced pulls at a dead end: the full tension of
// every wire, acting along the line, at each wire's own height.
func DeadEndPulls(wires []Wire, bearing int) []Pull {
	out := make([]Pull, 0, len(wires))
	for _, w := range wires {
		if w.Tension <= 0 {
			continue
		}
		out = append(out, Pull{
			Source: w.AttachmentID, Force: w.Tension,
			Bearing: bearing, Height: w.Height,
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Height > out[j].Height })
	return out
}

// AnglePulls builds the unbalanced pulls at a line angle, acting along the
// bisector of the deflection.
func AnglePulls(wires []Wire, bisectorBearing int) ([]Pull, error) {
	out := make([]Pull, 0, len(wires))
	for _, w := range wires {
		if w.AngleDegrees <= 0 || w.Tension <= 0 {
			continue
		}
		force, err := AngleTension(w.Tension, w.AngleDegrees)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", w.AttachmentID, err)
		}
		out = append(out, Pull{
			Source: w.AttachmentID, Force: force,
			Bearing: bisectorBearing, Height: w.Height,
		})
	}
	return out, nil
}

// WindPulls builds the transverse wind pull from a set of wires, acting across
// the line.
func WindPulls(wires []Wire, l nesc.Loading, bearing int) ([]Pull, error) {
	out := make([]Pull, 0, len(wires))
	for _, w := range wires {
		force, err := WireWind(w.Conductor, l, w.WindSpanFeet)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", w.AttachmentID, err)
		}
		out = append(out, Pull{
			Source: w.AttachmentID, Force: force,
			Bearing: bearing, Height: w.Height,
		})
	}
	return out, nil
}
