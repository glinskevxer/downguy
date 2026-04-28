package nesc

import (
	"errors"
	"fmt"
	"strings"
)

// ErrUnknownGrade is returned for a grade of construction the tables do not
// carry.
var ErrUnknownGrade = errors.New("nesc: unknown grade of construction")

// Grade is the grade of construction required for a structure. It is not a
// quality rating: it is set by what the line crosses and what it carries, and
// it decides how much margin the strength calculation has to leave.
type Grade string

// The grades this system works in.
const (
	GradeB Grade = "B"
	GradeC Grade = "C"
	GradeN Grade = "N"
)

// Factors are the multipliers a strength check applies. The load is multiplied
// up by the overload factor and the capacity is multiplied down by the
// strength factor, so a pole passes when factored load is under factored
// capacity.
type Factors struct {
	Grade Grade
	// TransverseWind is applied to the wind load on wires and structure.
	TransverseWind float64
	// TransverseWire is applied to the tension component at a line angle.
	TransverseWire float64
	// Vertical is applied to the weight of wires, ice and equipment.
	Vertical float64
	// Longitudinal is applied to an unbalanced pull along the line.
	Longitudinal float64
	// WoodStrength scales the pole capacity down at installation.
	WoodStrength float64
	// WoodAtReplacement is the reduced strength factor a pole is allowed to
	// fall to before it has to be replaced.
	WoodAtReplacement float64
}

var gradeFactors = map[Grade]Factors{
	GradeB: {
		Grade: GradeB, TransverseWind: 2.50, TransverseWire: 1.65,
		Vertical: 1.50, Longitudinal: 1.10, WoodStrength: 0.65, WoodAtReplacement: 0.50,
	},
	GradeC: {
		Grade: GradeC, TransverseWind: 2.20, TransverseWire: 1.30,
		Vertical: 1.90, Longitudinal: 1.00, WoodStrength: 0.85, WoodAtReplacement: 0.67,
	},
	GradeN: {
		Grade: GradeN, TransverseWind: 1.00, TransverseWire: 1.00,
		Vertical: 1.00, Longitudinal: 1.00, WoodStrength: 1.00, WoodAtReplacement: 1.00,
	},
}

// FactorsFor returns the factors for a grade.
func FactorsFor(g Grade) (Factors, error) {
	f, ok := gradeFactors[g]
	if !ok {
		return Factors{}, fmt.Errorf("%w: %q", ErrUnknownGrade, g)
	}
	return f, nil
}

// ParseGrade reads a grade letter.
func ParseGrade(text string) (Grade, error) {
	t := Grade(strings.ToUpper(strings.TrimSpace(text)))
	if _, ok := gradeFactors[t]; ok {
		return t, nil
	}
	return "", fmt.Errorf("%w: %q", ErrUnknownGrade, text)
}

// Grades returns the grades from the most demanding to the least.
func Grades() []Grade { return []Grade{GradeB, GradeC, GradeN} }

// Rank returns the position of a grade in the demand order, and reports
// whether the grade is known. A lower rank is a stricter grade.
func (g Grade) Rank() (int, bool) {
	for i, k := range Grades() {
		if k == g {
			return i, true
		}
	}
	return 0, false
}

// Stricter reports whether g demands more than other.
func (g Grade) Stricter(other Grade) bool {
	a, oka := g.Rank()
	b, okb := other.Rank()
	if !oka || !okb {
		return false
	}
	return a < b
}

// RequiredGrade returns the grade a supply conductor has to be built to over a
// given crossing, and the rule that says so. Supply over a railroad or over a
// communication line is grade B, ordinary supply along a road is grade C, and
// a line over open land it owns is grade N.
func RequiredGrade(crossesRailroad, crossesCommunication, isSupply bool) (Grade, string) {
	if !isSupply {
		if crossesRailroad {
			return GradeC, "NESC-242B"
		}
		return GradeN, "NESC-242A"
	}
	if crossesRailroad {
		return GradeB, "NESC-242C1"
	}
	if crossesCommunication {
		return GradeB, "NESC-242C2"
	}
	return GradeC, "NESC-242D"
}

// Governing returns the stricter of two grades, so that a structure carrying
// two crossings is built to the harder one.
func Governing(a, b Grade) Grade {
	if a.Stricter(b) {
		return a
	}
	if b.Stricter(a) {
		return b
	}
	return a
}

// KnownGradeForCheck reports whether a grade is one a check can be carried out
// at. It is Rank without the position, which is what nearly every caller
// actually wants.
func KnownGradeForCheck(g Grade) bool {
	_, ok := g.Rank()
	return ok
}

// String returns the grade letter.
func (g Grade) String() string { return string(g) }
