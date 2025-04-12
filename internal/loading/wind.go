// Package loading works out whether a pole can carry what is on it.
//
// The question the make ready planner asks is always the same: with the new
// attachment on, does the factored moment at the groundline stay under the
// factored capacity of the pole. Everything here builds one side or the other
// of that comparison.
package loading

import (
	"errors"
	"fmt"
	"math"

	"github.com/glinskevxer/downguy/internal/nesc"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/sagten"
	"github.com/glinskevxer/downguy/internal/unit"
)

// ErrBadLoad is returned when a load cannot be worked out from what was given.
var ErrBadLoad = errors.New("loading: cannot compute load")

// WindSpan returns the length of wire whose wind load a pole carries, in feet:
// half of each adjacent span. A pole between a short span and a long one
// carries the average of the two, not the longer.
func WindSpan(adjacentFeet ...int) float64 {
	total := 0.0
	for _, s := range adjacentFeet {
		if s <= 0 {
			continue
		}
		total += float64(s) / 2
	}
	return total
}

// WireWind returns the transverse wind force on one wire at a pole, in pounds.
func WireWind(c sagten.Conductor, l nesc.Loading, windSpanFeet float64) (float64, error) {
	if err := c.Validate(); err != nil {
		return 0, err
	}
	if windSpanFeet < 0 {
		return 0, fmt.Errorf("%w: wind span is %v feet", ErrBadLoad, windSpanFeet)
	}
	return l.WindLoadPerFoot(c.DiameterInches) * windSpanFeet, nil
}

// WireMoment returns the moment a wire's wind load puts on the groundline, in
// foot pounds. The lever arm is the height of the attachment above the
// groundline.
func WireMoment(c sagten.Conductor, l nesc.Loading, windSpanFeet float64, height unit.Height) (float64, error) {
	force, err := WireWind(c, l, windSpanFeet)
	if err != nil {
		return 0, err
	}
	if height <= 0 {
		return 0, fmt.Errorf("%w: attachment is at %s", ErrBadLoad, height)
	}
	return force * height.Float(), nil
}

// PoleWind returns the wind force on the pole itself and the height its
// resultant acts at, both worked out from the taper rather than from an
// average width, because a forty foot pole is a third wider at the bottom than
// at the top and the arm matters as much as the force.
func PoleWind(p plant.Pole, l nesc.Loading) (float64, unit.Height, error) {
	above := p.AboveGrade().Float()
	if above <= 0 {
		return 0, 0, fmt.Errorf("%w: %s stands %s above grade", ErrBadLoad, p.ID, p.AboveGrade())
	}
	buttCirc, err := plant.CircumferenceAt(p, 0)
	if err != nil {
		return 0, 0, err
	}
	topCirc, err := plant.CircumferenceAt(p, above)
	if err != nil {
		return 0, 0, err
	}
	buttWidth := buttCirc / math.Pi / 12
	topWidth := topCirc / math.Pi / 12
	area := (buttWidth + topWidth) / 2 * above
	force := area * l.WindPSF()
	// Centroid of a trapezoid measured from the wide end.
	arm := above / 3 * (buttWidth + 2*topWidth) / (buttWidth + topWidth)
	return force, unit.Tenths(int(math.Round(arm * 10))), nil
}

// PoleMoment returns the moment the wind on the pole itself puts on the
// groundline, in foot pounds.
func PoleMoment(p plant.Pole, l nesc.Loading) (float64, error) {
	force, arm, err := PoleWind(p, l)
	if err != nil {
		return 0, err
	}
	return force * arm.Float(), nil
}

// AngleTension returns the transverse force a line angle puts on a pole, in
// pounds. A wire pulled round a corner pulls sideways with twice the tension
// times the sine of half the deflection.
func AngleTension(tensionPounds float64, deflectionDegrees float64) (float64, error) {
	if tensionPounds < 0 {
		return 0, fmt.Errorf("%w: tension is %v pounds", ErrBadLoad, tensionPounds)
	}
	if deflectionDegrees < 0 || deflectionDegrees > 180 {
		return 0, fmt.Errorf("%w: deflection is %v degrees", ErrBadLoad, deflectionDegrees)
	}
	half := deflectionDegrees / 2 * math.Pi / 180
	return 2 * tensionPounds * math.Sin(half), nil
}

// AngleMoment returns the groundline moment from a line angle at one
// attachment height.
func AngleMoment(tensionPounds, deflectionDegrees float64, height unit.Height) (float64, error) {
	force, err := AngleTension(tensionPounds, deflectionDegrees)
	if err != nil {
		return 0, err
	}
	if height <= 0 {
		return 0, fmt.Errorf("%w: attachment is at %s", ErrBadLoad, height)
	}
	return force * height.Float(), nil
}
