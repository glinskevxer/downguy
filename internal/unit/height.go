// Package unit holds the measurement types the rest of the system stores and
// prints. Nothing here depends on any other package in the repository.
//
// Vertical measurements are held as whole tenths of a foot. Field crews read
// heights off a measuring stick to the nearest tenth, joint use tariffs are
// written in tenths, and every clearance table in NESC rounds to a tenth once
// it has been converted out of inches. Holding them as a float invites two
// surveys of the same pole to disagree in the ninth decimal place, so they are
// integers and the conversion happens once, on the way in.
package unit

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Height is a vertical distance in tenths of a foot. It is signed because
// clearance arithmetic produces deficits as often as it produces surpluses.
type Height int32

// TenthsPerFoot is the scale factor between Height and whole feet.
const TenthsPerFoot = 10

// InchesPerFoot is fixed here so the conversion helpers do not each carry
// their own literal.
const InchesPerFoot = 12

// ErrBadHeight is returned by ParseHeight for anything it cannot read.
var ErrBadHeight = errors.New("unit: cannot read height")

// Feet builds a Height from whole feet.
func Feet(f int) Height { return Height(f * TenthsPerFoot) }

// Tenths builds a Height from whole tenths of a foot.
func Tenths(t int) Height { return Height(t) }

// FeetInches builds a Height from feet and inches, rounding the inches to the
// nearest tenth of a foot. Half a tenth rounds away from zero, which matches
// what a crew does when the stick falls between two marks.
func FeetInches(feet, inches int) Height {
	total := feet*InchesPerFoot + inches
	return Inches(total)
}

// Inches builds a Height from whole inches, rounded to the nearest tenth.
func Inches(in int) Height {
	num := in * TenthsPerFoot
	if num >= 0 {
		return Height((num + InchesPerFoot/2) / InchesPerFoot)
	}
	return Height((num - InchesPerFoot/2) / InchesPerFoot)
}

// Tenths returns the raw tenths of a foot.
func (h Height) Tenths() int { return int(h) }

// WholeFeet returns the foot part, truncated toward zero.
func (h Height) WholeFeet() int { return int(h) / TenthsPerFoot }

// Float returns the height in decimal feet. Use it for arithmetic that has to
// leave the integer domain, never for storage.
func (h Height) Float() float64 { return float64(h) / TenthsPerFoot }

// Abs returns the magnitude of the height.
func (h Height) Abs() Height {
	if h < 0 {
		return -h
	}
	return h
}

// Add returns h plus o.
func (h Height) Add(o Height) Height { return h + o }

// Sub returns h minus o.
func (h Height) Sub(o Height) Height { return h - o }

// String prints decimal feet with a single place and a unit, which is the form
// that goes on a survey sheet.
func (h Height) String() string {
	sign := ""
	v := h
	if v < 0 {
		sign = "-"
		v = -v
	}
	return fmt.Sprintf("%s%d.%d ft", sign, int(v)/TenthsPerFoot, int(v)%TenthsPerFoot)
}

// FeetInches prints the height in feet and inches, rounding the remainder to
// the nearest whole inch. A tenth of a foot is never more than 11 inches once
// rounded, so no carry into the foot is possible here.
func (h Height) FeetInches() string {
	sign := ""
	v := h
	if v < 0 {
		sign = "-"
		v = -v
	}
	feet := int(v) / TenthsPerFoot
	rem := int(v) % TenthsPerFoot
	inches := (rem*InchesPerFoot + TenthsPerFoot/2) / TenthsPerFoot
	return fmt.Sprintf("%s%d' %d\"", sign, feet, inches)
}

// ParseHeight reads the forms a survey file actually contains: decimal feet
// with or without a unit, feet and inches with the prime and double prime
// marks, and a bare inch count.
func ParseHeight(s string) (Height, error) {
	t := strings.TrimSpace(s)
	if t == "" {
		return 0, fmt.Errorf("%w: empty", ErrBadHeight)
	}
	neg := false
	if strings.HasPrefix(t, "-") {
		neg = true
		t = strings.TrimSpace(t[1:])
	}
	h, err := parsePositive(t)
	if err != nil {
		return 0, err
	}
	if neg {
		return -h, nil
	}
	return h, nil
}

func parsePositive(t string) (Height, error) {
	lower := strings.ToLower(t)
	switch {
	case strings.Contains(lower, "'"):
		return parseFeetInches(lower, "'", "\"")
	case strings.Contains(lower, " ft") && strings.Contains(lower, "in"):
		return parseFeetInches(strings.ReplaceAll(lower, " ft", "'"), "'", "in")
	case strings.HasSuffix(lower, "in"):
		n, err := strconv.Atoi(strings.TrimSpace(strings.TrimSuffix(lower, "in")))
		if err != nil {
			return 0, fmt.Errorf("%w: %q", ErrBadHeight, t)
		}
		return Inches(n), nil
	default:
		return parseDecimalFeet(lower)
	}
}

func parseFeetInches(t, footMark, inchMark string) (Height, error) {
	idx := strings.Index(t, footMark)
	if idx < 0 {
		return 0, fmt.Errorf("%w: %q", ErrBadHeight, t)
	}
	feetPart := strings.TrimSpace(t[:idx])
	rest := strings.TrimSpace(t[idx+len(footMark):])
	rest = strings.TrimSuffix(rest, inchMark)
	rest = strings.TrimSpace(rest)
	feet, err := strconv.Atoi(feetPart)
	if err != nil {
		return 0, fmt.Errorf("%w: feet in %q", ErrBadHeight, t)
	}
	if rest == "" {
		return Feet(feet), nil
	}
	inches, err := strconv.Atoi(rest)
	if err != nil {
		return 0, fmt.Errorf("%w: inches in %q", ErrBadHeight, t)
	}
	if inches < 0 || inches >= InchesPerFoot {
		return 0, fmt.Errorf("%w: %d inches out of range", ErrBadHeight, inches)
	}
	return FeetInches(feet, inches), nil
}

func parseDecimalFeet(t string) (Height, error) {
	t = strings.TrimSuffix(t, "ft")
	t = strings.TrimSpace(t)
	whole := t
	frac := ""
	if dot := strings.Index(t, "."); dot >= 0 {
		whole = t[:dot]
		frac = t[dot+1:]
	}
	if whole == "" {
		whole = "0"
	}
	w, err := strconv.Atoi(whole)
	if err != nil {
		return 0, fmt.Errorf("%w: %q", ErrBadHeight, t)
	}
	if frac == "" {
		return Feet(w), nil
	}
	if len(frac) > 3 {
		return 0, fmt.Errorf("%w: %q has more precision than a survey carries", ErrBadHeight, t)
	}
	for len(frac) < 2 {
		frac += "0"
	}
	f, err := strconv.Atoi(frac[:2])
	if err != nil {
		return 0, fmt.Errorf("%w: %q", ErrBadHeight, t)
	}
	tenths := (f + 5) / 10
	if tenths >= TenthsPerFoot {
		w++
		tenths = 0
	}
	return Feet(w) + Tenths(tenths), nil
}
