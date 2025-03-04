// Package nesc holds the code rules this system checks against: the loading
// districts, the grades of construction, and the clearance tables.
//
// The tables are written here as data with a rule id attached to every entry,
// because a finding that cannot name the rule it came from is not something an
// attacher can argue with or an engineer can check.
package nesc

import (
	"errors"
	"fmt"
	"strings"

	"github.com/glinskevxer/downguy/internal/unit"
)

// ErrUnknownDistrict is returned for a district code the tables do not carry.
var ErrUnknownDistrict = errors.New("nesc: unknown loading district")

// District is the combined ice and wind loading district the line is in. It is
// the first thing anybody asks about a line, because it sets the ice, the
// wind, the temperature and the constant that every later number is built on.
type District string

// The three districts.
const (
	Heavy  District = "heavy"
	Medium District = "medium"
	Light  District = "light"
)

// Loading is the design case for a district.
type Loading struct {
	District District
	// RadialIce is the thickness of ice assumed all round the conductor, in
	// hundredths of an inch, so that a quarter inch is 25 and nothing has to
	// carry a float.
	RadialIce int
	// WindPressure is the horizontal wind on the projected area, in tenths of
	// a pound per square foot.
	WindPressure int
	// Temperature is the conductor temperature the case is worked out at.
	Temperature unit.Temperature
	// ConstantK is added to the resultant of the vertical and transverse loads
	// to get the design load per foot, in hundredths of a pound.
	ConstantK int
}

var districts = map[District]Loading{
	Heavy: {
		District: Heavy, RadialIce: 50, WindPressure: 40,
		Temperature: unit.TempHeavyDistrict, ConstantK: 30,
	},
	Medium: {
		District: Medium, RadialIce: 25, WindPressure: 40,
		Temperature: unit.TempMediumDistrict, ConstantK: 20,
	},
	Light: {
		District: Light, RadialIce: 0, WindPressure: 90,
		Temperature: unit.TempLightDistrict, ConstantK: 5,
	},
}

// LoadingFor returns the design case for a district.
func LoadingFor(d District) (Loading, error) {
	l, ok := districts[d]
	if !ok {
		return Loading{}, fmt.Errorf("%w: %q", ErrUnknownDistrict, d)
	}
	return l, nil
}

// ParseDistrict reads a district name.
func ParseDistrict(text string) (District, error) {
	t := District(strings.ToLower(strings.TrimSpace(text)))
	if _, ok := districts[t]; ok {
		return t, nil
	}
	return "", fmt.Errorf("%w: %q", ErrUnknownDistrict, text)
}

// Districts returns the three districts from the most severe to the least.
func Districts() []District { return []District{Heavy, Medium, Light} }

// IceInches returns the radial ice thickness in inches.
func (l Loading) IceInches() float64 { return float64(l.RadialIce) / 100 }

// WindPSF returns the wind pressure in pounds per square foot.
func (l Loading) WindPSF() float64 { return float64(l.WindPressure) / 10 }

// KPoundsPerFoot returns the constant added to the resultant, in pounds per
// foot.
func (l Loading) KPoundsPerFoot() float64 { return float64(l.ConstantK) / 100 }

// IceDensity is the weight of the ice the code assumes, in pounds per cubic
// foot.
const IceDensity = 57.0

// IceWeightPerFoot returns the weight of the ice sleeve on a conductor of the
// given bare diameter, in pounds per foot. The sleeve is an annulus, so the
// weight goes with the ice thickness times the sum of the bare diameter and
// the thickness, not with the thickness alone.
func (l Loading) IceWeightPerFoot(bareDiameterInches float64) float64 {
	if l.RadialIce == 0 || bareDiameterInches <= 0 {
		return 0
	}
	t := l.IceInches()
	areaSquareInches := 3.141592653589793 * t * (bareDiameterInches + t)
	return areaSquareInches / 144 * IceDensity
}

// IcedDiameter returns the diameter of the conductor with its ice sleeve, in
// inches. It is what the wind acts on.
func (l Loading) IcedDiameter(bareDiameterInches float64) float64 {
	return bareDiameterInches + 2*l.IceInches()
}

// WindLoadPerFoot returns the horizontal wind force on a foot of iced
// conductor, in pounds.
func (l Loading) WindLoadPerFoot(bareDiameterInches float64) float64 {
	if bareDiameterInches <= 0 {
		return 0
	}
	return l.IcedDiameter(bareDiameterInches) / 12 * l.WindPSF()
}

// Severity orders the districts, with a higher number meaning a heavier case.
func (d District) Severity() int {
	switch d {
	case Heavy:
		return 3
	case Medium:
		return 2
	case Light:
		return 1
	}
	return 0
}

// String returns the district name.
func (d District) String() string { return string(d) }
