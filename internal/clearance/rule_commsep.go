package clearance

import (
	"fmt"

	"github.com/glinskevxer/downguy/internal/nesc"
	"github.com/glinskevxer/downguy/internal/plant"
)

// CommSeparationRule checks the gap between neighbouring facilities in
// communication space. Two attachers need room to work without bringing the
// other one down; one attacher with two cables of its own needs much less,
// because the same crew owns both.
type CommSeparationRule struct{}

// ID returns the catalogue rule id.
func (CommSeparationRule) ID() string { return "NESC-235C2" }

// Check walks communication space from the top down and compares each
// neighbouring pair. Only adjacent facilities are compared: a cable is not in
// violation of one three feet below it just because something else sits
// between them.
func (r CommSeparationRule) Check(c PoleContext) Findings {
	comms := c.Assembly().Communication()
	if len(comms) < 2 {
		return nil
	}
	var out Findings
	for i := 0; i+1 < len(comms); i++ {
		upper, lower := comms[i], comms[i+1]
		same := sameOwner(upper, lower)
		required, ruleID := nesc.CommunicationSeparation(same)
		measured := upper.Height.Sub(lower.Height)
		if nesc.Meets(measured, required) {
			continue
		}
		rule := nesc.MustRule(ruleID)
		// The lower facility is the one that has to move, so it is named as
		// the owner unless the applicant is the one above.
		owner := lower.Owner
		if !c.IsApplicant(lower) && c.IsApplicant(upper) {
			owner = upper.Owner
		}
		out = append(out, Finding{
			RuleID: ruleID, Severity: rule.Severity,
			Pole: c.PoleID(), Subject: lower.ID, Against: upper.ID,
			Owner: owner, Blame: BlameUnattributed,
			Measured: measured, Required: required,
			Detail: fmt.Sprintf("%s under %s: %s", lower.ID, upper.ID,
				nesc.DescribeDeficit(measured, required)),
		})
	}
	return out
}

// sameOwner reports whether two facilities belong to the same attacher.
func sameOwner(a, b plant.Attachment) bool {
	return normaliseOwner(a.Owner) != "" && normaliseOwner(a.Owner) == normaliseOwner(b.Owner)
}
