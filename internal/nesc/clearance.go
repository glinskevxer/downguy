package nesc

import (
	"errors"
	"fmt"

	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/unit"
)

// ErrNoClearance is returned when the table has no entry for the combination
// asked about. It is deliberately an error rather than a zero, because a
// missing clearance that reads as zero passes every check.
var ErrNoClearance = errors.New("nesc: no tabulated clearance")

// Circuit is the electrical class of a span carrying facility. The clearance
// table is written against this rather than against the facility kind, because
// what matters over a road is the voltage, not who owns the wire.
type Circuit string

// The circuit classes the table distinguishes.
const (
	CircuitCommunication Circuit = "communication"
	CircuitNeutral       Circuit = "neutral"
	CircuitSecondary     Circuit = "secondary"
	CircuitPrimary15kV   Circuit = "primary_15kv"
	CircuitPrimary35kV   Circuit = "primary_35kv"
)

// CircuitOf returns the circuit class for a facility, working from its kind
// and its voltage. A facility that does not run into the span has no circuit
// class at all, because the ground clearance rule does not reach it.
func CircuitOf(k plant.Kind, volts int) (Circuit, error) {
	if !k.IsSpanCarrying() {
		return "", fmt.Errorf("%w: %s does not run into the span", ErrNoClearance, k)
	}
	if k.IsCommunication() {
		return CircuitCommunication, nil
	}
	switch k {
	case plant.KindNeutral:
		return CircuitNeutral, nil
	case plant.KindSecondary, plant.KindServiceDrop:
		return CircuitSecondary, nil
	case plant.KindPrimary:
		switch {
		case volts <= 0:
			return "", fmt.Errorf("%w: a primary at %d volts", ErrNoClearance, volts)
		case volts <= 15000:
			return CircuitPrimary15kV, nil
		case volts <= 35000:
			return CircuitPrimary35kV, nil
		default:
			return "", fmt.Errorf("%w: %d volts is above what this system carries", ErrNoClearance, volts)
		}
	}
	return "", fmt.Errorf("%w: %s has no circuit class", ErrNoClearance, k)
}

// groundClearance is the required vertical clearance above the ground cover,
// in tenths of a foot, keyed by cover and then by circuit class. The figures
// are the ones a joint use survey is checked against, at the conductor's final
// sag at its maximum operating temperature.
var groundClearance = map[plant.Cover]map[Circuit]int{
	plant.CoverRoad: {
		CircuitCommunication: 155, CircuitNeutral: 155,
		CircuitSecondary: 160, CircuitPrimary15kV: 185, CircuitPrimary35kV: 190,
	},
	plant.CoverDriveway: {
		CircuitCommunication: 155, CircuitNeutral: 155,
		CircuitSecondary: 160, CircuitPrimary15kV: 185, CircuitPrimary35kV: 190,
	},
	plant.CoverAlley: {
		CircuitCommunication: 155, CircuitNeutral: 155,
		CircuitSecondary: 160, CircuitPrimary15kV: 185, CircuitPrimary35kV: 190,
	},
	plant.CoverParking: {
		CircuitCommunication: 155, CircuitNeutral: 155,
		CircuitSecondary: 160, CircuitPrimary15kV: 185, CircuitPrimary35kV: 190,
	},
	plant.CoverPedestrian: {
		CircuitCommunication: 95, CircuitNeutral: 120,
		CircuitSecondary: 120, CircuitPrimary15kV: 145, CircuitPrimary35kV: 150,
	},
	plant.CoverField: {
		CircuitCommunication: 120, CircuitNeutral: 145,
		CircuitSecondary: 150, CircuitPrimary15kV: 180, CircuitPrimary35kV: 185,
	},
	plant.CoverCultivated: {
		CircuitCommunication: 155, CircuitNeutral: 155,
		CircuitSecondary: 160, CircuitPrimary15kV: 185, CircuitPrimary35kV: 190,
	},
	plant.CoverWaterNoSail: {
		CircuitCommunication: 145, CircuitNeutral: 145,
		CircuitSecondary: 150, CircuitPrimary15kV: 175, CircuitPrimary35kV: 180,
	},
	plant.CoverRailroad: {
		CircuitCommunication: 235, CircuitNeutral: 235,
		CircuitSecondary: 235, CircuitPrimary15kV: 260, CircuitPrimary35kV: 265,
	},
	plant.CoverBuilding: {
		CircuitCommunication: 105, CircuitNeutral: 105,
		CircuitSecondary: 105, CircuitPrimary15kV: 125, CircuitPrimary35kV: 130,
	},
}

// GroundClearance returns the required clearance above a ground cover for a
// circuit class, and the rule that requires it.
func GroundClearance(cover plant.Cover, c Circuit) (unit.Height, string, error) {
	byCircuit, ok := groundClearance[cover]
	if !ok {
		return 0, "", fmt.Errorf("%w: ground cover %q", ErrNoClearance, cover)
	}
	tenths, ok := byCircuit[c]
	if !ok {
		return 0, "", fmt.Errorf("%w: circuit %q over %q", ErrNoClearance, c, cover)
	}
	rule := "NESC-232A"
	if cover == plant.CoverRailroad {
		rule = "NESC-232B"
	}
	if cover == plant.CoverBuilding {
		rule = "NESC-234C"
	}
	return unit.Tenths(tenths), rule, nil
}

// Circuits returns the circuit classes from the least energised to the most.
func Circuits() []Circuit {
	return []Circuit{
		CircuitCommunication, CircuitNeutral, CircuitSecondary,
		CircuitPrimary15kV, CircuitPrimary35kV,
	}
}
