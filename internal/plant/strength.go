package plant

import (
	"errors"
	"fmt"
	"math"
)

// ErrNoTable is returned when a pole length and class combination is not one
// the standard tabulates.
var ErrNoTable = errors.New("plant: no table entry for that pole")

// fibreStress is the designated fibre stress in pounds per square inch for
// each species. It is the number the groundline moment capacity is built on,
// and it is the reason two poles of the same class do not have the same
// strength.
var fibreStress = map[Species]int{
	SouthernPine:    8000,
	DouglasFir:      8000,
	WesternLarch:    8400,
	LodgepolePine:   6600,
	RedPine:         6600,
	JackPine:        6600,
	WesternRedCedar: 6000,
	PonderosaPine:   6000,
}

// FibreStress returns the designated fibre stress for the species and reports
// whether the species is tabulated.
func FibreStress(s Species) (int, bool) {
	v, ok := fibreStress[s]
	return v, ok
}

// buttCircumference holds the circumference six feet from the butt, in whole
// inches, keyed by class and then by pole length. These are the southern pine
// figures; the other species are derived from them by the ratio of their
// fibre stress, which is how the standard builds its own tables.
var buttCircumference = map[Class]map[int]int{
	ClassH2: {45: 43, 50: 45, 55: 47, 60: 48, 65: 50, 70: 51},
	ClassH1: {40: 39, 45: 41, 50: 43, 55: 44, 60: 46, 65: 47, 70: 49},
	Class1:  {30: 33, 35: 35, 40: 37, 45: 39, 50: 41, 55: 42, 60: 44, 65: 45, 70: 46},
	Class2:  {25: 29, 30: 31, 35: 33, 40: 35, 45: 37, 50: 38, 55: 40, 60: 41, 65: 43},
	Class3:  {25: 27, 30: 29, 35: 31, 40: 33, 45: 34, 50: 36, 55: 37, 60: 39},
	Class4:  {20: 23, 25: 25, 30: 27, 35: 29, 40: 31, 45: 32, 50: 34, 55: 35},
	Class5:  {20: 22, 25: 24, 30: 25, 35: 27, 40: 29, 45: 30},
	Class6:  {20: 20, 25: 22, 30: 23, 35: 25},
	Class7:  {20: 19, 25: 20, 30: 22},
}

// ButtCircumference returns the circumference six feet from the butt for a
// southern pine pole of the class and length. Poles are bought by class and
// length, so the pair is the whole key.
func ButtCircumference(c Class, length int) (int, error) {
	byLength, ok := buttCircumference[c]
	if !ok {
		return 0, fmt.Errorf("%w: class %s", ErrNoTable, c)
	}
	v, ok := byLength[length]
	if !ok {
		return 0, fmt.Errorf("%w: class %s at %d feet", ErrNoTable, c, length)
	}
	return v, nil
}

// SectionModulus returns the section modulus of a round section of the given
// circumference, in cubic inches. For a circle this is the cube of the
// circumference over thirty two pi squared.
func SectionModulus(circumferenceInches float64) float64 {
	return math.Pow(circumferenceInches, 3) / (32 * math.Pi * math.Pi)
}

// GroundlineCapacity returns the ultimate resisting moment of a pole at the
// groundline in foot pounds. It is the fibre stress times the section modulus
// at the groundline, which for a set pole is taken at the six foot from butt
// circumference.
func GroundlineCapacity(p Pole) (float64, error) {
	stress, ok := FibreStress(p.Species)
	if !ok {
		return 0, fmt.Errorf("%w: species %s", ErrNoTable, p.Species)
	}
	circ, err := ButtCircumference(p.Class, p.Length)
	if err != nil {
		return 0, err
	}
	modulus := SectionModulus(float64(circ))
	inchPounds := float64(stress) * modulus
	return inchPounds / 12, nil
}

// CircumferenceAt returns the circumference of the pole at a height above the
// groundline, in inches. A pole is a truncated cone, so the circumference runs
// straight from the six foot from butt figure down to the class top figure at
// the tip.
func CircumferenceAt(p Pole, feetAboveGrade float64) (float64, error) {
	butt, err := ButtCircumference(p.Class, p.Length)
	if err != nil {
		return 0, err
	}
	top, ok := TopCircumference(p.Class)
	if !ok {
		return 0, fmt.Errorf("%w: class %s", ErrNoTable, p.Class)
	}
	above := p.AboveGrade().Float()
	if above <= 0 {
		return 0, fmt.Errorf("%w: %s has no height above grade", ErrNoTable, p.ID)
	}
	if feetAboveGrade < 0 {
		feetAboveGrade = 0
	}
	if feetAboveGrade > above {
		feetAboveGrade = above
	}
	taper := (float64(butt) - float64(top)) / above
	return float64(butt) - taper*feetAboveGrade, nil
}
