package unit

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Span is a horizontal distance in whole feet. Spans between distribution
// poles run from about sixty feet in a subdivision to four hundred on a rural
// line, and nobody measures them to a tenth, so the type is whole feet.
type Span int32

// ErrBadSpan is returned when a span cannot be read or is outside the range
// this system will engineer.
var ErrBadSpan = errors.New("unit: cannot read span")

// MinEngineeredSpan and MaxEngineeredSpan bound what the sag routines will
// accept. Below the minimum the parabolic approximation stops being worth
// running, and above the maximum the structure is a transmission crossing that
// this system does not model.
const (
	MinEngineeredSpan Span = 20
	MaxEngineeredSpan Span = 1200
)

// SpanFeet builds a Span from whole feet.
func SpanFeet(f int) Span { return Span(f) }

// Feet returns the span in whole feet.
func (s Span) Feet() int { return int(s) }

// Float returns the span as a float for the sag arithmetic.
func (s Span) Float() float64 { return float64(s) }

// Half returns half the span rounded down, which is where a level span reaches
// its low point.
func (s Span) Half() Span { return s / 2 }

// String prints the span with a unit.
func (s Span) String() string { return fmt.Sprintf("%d ft", int(s)) }

// Valid reports whether the span is inside the engineered range.
func (s Span) Valid() bool { return s >= MinEngineeredSpan && s <= MaxEngineeredSpan }

// ParseSpan reads a span from a plant file. It accepts a bare number and a
// number with a foot unit, and it rounds a decimal to the nearest foot.
func ParseSpan(text string) (Span, error) {
	t := strings.ToLower(strings.TrimSpace(text))
	t = strings.TrimSuffix(t, "ft")
	t = strings.TrimSuffix(t, "'")
	t = strings.TrimSpace(t)
	if t == "" {
		return 0, fmt.Errorf("%w: empty", ErrBadSpan)
	}
	if dot := strings.Index(t, "."); dot >= 0 {
		whole, err := strconv.Atoi(t[:dot])
		if err != nil {
			return 0, fmt.Errorf("%w: %q", ErrBadSpan, text)
		}
		frac := t[dot+1:]
		if frac == "" || !allDigits(frac) {
			return 0, fmt.Errorf("%w: %q", ErrBadSpan, text)
		}
		if frac[0] >= '5' {
			whole++
		}
		return Span(whole), nil
	}
	n, err := strconv.Atoi(t)
	if err != nil {
		return 0, fmt.Errorf("%w: %q", ErrBadSpan, text)
	}
	return Span(n), nil
}

func allDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return len(s) > 0
}

// Temperature is a whole degree Fahrenheit. Sag tables in this trade are
// written in Fahrenheit and there is no reason to convert them twice.
type Temperature int16

// The temperatures the loading rules and the field runs are written at.
const (
	TempHeavyDistrict  Temperature = 0
	TempMediumDistrict Temperature = 15
	TempLightDistrict  Temperature = 30
	TempStringing      Temperature = 60
	TempMaxOperating   Temperature = 120
)

// Degrees returns the temperature in whole degrees Fahrenheit.
func (t Temperature) Degrees() int { return int(t) }

// Float returns the temperature for the change of state arithmetic.
func (t Temperature) Float() float64 { return float64(t) }

// String prints the temperature with its unit.
func (t Temperature) String() string { return fmt.Sprintf("%d F", int(t)) }

// Delta returns t minus other, in degrees.
func (t Temperature) Delta(other Temperature) int { return int(t) - int(other) }
