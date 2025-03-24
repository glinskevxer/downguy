package sagten

import (
	"errors"
	"fmt"
	"math"

	"github.com/glinskevxer/downguy/internal/unit"
)

// ErrBadSag is returned when the sag arithmetic is handed something it cannot
// work with.
var ErrBadSag = errors.New("sagten: cannot compute sag")

// ParabolicSag returns the sag at the middle of a level span, in feet, from
// the load per foot, the span and the horizontal tension. It is the
// approximation the trade works in and it is accurate to well under an inch
// for distribution spans.
func ParabolicSag(loadPerFoot, spanFeet, tensionPounds float64) (float64, error) {
	if loadPerFoot <= 0 {
		return 0, fmt.Errorf("%w: load per foot is %v", ErrBadSag, loadPerFoot)
	}
	if spanFeet <= 0 {
		return 0, fmt.Errorf("%w: span is %v feet", ErrBadSag, spanFeet)
	}
	if tensionPounds <= 0 {
		return 0, fmt.Errorf("%w: tension is %v pounds", ErrBadSag, tensionPounds)
	}
	return loadPerFoot * spanFeet * spanFeet / (8 * tensionPounds), nil
}

// CatenarySag returns the exact sag of a level span, in feet. It is the same
// number to three decimal places as the parabola for anything a distribution
// pole carries, and it is here so the approximation can be checked rather than
// believed.
func CatenarySag(loadPerFoot, spanFeet, tensionPounds float64) (float64, error) {
	if loadPerFoot <= 0 || spanFeet <= 0 || tensionPounds <= 0 {
		return 0, fmt.Errorf("%w: catenary needs positive load, span and tension", ErrBadSag)
	}
	c := tensionPounds / loadPerFoot
	return c * (math.Cosh(spanFeet/(2*c)) - 1), nil
}

// TensionForSag returns the horizontal tension that produces a wanted sag on a
// level span. It is the parabola read backwards, and it is how a stringing
// chart turns a sag target into a dynamometer reading.
func TensionForSag(loadPerFoot, spanFeet, sagFeet float64) (float64, error) {
	if loadPerFoot <= 0 || spanFeet <= 0 {
		return 0, fmt.Errorf("%w: tension needs a positive load and span", ErrBadSag)
	}
	if sagFeet <= 0 {
		return 0, fmt.Errorf("%w: sag is %v feet", ErrBadSag, sagFeet)
	}
	return loadPerFoot * spanFeet * spanFeet / (8 * sagFeet), nil
}

// ArcLength returns the length of wire in a level span, in feet. The wire is
// always longer than the span, and the difference is what the change of state
// equation is about.
func ArcLength(spanFeet, sagFeet float64) float64 {
	if spanFeet <= 0 {
		return 0
	}
	if sagFeet <= 0 {
		return spanFeet
	}
	return spanFeet + 8*sagFeet*sagFeet/(3*spanFeet)
}

// Profile describes where a wire sits across one span. It is the object the
// clearance rules read, and it is deliberately in feet above the near pole's
// groundline so that a drop between the two poles is carried explicitly.
type Profile struct {
	SpanFeet     float64
	SagFeet      float64
	NearHeight   unit.Height
	FarHeight    unit.Height
	GroundDrop   unit.Height
	LowPointFeet float64
}

// NewProfile builds a profile for a span. NearHeight and FarHeight are the
// attachment heights above their own poles' groundlines, and groundDrop is how
// much lower the ground at the far pole is than at the near pole.
func NewProfile(spanFeet, sagFeet float64, near, far, groundDrop unit.Height) (Profile, error) {
	if spanFeet <= 0 {
		return Profile{}, fmt.Errorf("%w: span is %v feet", ErrBadSag, spanFeet)
	}
	if sagFeet < 0 {
		return Profile{}, fmt.Errorf("%w: sag is %v feet", ErrBadSag, sagFeet)
	}
	p := Profile{
		SpanFeet: spanFeet, SagFeet: sagFeet,
		NearHeight: near, FarHeight: far, GroundDrop: groundDrop,
	}
	p.LowPointFeet = p.lowPoint()
	return p, nil
}

// lowPoint returns the distance from the near pole to the lowest point of the
// wire, in feet. On a level span it is the middle. On an inclined span the low
// point shifts toward the lower end, and past a certain incline it leaves the
// span entirely, at which point the lower attachment is itself the lowest
// point.
func (p Profile) lowPoint() float64 {
	rise := p.riseFeet()
	if p.SagFeet <= 0 {
		if rise >= 0 {
			return 0
		}
		return p.SpanFeet
	}
	offset := rise * p.SpanFeet / (8 * p.SagFeet)
	x := p.SpanFeet/2 - offset
	return math.Max(0, math.Min(p.SpanFeet, x))
}

// riseFeet returns how much higher the far attachment is than the near one,
// measured from a common datum, which is the near pole's groundline.
func (p Profile) riseFeet() float64 {
	return p.FarHeight.Float() - p.GroundDrop.Float() - p.NearHeight.Float()
}

// HeightAt returns the height of the wire above the near pole's groundline at
// a distance along the span, in feet.
func (p Profile) HeightAt(x float64) float64 {
	if x < 0 {
		x = 0
	}
	if x > p.SpanFeet {
		x = p.SpanFeet
	}
	chord := p.NearHeight.Float() + p.riseFeet()*x/p.SpanFeet
	if p.SagFeet <= 0 || p.SpanFeet <= 0 {
		return chord
	}
	drop := 4 * p.SagFeet * x * (p.SpanFeet - x) / (p.SpanFeet * p.SpanFeet)
	return chord - drop
}

// GroundAt returns the ground elevation at a distance along the span, relative
// to the near pole's groundline. The ground is taken as a straight grade
// between the two poles.
func (p Profile) GroundAt(x float64) float64 {
	if p.SpanFeet <= 0 {
		return 0
	}
	if x < 0 {
		x = 0
	}
	if x > p.SpanFeet {
		x = p.SpanFeet
	}
	return -p.GroundDrop.Float() * x / p.SpanFeet
}

// ClearanceAt returns the wire's height above the ground at a distance along
// the span.
func (p Profile) ClearanceAt(x float64) float64 { return p.HeightAt(x) - p.GroundAt(x) }

// MinimumClearance returns the smallest clearance anywhere in the span, and
// where it occurs. On a graded span the tightest point is not the same as the
// lowest point of the wire, so both are worked out rather than assumed.
func (p Profile) MinimumClearance() (unit.Height, float64) {
	if p.SpanFeet <= 0 {
		return 0, 0
	}
	best := math.Inf(1)
	bestAt := 0.0
	steps := 200
	for i := 0; i <= steps; i++ {
		x := p.SpanFeet * float64(i) / float64(steps)
		if c := p.ClearanceAt(x); c < best {
			best = c
			bestAt = x
		}
	}
	return unit.Tenths(int(math.Round(best * 10))), bestAt
}

// MidspanHeight returns the wire height at the middle of the span above the
// near pole's groundline.
func (p Profile) MidspanHeight() unit.Height {
	return unit.Tenths(int(math.Round(p.HeightAt(p.SpanFeet/2) * 10)))
}
