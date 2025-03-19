package nesc

import (
	"fmt"

	"github.com/glinskevxer/downguy/internal/unit"
)

// The separations that hold a joint use pole together, in inches. They are
// written as inches because that is how the code writes them and how a field
// crew argues about them, and converted once on the way out.
const (
	// SafetyZoneInches is the gap a communication worker has to be able to
	// stand in between the top of communication space and the lowest supply
	// facility. It is the single most cited number in joint use.
	SafetyZoneInches = 40
	// SafetyZoneReducedInches applies where the supply facility directly above
	// is an effectively grounded neutral and nothing else.
	SafetyZoneReducedInches = 30
	// CommToCommInches is the separation between two attachers in
	// communication space, which exists so each can work without the other
	// coming down.
	CommToCommInches = 12
	// CommToCommSameOwnerInches applies where both facilities belong to one
	// attacher, who can work on both.
	CommToCommSameOwnerInches = 4
	// EquipmentToConductorInches is the clearance from a box or a power supply
	// in communication space up to the facility above it.
	EquipmentToConductorInches = 30
	// VerticalRunInches is the clearance a riser or a vertical run keeps from
	// other facilities on the pole face.
	VerticalRunInches = 3
	// ClimbingSpaceInches is the side of the square kept clear up the climbing
	// face of the pole.
	ClimbingSpaceInches = 30
	// MidspanCommInches is the separation two communication facilities keep
	// from each other out in the span, where sag has closed the gap they had
	// at the poles.
	MidspanCommInches = 12
	// MidspanSupplyToCommInches is the separation supply keeps from
	// communication at mid span.
	MidspanSupplyToCommInches = 30
)

// SafetyZone returns the required separation between the lowest supply
// facility and the highest communication facility, and the rule that requires
// it. The reduced figure applies only when the supply facility immediately
// above communication space is an effectively grounded neutral.
func SafetyZone(lowestSupplyIsGroundedNeutral bool) (unit.Height, string) {
	if lowestSupplyIsGroundedNeutral {
		return unit.Inches(SafetyZoneReducedInches), "NESC-235C1"
	}
	return unit.Inches(SafetyZoneInches), "NESC-235C1"
}

// CommunicationSeparation returns the separation two communication facilities
// keep on a structure, and the rule. Two facilities belonging to the same
// attacher may sit closer, because one crew owns both.
func CommunicationSeparation(sameOwner bool) (unit.Height, string) {
	if sameOwner {
		return unit.Inches(CommToCommSameOwnerInches), "NESC-235C2"
	}
	return unit.Inches(CommToCommInches), "NESC-235C2"
}

// MidspanSeparation returns the separation required at mid span between two
// facilities, and the rule. Supply over communication is held further apart
// than two communication facilities are.
func MidspanSeparation(upperIsSupply bool) (unit.Height, string) {
	if upperIsSupply {
		return unit.Inches(MidspanSupplyToCommInches), "NESC-235E"
	}
	return unit.Inches(MidspanCommInches), "NESC-235E"
}

// EquipmentClearance returns the clearance a box or a power supply in
// communication space keeps below the facility above it, and the rule.
func EquipmentClearance() (unit.Height, string) {
	return unit.Inches(EquipmentToConductorInches), "NESC-238A"
}

// VerticalRunClearance returns the clearance a riser keeps from other
// facilities on the pole face, and the rule.
func VerticalRunClearance() (unit.Height, string) {
	return unit.Inches(VerticalRunInches), "NESC-239E"
}

// ClimbingSpace returns the side of the clear square up the climbing face, and
// the rule.
func ClimbingSpace() (unit.Height, string) {
	return unit.Inches(ClimbingSpaceInches), "NESC-236A"
}

// Deficit returns how far short a measured gap falls of a required one. A gap
// that meets or beats the requirement has no deficit, which is a zero rather
// than a negative number, so that a report can sum deficits without them
// cancelling out.
func Deficit(measured, required unit.Height) unit.Height {
	if measured >= required {
		return 0
	}
	return required.Sub(measured)
}

// Meets reports whether a measured gap satisfies a required one.
func Meets(measured, required unit.Height) bool { return measured >= required }

// DescribeDeficit writes the sentence a finding carries.
func DescribeDeficit(measured, required unit.Height) string {
	short := Deficit(measured, required)
	if short == 0 {
		return fmt.Sprintf("%s measured against %s required", measured, required)
	}
	return fmt.Sprintf("%s measured against %s required, short by %s", measured, required, short)
}
