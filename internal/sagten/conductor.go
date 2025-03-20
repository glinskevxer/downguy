// Package sagten works out where a wire actually is between two poles.
//
// A clearance is not a property of a pole. It is a property of the wire at the
// point in the span where it hangs lowest, at the temperature and under the
// load the code says to check it at. Everything in this package exists to turn
// a conductor, a span and a loading case into that one height.
package sagten

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ErrUnknownConductor is returned for a conductor code the catalogue does not
// carry.
var ErrUnknownConductor = errors.New("sagten: unknown conductor")

// Conductor is the physical description of a wire. Every field here feeds the
// change of state equation, which is why they are all on one record rather
// than being looked up separately.
type Conductor struct {
	// Code is the trade name the catalogue is keyed on.
	Code string
	// Kind separates a bare supply conductor from a communication cable, which
	// matters because they are strung to different tension limits.
	Kind string
	// DiameterInches is the bare overall diameter.
	DiameterInches float64
	// WeightPerFoot is the bare weight in pounds per foot.
	WeightPerFoot float64
	// AreaSquareInches is the cross sectional area the modulus applies to.
	AreaSquareInches float64
	// RatedStrength is the ultimate tensile strength in pounds.
	RatedStrength float64
	// ModulusPSI is the final modulus of elasticity.
	ModulusPSI float64
	// ThermalCoefficient is the linear expansion per degree Fahrenheit.
	ThermalCoefficient float64
}

var catalogue = map[string]Conductor{
	"6 ACSR RAVEN": {
		Code: "6 ACSR RAVEN", Kind: "supply",
		DiameterInches: 0.398, WeightPerFoot: 0.1451, AreaSquareInches: 0.0969,
		RatedStrength: 3277, ModulusPSI: 11.6e6, ThermalCoefficient: 10.4e-6,
	},
	"4 ACSR SWAN": {
		Code: "4 ACSR SWAN", Kind: "supply",
		DiameterInches: 0.447, WeightPerFoot: 0.1828, AreaSquareInches: 0.1222,
		RatedStrength: 4062, ModulusPSI: 11.6e6, ThermalCoefficient: 10.4e-6,
	},
	"2 ACSR SPARROW": {
		Code: "2 ACSR SPARROW", Kind: "supply",
		DiameterInches: 0.563, WeightPerFoot: 0.2906, AreaSquareInches: 0.1943,
		RatedStrength: 5926, ModulusPSI: 11.6e6, ThermalCoefficient: 10.4e-6,
	},
	"1/0 ACSR RAVEN": {
		Code: "1/0 ACSR RAVEN", Kind: "supply",
		DiameterInches: 0.398, WeightPerFoot: 0.1451, AreaSquareInches: 0.0969,
		RatedStrength: 4380, ModulusPSI: 11.6e6, ThermalCoefficient: 10.4e-6,
	},
	"4/0 ACSR PENGUIN": {
		Code: "4/0 ACSR PENGUIN", Kind: "supply",
		DiameterInches: 0.563, WeightPerFoot: 0.2911, AreaSquareInches: 0.1939,
		RatedStrength: 8350, ModulusPSI: 11.6e6, ThermalCoefficient: 10.9e-6,
	},
	"336 ACSR MERLIN": {
		Code: "336 ACSR MERLIN", Kind: "supply",
		DiameterInches: 0.684, WeightPerFoot: 0.3625, AreaSquareInches: 0.2646,
		RatedStrength: 8680, ModulusPSI: 9.9e6, ThermalCoefficient: 11.7e-6,
	},
	"1/4 EHS MESSENGER": {
		Code: "1/4 EHS MESSENGER", Kind: "communication",
		DiameterInches: 0.250, WeightPerFoot: 0.121, AreaSquareInches: 0.0378,
		RatedStrength: 6650, ModulusPSI: 27.0e6, ThermalCoefficient: 6.5e-6,
	},
	"5/16 EHS MESSENGER": {
		Code: "5/16 EHS MESSENGER", Kind: "communication",
		DiameterInches: 0.3125, WeightPerFoot: 0.205, AreaSquareInches: 0.0589,
		RatedStrength: 11200, ModulusPSI: 27.0e6, ThermalCoefficient: 6.5e-6,
	},
	"3/8 EHS MESSENGER": {
		Code: "3/8 EHS MESSENGER", Kind: "communication",
		DiameterInches: 0.375, WeightPerFoot: 0.273, AreaSquareInches: 0.0799,
		RatedStrength: 15400, ModulusPSI: 27.0e6, ThermalCoefficient: 6.5e-6,
	},
	"144F ADSS": {
		Code: "144F ADSS", Kind: "communication",
		DiameterInches: 0.551, WeightPerFoot: 0.116, AreaSquareInches: 0.0435,
		RatedStrength: 5620, ModulusPSI: 2.3e6, ThermalCoefficient: 3.0e-6,
	},
	"625 COAX": {
		Code: "625 COAX", Kind: "communication",
		DiameterInches: 0.750, WeightPerFoot: 0.229, AreaSquareInches: 0.0510,
		RatedStrength: 1200, ModulusPSI: 4.5e6, ThermalCoefficient: 12.8e-6,
	},
}

