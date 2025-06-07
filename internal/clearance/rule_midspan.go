package clearance

import (
	"fmt"
	"sort"

	"github.com/glinskevxer/downguy/internal/nesc"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/survey"
	"github.com/glinskevxer/downguy/internal/unit"
)

// MidspanRule checks the separation between facilities out in the span rather
// than at the pole. Two cables strung a foot apart at the poles can be touching
// at mid span if one of them sags twice as far as the other, and that is not
// visible from a pole survey at all.
type MidspanRule struct{}

// ID returns the catalogue rule id.
func (MidspanRule) ID() string { return "NESC-235E" }

// Check compares every neighbouring pair of facilities at every station.
func (r MidspanRule) Check(c SpanContext) Findings {
	var out Findings
	for _, station := range c.Sheet.Ordered() {
		out = append(out, r.checkStation(c, station)...)
	}
	return out
}

func (r MidspanRule) checkStation(c SpanContext, station survey.Station) Findings {
	type reading struct {
		m   survey.Measurement
		att plant.Attachment
	}
	var known []reading
	for _, m := range station.Measurements {
		att, ok := c.Facility(m.Subject)
		if !ok {
			continue
		}
		known = append(known, reading{m: m, att: att})
	}
	if len(known) < 2 {
		return nil
	}
	sort.SliceStable(known, func(i, j int) bool {
		if known[i].m.Height != known[j].m.Height {
			return known[i].m.Height > known[j].m.Height
		}
		return known[i].m.Subject < known[j].m.Subject
	})

	var out Findings
	for i := 0; i+1 < len(known); i++ {
		upper, lower := known[i], known[i+1]
		if upper.att.Kind.IsCommunication() && lower.att.Kind.IsCommunication() &&
			SameAttacher(upper.att.Owner, lower.att.Owner) {
			// One attacher's own two cables are its own business out in the
			// span, exactly as they are on the pole.
			continue
		}
		required, ruleID := nesc.MidspanSeparation(upper.att.Kind.IsSupply())
		measured := upper.m.Height.Sub(lower.m.Height)
		if nesc.Meets(measured, required) {
			continue
		}
		rule := nesc.MustRule(ruleID)
		blame := BlamePreExisting
		owner := lower.att.Owner
		if c.IsApplicant(lower.att) {
			blame = BlameApplicant
		} else if c.IsApplicant(upper.att) {
			blame = BlameApplicant
			owner = upper.att.Owner
		}
		out = append(out, Finding{
			RuleID: ruleID, Severity: rule.Severity,
			Pole: string(c.Sheet.FromPole), Span: c.Sheet.ID,
			Subject: lower.m.Subject, Against: upper.m.Subject,
			Owner: owner, Blame: blame,
			Measured: measured, Required: required,
			Detail: fmt.Sprintf("%s under %s at %d feet: %s", lower.m.Subject,
				upper.m.Subject, station.Feet, nesc.DescribeDeficit(measured, required)),
		})
	}
	return out
}

// TightestPair returns the smallest gap between any two facilities anywhere in
// the span, and the station it happens at. It is the number a designer wants
// before a rule fires, because a span that clears by a tenth is a span that
// will not clear next summer.
func TightestPair(c SpanContext) (unit.Height, int, bool) {
	best := unit.Height(0)
	at := 0
	found := false
	for _, station := range c.Sheet.Ordered() {
		heights := make([]unit.Height, 0, len(station.Measurements))
		for _, m := range station.Measurements {
			if _, ok := c.Facility(m.Subject); ok {
				heights = append(heights, m.Height)
			}
		}
		if len(heights) < 2 {
			continue
		}
		sort.Slice(heights, func(i, j int) bool { return heights[i] > heights[j] })
		for i := 0; i+1 < len(heights); i++ {
			gap := heights[i].Sub(heights[i+1])
			if !found || gap < best {
				best, at, found = gap, station.Feet, true
			}
		}
	}
	return best, at, found
}
