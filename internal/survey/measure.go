// Package survey holds what the field crew actually wrote down, and the rules
// for deciding whether it is good enough to engineer from.
//
// A survey is not the plant record. It is a set of observations taken on one
// day by one crew with one instrument, and every one of them has a tolerance.
// Keeping the two apart is what lets the system say "the record says 20.0 and
// the crew measured 19.4" instead of quietly overwriting one with the other.
package survey

import (
	"errors"
	"fmt"
	"strings"

	"github.com/glinskevxer/downguy/internal/unit"
)

// ErrBadMeasurement covers every rejected observation.
var ErrBadMeasurement = errors.New("survey: bad measurement")

// Method is how a height was obtained. It decides the tolerance the reading
// carries, which decides whether a marginal clearance can be called either
// way without going back out.
type Method string

// The methods this utility's crews use.
const (
	MethodStick     Method = "stick"
	MethodLaser     Method = "laser"
	MethodBucket    Method = "bucket"
	MethodPhoto     Method = "photogrammetry"
	MethodEstimated Method = "estimated"
	MethodRecord    Method = "from_record"
)

// toleranceTenths is the half width of the band a reading is trusted to, in
// tenths of a foot.
var toleranceTenths = map[Method]int{
	MethodStick:     2,
	MethodLaser:     1,
	MethodBucket:    1,
	MethodPhoto:     5,
	MethodEstimated: 10,
	MethodRecord:    5,
}

// KnownMethod reports whether the method has a tolerance.
func KnownMethod(m Method) bool {
	_, ok := toleranceTenths[m]
	return ok
}

// Tolerance returns the half width of the band a reading by this method is
// trusted to.
func Tolerance(m Method) (unit.Height, bool) {
	t, ok := toleranceTenths[m]
	return unit.Tenths(t), ok
}

// ParseMethod reads a method written with a space, a dash or an underscore.
func ParseMethod(text string) (Method, error) {
	t := strings.ToLower(strings.TrimSpace(text))
	t = strings.ReplaceAll(t, " ", "_")
	t = strings.ReplaceAll(t, "-", "_")
	if KnownMethod(Method(t)) {
		return Method(t), nil
	}
	return "", fmt.Errorf("%w: unknown method %q", ErrBadMeasurement, text)
}

// Reliable reports whether a method is good enough to make a clearance
// decision on without a second visit.
func (m Method) Reliable() bool {
	switch m {
	case MethodStick, MethodLaser, MethodBucket:
		return true
	}
	return false
}

// Measurement is one height read in the field.
type Measurement struct {
	Subject string
	Height  unit.Height
	Method  Method
	Crew    string
	Note    string
}

// Validate checks a measurement.
func (m Measurement) Validate() error {
	if strings.TrimSpace(m.Subject) == "" {
		return fmt.Errorf("%w: no subject", ErrBadMeasurement)
	}
	if !KnownMethod(m.Method) {
		return fmt.Errorf("%w: %s was taken by %q", ErrBadMeasurement, m.Subject, m.Method)
	}
	if m.Height <= 0 {
		return fmt.Errorf("%w: %s reads %s", ErrBadMeasurement, m.Subject, m.Height)
	}
	if m.Height > unit.Feet(120) {
		return fmt.Errorf("%w: %s reads %s, which is taller than any pole here", ErrBadMeasurement, m.Subject, m.Height)
	}
	if strings.TrimSpace(m.Crew) == "" {
		return fmt.Errorf("%w: %s has no crew against it", ErrBadMeasurement, m.Subject)
	}
	return nil
}

// Band returns the range the true height is believed to lie in.
func (m Measurement) Band() (unit.Height, unit.Height) {
	t, ok := Tolerance(m.Method)
	if !ok {
		return m.Height, m.Height
	}
	return m.Height.Sub(t), m.Height.Add(t)
}

// Clears reports whether the measurement clears a requirement outright, is
// inside its own tolerance of it, or fails outright. The three way answer is
// the point: a reading that is short by less than its own tolerance is not a
// violation, it is a reason to go back with a laser.
func (m Measurement) Clears(required unit.Height) Verdict {
	low, high := m.Band()
	switch {
	case low >= required:
		return VerdictClear
	case high < required:
		return VerdictShort
	default:
		return VerdictMarginal
	}
}

// Verdict is the three way answer a measurement gives against a requirement.
type Verdict string

// The three verdicts.
const (
	VerdictClear    Verdict = "clear"
	VerdictMarginal Verdict = "marginal"
	VerdictShort    Verdict = "short"
)

// Failing reports whether the verdict is one that has to be written up.
func (v Verdict) Failing() bool { return v == VerdictShort }

// String prints the measurement the way a survey sheet lists it.
func (m Measurement) String() string {
	return fmt.Sprintf("%s %s by %s (%s)", m.Subject, m.Height, m.Method, m.Crew)
}