// Lookup returns a conductor by code, matched without regard to case or
// surrounding space.
func Lookup(code string) (Conductor, error) {
	c, ok := catalogue[strings.ToUpper(strings.TrimSpace(code))]
	if !ok {
		return Conductor{}, fmt.Errorf("%w: %q", ErrUnknownConductor, code)
	}
	return c, nil
}

// Codes returns every catalogued conductor code, sorted.
func Codes() []string {
	out := make([]string, 0, len(catalogue))
	for code := range catalogue {
		out = append(out, code)
	}
	sort.Strings(out)
	return out
}

// IsCommunication reports whether the conductor is strung as a communication
// facility.
func (c Conductor) IsCommunication() bool { return c.Kind == "communication" }

// Validate checks that a conductor record can be used in the sag arithmetic
// without producing a division by zero or a negative root.
func (c Conductor) Validate() error {
	switch {
	case strings.TrimSpace(c.Code) == "":
		return fmt.Errorf("%w: conductor has no code", ErrUnknownConductor)
	case c.DiameterInches <= 0:
		return fmt.Errorf("%w: %s has diameter %v", ErrUnknownConductor, c.Code, c.DiameterInches)
	case c.WeightPerFoot <= 0:
		return fmt.Errorf("%w: %s weighs %v per foot", ErrUnknownConductor, c.Code, c.WeightPerFoot)
	case c.AreaSquareInches <= 0:
		return fmt.Errorf("%w: %s has area %v", ErrUnknownConductor, c.Code, c.AreaSquareInches)
	case c.RatedStrength <= 0:
		return fmt.Errorf("%w: %s is rated %v pounds", ErrUnknownConductor, c.Code, c.RatedStrength)
	case c.ModulusPSI <= 0:
		return fmt.Errorf("%w: %s has modulus %v", ErrUnknownConductor, c.Code, c.ModulusPSI)
	case c.ThermalCoefficient <= 0:
		return fmt.Errorf("%w: %s expands at %v", ErrUnknownConductor, c.Code, c.ThermalCoefficient)
	}
	return nil
}

// TensionLimit returns the working tension the conductor is strung to as a
// fraction of its rated strength under the given condition. Supply conductors
// are held to a quarter of rating at their everyday temperature and a third
// under the loading case; messengers are strung harder because a sagging
// communication cable is a clearance problem before it is a strength one.
func (c Conductor) TensionLimit(underLoad bool) float64 {
	if c.IsCommunication() {
		if underLoad {
			return 0.50
		}
		return 0.35
	}
	if underLoad {
		return 0.33
	}
	return 0.25
}

// MaxTension returns the working tension limit in pounds.
func (c Conductor) MaxTension(underLoad bool) float64 {
	return c.RatedStrength * c.TensionLimit(underLoad)
}
