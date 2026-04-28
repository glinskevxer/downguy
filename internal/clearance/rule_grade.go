package clearance

import (
	"fmt"

	"github.com/glinskevxer/downguy/internal/nesc"
	"github.com/glinskevxer/downguy/internal/plant"
)

// GradeRule checks that a span is engineered to the grade of construction its
// crossings actually require. The grade is not a setting somebody picks: a
// supply conductor over a railroad is grade B whatever the rest of the line is
// built to, and a joint use pole carries supply over communication, which is
// grade B on its own.
//
// It fires against the grade the check was run at, so a run at grade C over a
// railroad reports that the whole analysis was done to the wrong standard,
// which is a more useful finding than any single clearance underneath it.
type GradeRule struct {
	// Grade is what the run was carried out at. It is a field rather than a
	// read off the context because the span context does not carry one: the
	// grade belongs to the structure, and this rule is the thing that ties the
	// two together.
	Grade nesc.Grade
}

// ID returns the catalogue rule id.
func (GradeRule) ID() string { return "NESC-242C1" }

// Check compares the grade the run used against what the span demands.
func (r GradeRule) Check(c SpanContext) Findings {
	if !nesc.KnownGradeForCheck(r.Grade) {
		return nil
	}
	crossesRail := false
	for _, station := range c.Sheet.Ordered() {
		if station.Cover == plant.CoverRailroad {
			crossesRail = true
		}
	}
	hasSupply, hasComm := false, false
	for _, station := range c.Sheet.Ordered() {
		for _, m := range station.Measurements {
			att, ok := c.Facility(m.Subject)
			if !ok {
				continue
			}
			if att.Kind.IsSupply() {
				hasSupply = true
			}
			if att.Kind.IsCommunication() {
				hasComm = true
			}
		}
	}
	if !hasSupply && !crossesRail {
		return nil
	}

	required, ruleID := nesc.RequiredGrade(crossesRail, hasComm, hasSupply)
	if !required.Stricter(r.Grade) {
		return nil
	}
	rule := nesc.MustRule(gradeRuleFor(ruleID))
	return Findings{{
		RuleID: rule.ID, Severity: rule.Severity,
		Pole: string(c.Sheet.FromPole), Span: c.Sheet.ID,
		Subject: c.Sheet.ID, Against: reasonFor(crossesRail, hasComm),
		Blame: BlameUnattributed,
		Detail: fmt.Sprintf("the span %s and needs grade %s, and this check was run at grade %s",
			reasonFor(crossesRail, hasComm), required, r.Grade),
	}}
}

// gradeRuleFor maps the citation the grade table returns onto the catalogue.
// The table cites the paragraph; the catalogue carries one entry for the
// section, because a finding that cites a paragraph nobody can look up is
// worse than one that cites the section.
func gradeRuleFor(cite string) string {
	switch cite {
	case "NESC-242C1", "NESC-242C2", "NESC-242D", "NESC-242B", "NESC-242A":
		return "NESC-242"
	}
	return "NESC-242"
}

func reasonFor(crossesRail, hasComm bool) string {
	switch {
	case crossesRail:
		return "crosses a railroad"
	case hasComm:
		return "carries supply over communication"
	default:
		return "carries supply"
	}
}
