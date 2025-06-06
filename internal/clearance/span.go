package clearance

import (
	"fmt"
	"sort"

	"github.com/glinskevxer/downguy/internal/nesc"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/survey"
)

// SpanContext is everything a span rule needs. It carries the span sheet the
// crew filled in, and the attachments at the near pole so that a reading of
// "A3" in the span can be resolved to a facility with an owner and a kind.
type SpanContext struct {
	Sheet     survey.SpanSheet
	Near      *plant.Assembly
	District  nesc.District
	Applicant string
}

// Validate checks the context is complete enough to run rules against.
func (c SpanContext) Validate() error {
	if err := c.Sheet.Validate(); err != nil {
		return err
	}
	if c.Near == nil {
		return fmt.Errorf("%w: no near pole assembly", ErrBadContext)
	}
	if c.Sheet.FromPole != c.Near.Pole.ID {
		return fmt.Errorf("%w: sheet %s starts at %s, assembly is %s",
			ErrBadContext, c.Sheet.ID, c.Sheet.FromPole, c.Near.Pole.ID)
	}
	if _, err := nesc.LoadingFor(c.District); err != nil {
		return err
	}
	return nil
}

// Facility resolves a survey subject to the attachment it names.
func (c SpanContext) Facility(subject string) (plant.Attachment, bool) {
	if c.Near == nil {
		return plant.Attachment{}, false
	}
	return c.Near.Find(subject)
}

// IsApplicant reports whether a facility belongs to the attacher applying.
func (c SpanContext) IsApplicant(att plant.Attachment) bool {
	return c.Applicant != "" && SameAttacher(att.Owner, c.Applicant)
}

// SpanRule is one checkable requirement against a span.
type SpanRule interface {
	ID() string
	Check(SpanContext) Findings
}

// RegisterSpan adds a span rule to the engine.
func (e *Engine) RegisterSpan(r SpanRule) { e.spanRules = append(e.spanRules, r) }

// SpanRuleIDs returns the ids of the registered span rules, sorted.
func (e *Engine) SpanRuleIDs() []string {
	out := make([]string, 0, len(e.spanRules))
	for _, r := range e.spanRules {
		out = append(out, r.ID())
	}
	sort.Strings(out)
	return out
}

// CheckSpan runs every registered span rule and returns the findings, worst
// first.
func (e *Engine) CheckSpan(c SpanContext) (Findings, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	var out Findings
	for _, r := range e.spanRules {
		out = append(out, r.Check(c)...)
	}
	out.Sort()
	return out, nil
}

// GroundClearanceRule checks every reading in the span against the clearance
// the ground cover at that station demands. It is the rule that fires most
// often on a real system, because a cable strung on a warm afternoon in July
// is nowhere near where it was measured in March.
type GroundClearanceRule struct{}

// ID returns the catalogue rule id.
func (GroundClearanceRule) ID() string { return "NESC-232A" }

// Check walks the stations from the near pole outward.
func (r GroundClearanceRule) Check(c SpanContext) Findings {
	var out Findings
	for _, station := range c.Sheet.Ordered() {
		for _, m := range station.Measurements {
			att, ok := c.Facility(m.Subject)
			if !ok {
				continue
			}
			circuit, err := nesc.CircuitOf(att.Kind, att.Voltage)
			if err != nil {
				continue
			}
			required, ruleID, err := nesc.GroundClearance(station.Cover, circuit)
			if err != nil {
				continue
			}
			if m.Clears(required) != survey.VerdictShort {
				continue
			}
			rule := nesc.MustRule(ruleID)
			blame := BlamePreExisting
			if c.IsApplicant(att) {
				blame = BlameApplicant
			}
			out = append(out, Finding{
				RuleID: ruleID, Severity: rule.Severity,
				Pole: string(c.Sheet.FromPole), Span: c.Sheet.ID,
				Subject: m.Subject, Against: string(station.Cover),
				Owner: att.Owner, Blame: blame,
				Measured: m.Height, Required: required,
				Detail: fmt.Sprintf("%s at %d feet over %s: %s", m.Subject, station.Feet,
					station.Cover.Name(), nesc.DescribeDeficit(m.Height, required)),
			})
		}
	}
	return out
}
