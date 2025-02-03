// Package plant holds the physical record: the poles, what is framed on them,
// what is attached to them, and the spans between them.
//
// Nothing in this package computes a clearance or a load. It is the inventory
// the engineering packages read.
package plant

import (
	"errors"
	"fmt"
	"strings"

	"github.com/glinskevxer/downguy/internal/unit"
)

// PoleID is the tag number stamped on the pole. It is the key everything else
// in the system hangs off, so it is validated once, here, rather than being
// trusted wherever it turns up.
type PoleID string

// ErrBadPole covers every rejected pole record.
var ErrBadPole = errors.New("plant: bad pole record")

// ParsePoleID normalises and checks a tag number. Tags in this utility run as
// a two letter district prefix, a dash, and six or seven digits.
func ParsePoleID(text string) (PoleID, error) {
	t := strings.ToUpper(strings.TrimSpace(text))
	dash := strings.Index(t, "-")
	if dash != 2 {
		return "", fmt.Errorf("%w: %q has no two letter district prefix", ErrBadPole, text)
	}
	for i := 0; i < 2; i++ {
		if t[i] < 'A' || t[i] > 'Z' {
			return "", fmt.Errorf("%w: %q prefix is not letters", ErrBadPole, text)
		}
	}
	digits := t[dash+1:]
	if len(digits) < 6 || len(digits) > 7 {
		return "", fmt.Errorf("%w: %q has %d digits, want six or seven", ErrBadPole, text, len(digits))
	}
	for i := 0; i < len(digits); i++ {
		if digits[i] < '0' || digits[i] > '9' {
			return "", fmt.Errorf("%w: %q is not all digits after the dash", ErrBadPole, text)
		}
	}
	return PoleID(t), nil
}

// District returns the two letter prefix.
func (p PoleID) District() string {
	if len(p) < 2 {
		return ""
	}
	return string(p[:2])
}

// String returns the tag as written.
func (p PoleID) String() string { return string(p) }

// Species is the wood the pole is cut from. It decides the fibre stress used
// in the loading calculation and the setting depth allowance in rock.
type Species string

// The species this utility buys.
const (
	SouthernPine    Species = "SPF"
	DouglasFir      Species = "DFR"
	WesternRedCedar Species = "WRC"
	LodgepolePine   Species = "LPP"
	WesternLarch    Species = "WLC"
	RedPine         Species = "RDP"
	JackPine        Species = "JKP"
	PonderosaPine   Species = "PPN"
)

var speciesNames = map[Species]string{
	SouthernPine:    "southern yellow pine",
	DouglasFir:      "douglas fir",
	WesternRedCedar: "western red cedar",
	LodgepolePine:   "lodgepole pine",
	WesternLarch:    "western larch",
	RedPine:         "red pine",
	JackPine:        "jack pine",
	PonderosaPine:   "ponderosa pine",
}

// KnownSpecies reports whether the code is one this system carries tables for.
func KnownSpecies(s Species) bool {
	_, ok := speciesNames[s]
	return ok
}

// Name returns the long form for a report.
func (s Species) Name() string {
	if n, ok := speciesNames[s]; ok {
		return n
	}
	return "unknown species " + string(s)
}

// ParseSpecies reads a species code or its long name.
func ParseSpecies(text string) (Species, error) {
	t := strings.ToUpper(strings.TrimSpace(text))
	if KnownSpecies(Species(t)) {
		return Species(t), nil
	}
	lower := strings.ToLower(strings.TrimSpace(text))
	for code, name := range speciesNames {
		if name == lower {
			return code, nil
		}
	}
	return "", fmt.Errorf("%w: unknown species %q", ErrBadPole, text)
}

// Condition is the field assessment of the pole itself, which scales the
// strength the loading calculation is allowed to count on.
type Condition string

// The condition grades a groundline inspection returns.
const (
	ConditionNew        Condition = "new"
	ConditionSound      Condition = "sound"
	ConditionServiceabe Condition = "serviceable"
	ConditionRestorable Condition = "restorable"
	ConditionReject     Condition = "reject"
	ConditionUnknown    Condition = "unknown"
)

var conditionOrder = map[Condition]int{
	ConditionNew:        0,
	ConditionSound:      1,
	ConditionServiceabe: 2,
	ConditionRestorable: 3,
	ConditionReject:     4,
	ConditionUnknown:    5,
}

// KnownCondition reports whether the grade is one the tables cover.
func KnownCondition(c Condition) bool {
	_, ok := conditionOrder[c]
	return ok
}

// WorseThan reports whether c is a worse grade than other.
func (c Condition) WorseThan(other Condition) bool {
	return conditionOrder[c] > conditionOrder[other]
}

// Pole is one structure in the inventory.
type Pole struct {
	ID          PoleID
	Length      int
	Class       Class
	Species     Species
	SetDepth    unit.Height
	InstalledIn int
	Owner       string
	Condition   Condition
	Latitude    float64
	Longitude   float64
}

// AboveGrade returns the height of the pole top over the ground.
func (p Pole) AboveGrade() unit.Height { return unit.Feet(p.Length).Sub(p.SetDepth) }

// StandardSetDepth returns the setting depth this utility specifies for a pole
// of the given length in normal soil: six feet at thirty, and six more inches
// for every five feet of length after that.
func StandardSetDepth(length int) unit.Height {
	if length <= 30 {
		return unit.Feet(6)
	}
	steps := (length - 30 + 4) / 5
	return unit.Feet(6).Add(unit.Tenths(5 * steps))
}

// Validate checks a pole record hard enough that no engineering package has to
// check it again.
func (p Pole) Validate() error {
	if _, err := ParsePoleID(string(p.ID)); err != nil {
		return err
	}
	if p.Length < 20 || p.Length > 125 {
		return fmt.Errorf("%w: %s is %d feet long", ErrBadPole, p.ID, p.Length)
	}
	if p.Length%5 != 0 {
		return fmt.Errorf("%w: %s length %d is not a stock length", ErrBadPole, p.ID, p.Length)
	}
	if !KnownClass(p.Class) {
		return fmt.Errorf("%w: %s has class %q", ErrBadPole, p.ID, p.Class)
	}
	if !KnownSpecies(p.Species) {
		return fmt.Errorf("%w: %s has species %q", ErrBadPole, p.ID, p.Species)
	}
	if !KnownCondition(p.Condition) {
		return fmt.Errorf("%w: %s has condition %q", ErrBadPole, p.ID, p.Condition)
	}
	if p.SetDepth <= 0 || p.SetDepth >= unit.Feet(p.Length) {
		return fmt.Errorf("%w: %s is set %s into a %d foot pole", ErrBadPole, p.ID, p.SetDepth, p.Length)
	}
	if p.Owner == "" {
		return fmt.Errorf("%w: %s has no owner", ErrBadPole, p.ID)
	}
	return nil
}

// Age returns the pole's age in years as of the given year.
func (p Pole) Age(asOfYear int) int {
	if p.InstalledIn == 0 {
		return 0
	}
	return asOfYear - p.InstalledIn
}

// String prints the pole the way a work order names it.
func (p Pole) String() string {
	return fmt.Sprintf("%s %d/%s %s", p.ID, p.Length, p.Class, p.Species)
}
