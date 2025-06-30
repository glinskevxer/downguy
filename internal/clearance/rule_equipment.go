package clearance

import (
	"fmt"

	"github.com/glinskevxer/downguy/internal/nesc"
	"github.com/glinskevxer/downguy/internal/plant"
)

// EquipmentRule checks the gap between a box or a power supply in
// communication space and whatever sits directly above it. An amplifier
// housing is a foot deep and a worker opens it standing on a ladder, so it
// needs more room over it than a cable does.
type EquipmentRule struct{}

// ID returns the catalogue rule id.
func (EquipmentRule) ID() string { return "NESC-238A" }

// Check returns a finding for every piece of equipment that is too close to
// the facility above it.
func (r EquipmentRule) Check(c PoleContext) Findings {
	required, ruleID := nesc.EquipmentClearance()
	rule := nesc.MustRule(ruleID)
	a := c.Assembly()
	var out Findings
	for _, att := range a.Attachments {
		if !isEquipment(att.Kind) {
			continue
		}
		above, ok := a.Lowest(func(other plant.Attachment) bool {
			return other.ID != att.ID && other.Height > att.Height
		})
		if !ok {
			continue
		}
		measured := above.Height.Sub(att.Height)
		if nesc.Meets(measured, required) {
			continue
		}
		out = append(out, Finding{
			RuleID: ruleID, Severity: rule.Severity,
			Pole: c.PoleID(), Subject: att.ID, Against: above.ID,
			Owner: att.Owner, Blame: BlameUnattributed,
			Measured: measured, Required: required,
			Detail: fmt.Sprintf("%s under %s: %s", att.ID, above.ID,
				nesc.DescribeDeficit(measured, required)),
		})
	}
	return out
}

// isEquipment reports whether a facility is a box rather than a wire.
func isEquipment(k plant.Kind) bool {
	switch k {
	case plant.KindEquipmentBox, plant.KindPowerSupply:
		return true
	}
	return false
}

// VerticalRunRule checks the clearance a riser keeps from other facilities on
// the pole face. A riser runs the length of the pole, so it is checked against
// everything rather than against its nearest neighbour.
type VerticalRunRule struct{}

// ID returns the catalogue rule id.
func (VerticalRunRule) ID() string { return "NESC-239E" }

// Check returns a finding for every facility a riser passes too closely.
func (r VerticalRunRule) Check(c PoleContext) Findings {
	required, ruleID := nesc.VerticalRunClearance()
	rule := nesc.MustRule(ruleID)
	a := c.Assembly()
	var out Findings
	for _, riser := range a.Attachments {
		if riser.Kind != plant.KindRiser {
			continue
		}
		if riser.Face == plant.FaceUnknown {
			// Without a recorded face there is nothing to measure against.
			continue
		}
		for _, other := range a.Attachments {
			if other.ID == riser.ID || other.Face != riser.Face {
				continue
			}
			measured := other.Height.Sub(riser.Height).Abs()
			if nesc.Meets(measured, required) {
				continue
			}
			out = append(out, Finding{
				RuleID: ruleID, Severity: rule.Severity,
				Pole: c.PoleID(), Subject: riser.ID, Against: other.ID,
				Owner: riser.Owner, Blame: BlameUnattributed,
				Measured: measured, Required: required,
				Detail: fmt.Sprintf("%s passes %s on the %s: %s", riser.ID, other.ID,
					riser.Face.Name(), nesc.DescribeDeficit(measured, required)),
			})
		}
	}
	return out
}
