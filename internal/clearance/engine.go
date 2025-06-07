package clearance

import (
	"errors"
	"fmt"
	"sort"

	"github.com/glinskevxer/downguy/internal/nesc"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/survey"
)

// ErrBadContext is returned when the engine is handed something it cannot
// check.
var ErrBadContext = errors.New("clearance: cannot check")

// PoleContext is everything a pole rule needs. It carries the surveyed
// assembly rather than the record, and it names the applicant so that a rule
// can tell the applicant's own facility from everybody else's.
type PoleContext struct {
	Applied   *survey.Applied
	District  nesc.District
	Grade     nesc.Grade
	Applicant string
}

// Assembly returns the surveyed assembly.
func (c PoleContext) Assembly() *plant.Assembly { return c.Applied.Assembly }

// PoleID returns the pole being checked.
func (c PoleContext) PoleID() string { return string(c.Applied.Assembly.Pole.ID) }

// IsApplicant reports whether a facility belongs to the attacher applying.
func (c PoleContext) IsApplicant(att plant.Attachment) bool {
	return c.Applicant != "" && att.Owner == c.Applicant
}

// Validate checks the context is complete enough to run rules against.
func (c PoleContext) Validate() error {
	if c.Applied == nil || c.Applied.Assembly == nil {
		return fmt.Errorf("%w: no surveyed assembly", ErrBadContext)
	}
	if _, err := nesc.LoadingFor(c.District); err != nil {
		return err
	}
	if _, err := nesc.FactorsFor(c.Grade); err != nil {
		return err
	}
	return c.Applied.Assembly.Validate()
}

// PoleRule is one checkable requirement against a single structure.
type PoleRule interface {
	// ID returns the catalogue rule the finding will cite.
	ID() string
	// Check returns every way the rule fails on this pole. A rule that does
	// not apply returns nothing rather than an error.
	Check(PoleContext) Findings
}

// Engine holds the registered rules and runs them.
type Engine struct {
	poleRules []PoleRule
	spanRules []SpanRule
}

// New builds an engine with the standard rule set registered.
func New() *Engine {
	e := &Engine{}
	e.RegisterPole(SafetyZoneRule{})
	e.RegisterPole(CommSeparationRule{})
	e.RegisterPole(ClimbingSpaceRule{})
	e.RegisterPole(EquipmentRule{})
	e.RegisterPole(VerticalRunRule{})
	e.RegisterSpan(GroundClearanceRule{})
	e.RegisterSpan(MidspanRule{})
	return e
}

// Empty builds an engine with nothing registered, for a caller that wants to
// choose its own rules.
func Empty() *Engine { return &Engine{} }

// RegisterPole adds a pole rule.
func (e *Engine) RegisterPole(r PoleRule) { e.poleRules = append(e.poleRules, r) }

// PoleRuleIDs returns the ids of the registered pole rules, sorted.
func (e *Engine) PoleRuleIDs() []string {
	out := make([]string, 0, len(e.poleRules))
	for _, r := range e.poleRules {
		out = append(out, r.ID())
	}
	sort.Strings(out)
	return out
}

// CheckPole runs every registered pole rule and returns the findings, worst
// first.
func (e *Engine) CheckPole(c PoleContext) (Findings, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	var out Findings
	for _, r := range e.poleRules {
		out = append(out, r.Check(c)...)
	}
	out.Sort()
	return out, nil
}

// SafetyZoneRule checks the gap between the lowest supply facility and the top
// of communication space. It is the rule that stops work when it fails,
// because a communication worker climbing into a forty inch zone that is
// twenty eight inches is the accident the whole code section exists to stop.
type SafetyZoneRule struct{}

// ID returns the catalogue rule id.
func (SafetyZoneRule) ID() string { return "NESC-235C1" }

// Check runs the rule.
func (r SafetyZoneRule) Check(c PoleContext) Findings {
	a := c.Assembly()
	supply, ok := a.LowestSupply()
	if !ok {
		return nil
	}
	comm, ok := a.HighestCommunication()
	if !ok {
		return nil
	}
	neutral, hasNeutral := a.Neutral()
	reduced := hasNeutral && neutral.ID == supply.ID
	required, ruleID := nesc.SafetyZone(reduced)
	measured := supply.Height.Sub(comm.Height)
	if nesc.Meets(measured, required) {
		return nil
	}
	rule := nesc.MustRule(ruleID)
	return Findings{{
		RuleID: ruleID, Severity: rule.Severity,
		Pole: c.PoleID(), Subject: comm.ID, Against: supply.ID,
		Owner: comm.Owner, Blame: BlamePreExisting,
		Measured: measured, Required: required,
		Detail: fmt.Sprintf("%s under %s: %s", comm.ID, supply.ID,
			nesc.DescribeDeficit(measured, required)),
	}}
}
