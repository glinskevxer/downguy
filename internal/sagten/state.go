package sagten

import (
	"errors"
	"fmt"
	"math"

	"github.com/glinskevxer/downguy/internal/nesc"
	"github.com/glinskevxer/downguy/internal/unit"
)

// ErrNoSolution is returned when the change of state equation does not settle
// on a tension.
var ErrNoSolution = errors.New("sagten: change of state did not converge")

// Condition is one state of a wire: the load it carries per foot and the
// temperature it is at. Every sag question is really a question about moving
// from one condition to another.
type Condition struct {
	LoadPerFoot float64
	Temperature unit.Temperature
	Tension     float64
}

// ResultantLoad returns the load per foot on a conductor under a district
// loading case, in pounds. It is the resultant of the vertical load, which is
// the bare weight plus the ice, and the horizontal wind on the iced diameter,
// with the district constant added to the resultant afterwards.
func ResultantLoad(c Conductor, l nesc.Loading) (float64, error) {
	if err := c.Validate(); err != nil {
		return 0, err
	}
	vertical := c.WeightPerFoot + l.IceWeightPerFoot(c.DiameterInches)
	horizontal := l.WindLoadPerFoot(c.DiameterInches)
	return math.Hypot(vertical, horizontal) + l.KPoundsPerFoot(), nil
}

// BareLoad returns the load per foot with no ice and no wind, which is what
// the conductor carries on an ordinary day.
func BareLoad(c Conductor) float64 { return c.WeightPerFoot }

// SolveTension returns the horizontal tension a conductor settles at in the
// second condition, given where it started. The relation is the standard cubic
// in the new tension: the wire stretches elastically under load and thermally
// with temperature, and the length it ends up with has to match the length the
// new sag implies.
//
// The cubic has exactly one positive root for physical inputs, so it is solved
// by bracketing that root and bisecting, which cannot run away the way Newton
// can when the initial guess is poor.
func SolveTension(c Conductor, rulingSpanFeet float64, from Condition, toLoad float64, toTemp unit.Temperature) (float64, error) {
	if err := c.Validate(); err != nil {
		return 0, err
	}
	if rulingSpanFeet <= 0 {
		return 0, fmt.Errorf("%w: ruling span is %v feet", ErrBadSag, rulingSpanFeet)
	}
	if from.Tension <= 0 || from.LoadPerFoot <= 0 {
		return 0, fmt.Errorf("%w: the starting condition is not physical", ErrBadSag)
	}
	if toLoad <= 0 {
		return 0, fmt.Errorf("%w: the new load is %v per foot", ErrBadSag, toLoad)
	}

	ea := c.ModulusPSI * c.AreaSquareInches
	s2 := rulingSpanFeet * rulingSpanFeet
	dt := float64(toTemp.Delta(from.Temperature))

	// The cubic, written so that f(T) rises through zero exactly once.
	b := from.LoadPerFoot*from.LoadPerFoot*s2*ea/(24*from.Tension*from.Tension) -
		from.Tension + c.ThermalCoefficient*ea*dt
	rhs := toLoad * toLoad * s2 * ea / 24
	f := func(t float64) float64 { return t*t*t + b*t*t - rhs }

	lo := 1.0
	hi := math.Max(from.Tension, c.RatedStrength) * 4
	if f(lo) > 0 {
		return 0, fmt.Errorf("%w: the root is below one pound of tension", ErrNoSolution)
	}
	for i := 0; f(hi) < 0; i++ {
		hi *= 2
		if i > 40 {
			return 0, fmt.Errorf("%w: could not bracket the root", ErrNoSolution)
		}
	}
	for i := 0; i < 200; i++ {
		mid := (lo + hi) / 2
		if f(mid) < 0 {
			lo = mid
		} else {
			hi = mid
		}
		if hi-lo < 1e-9 {
			break
		}
	}
	root := (lo + hi) / 2
	if math.IsNaN(root) || root <= 0 {
		return 0, fmt.Errorf("%w: settled on %v pounds", ErrNoSolution, root)
	}
	return root, nil
}

// SagAt returns the sag of a conductor in the second condition, having solved
// for the tension it settles at.
func SagAt(c Conductor, rulingSpanFeet, spanFeet float64, from Condition, toLoad float64, toTemp unit.Temperature) (float64, float64, error) {
	tension, err := SolveTension(c, rulingSpanFeet, from, toLoad, toTemp)
	if err != nil {
		return 0, 0, err
	}
	sag, err := ParabolicSag(toLoad, spanFeet, tension)
	if err != nil {
		return 0, 0, err
	}
	return sag, tension, nil
}

// InitialCondition builds the starting state a conductor is strung at: the
// bare load at the stringing temperature, held at the everyday tension limit.
func InitialCondition(c Conductor) (Condition, error) {
	if err := c.Validate(); err != nil {
		return Condition{}, err
	}
	return Condition{
		LoadPerFoot: BareLoad(c),
		Temperature: unit.TempStringing,
		Tension:     c.MaxTension(false),
	}, nil
}

// DistrictCondition builds the state a conductor is in under a district
// loading case.
func DistrictCondition(c Conductor, d nesc.District) (float64, unit.Temperature, error) {
	l, err := nesc.LoadingFor(d)
	if err != nil {
		return 0, 0, err
	}
	load, err := ResultantLoad(c, l)
	if err != nil {
		return 0, 0, err
	}
	return load, l.Temperature, nil
}
