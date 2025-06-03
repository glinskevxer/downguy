package clearance

import (
	"fmt"

	"github.com/glinskevxer/downguy/internal/nesc"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/unit"
)

// ClimbingSpaceRule checks that nothing is bolted into the square a worker
// climbs through. A cable running across the face is not the problem; a power
// supply, an amplifier housing or a coiled slack loop hung on the climbing
// face is, because a climber cannot get past it without unhooking.
type ClimbingSpaceRule struct{}

// ID returns the catalogue rule id.
func (ClimbingSpaceRule) ID() string { return "NESC-236A" }

// ClimbingSpaceFloor is how far up the pole the clear square has to start. A
// worker gets onto the pole below this, so an obstruction at four feet is a
// nuisance rather than a code violation.
const ClimbingSpaceFloor = unit.Height(80)

// Check returns a finding for every facility mounted on the climbing face
// above the floor.
func (r ClimbingSpaceRule) Check(c PoleContext) Findings {
	required, ruleID := nesc.ClimbingSpace()
	rule := nesc.MustRule(ruleID)
	var out Findings
	for _, att := range c.Assembly().Attachments {
		if !att.Face.Obstructs() {
			continue
		}
		if att.Height < ClimbingSpaceFloor {
			continue
		}
		if att.Kind.IsSpanCarrying() {
			// A wire crossing the climbing face is passed rather than climbed
			// around, and the code treats it that way too.
			continue
		}
		blame := BlamePreExisting
		if c.IsApplicant(att) {
			blame = BlameApplicant
		}
		out = append(out, Finding{
			RuleID: ruleID, Severity: rule.Severity,
			Pole: c.PoleID(), Subject: att.ID,
			Owner: att.Owner, Blame: blame,
			Measured: 0, Required: required,
			Detail: fmt.Sprintf("%s is a %s on the %s at %s, inside the %s climbing square",
				att.ID, att.Kind, att.Face.Name(), att.Height, required),
		})
	}
	return out
}

// ObstructionsOn returns the facilities on one face of a pole, from the top
// down. It is here rather than on the assembly because the face is a
// clearance concept and the plant record does not care about it.
func ObstructionsOn(a *plant.Assembly, f plant.Face) []plant.Attachment {
	return a.Where(func(att plant.Attachment) bool {
		return att.Face == f && !att.Kind.IsSpanCarrying()
	})
}
