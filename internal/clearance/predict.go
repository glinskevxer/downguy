package clearance

import (
	"fmt"
	"math"

	"github.com/glinskevxer/downguy/internal/nesc"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/sagten"
	"github.com/glinskevxer/downguy/internal/unit"
)

// Proposal is an attachment that does not exist yet. There is nothing to
// measure, so its clearance has to be worked out: strung at the design
// tension, sagged to its worst condition, and checked against the ground it
// crosses.
type Proposal struct {
	AttachmentID string
	Owner        string
	Kind         plant.Kind
	Conductor    string
	NearHeight   unit.Height
	FarHeight    unit.Height
	SpanFeet     float64
	RulingSpan   float64
	GroundDrop   unit.Height
	Cover        plant.Cover
	District     nesc.District
}

// Validate checks a proposal carries enough to be worked out.
func (p Proposal) Validate() error {
	switch {
	case p.AttachmentID == "":
		return fmt.Errorf("%w: proposal has no attachment id", ErrBadContext)
	case p.Owner == "":
		return fmt.Errorf("%w: %s has no owner", ErrBadContext, p.AttachmentID)
	case !plant.KnownKind(p.Kind):
		return fmt.Errorf("%w: %s is a %q", ErrBadContext, p.AttachmentID, p.Kind)
	case !p.Kind.IsSpanCarrying():
		return fmt.Errorf("%w: %s does not run into the span", ErrBadContext, p.AttachmentID)
	case p.NearHeight <= 0 || p.FarHeight <= 0:
		return fmt.Errorf("%w: %s is proposed at %s and %s", ErrBadContext, p.AttachmentID, p.NearHeight, p.FarHeight)
	case p.SpanFeet <= 0:
		return fmt.Errorf("%w: %s crosses %v feet", ErrBadContext, p.AttachmentID, p.SpanFeet)
	case p.RulingSpan <= 0:
		return fmt.Errorf("%w: %s has no ruling span", ErrBadContext, p.AttachmentID)
	case !plant.KnownCover(p.Cover):
		return fmt.Errorf("%w: %s crosses %q", ErrBadContext, p.AttachmentID, p.Cover)
	}
	if _, err := sagten.Lookup(p.Conductor); err != nil {
		return err
	}
	if _, err := nesc.LoadingFor(p.District); err != nil {
		return err
	}
	return nil
}

// Prediction is where a proposed facility will actually be.
type Prediction struct {
	Proposal   Proposal
	SagFeet    float64
	LowPointAt float64
	Clearance  unit.Height
	Required   unit.Height
	RuleID     string
	Clears     bool
}

// Predict works out the worst case sag of a proposed facility, the clearance
// that leaves over the ground, and whether that satisfies the code.
func Predict(p Proposal) (*Prediction, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	c, err := sagten.Lookup(p.Conductor)
	if err != nil {
		return nil, err
	}
	start, err := sagten.InitialCondition(c, p.RulingSpan, p.District)
	if err != nil {
		return nil, err
	}
	hot, _, err := sagten.SagAt(c, p.RulingSpan, p.SpanFeet, start, sagten.BareLoad(c), unit.TempMaxOperating)
	if err != nil {
		return nil, err
	}
	load, temp, err := sagten.DistrictCondition(c, p.District)
	if err != nil {
		return nil, err
	}
	iced, _, err := sagten.SagAt(c, p.RulingSpan, p.SpanFeet, start, load, temp)
	if err != nil {
		return nil, err
	}
	sag := math.Max(hot, iced)

	profile, err := sagten.NewProfile(p.SpanFeet, sag, p.NearHeight, p.FarHeight, p.GroundDrop)
	if err != nil {
		return nil, err
	}
	clearance, at := profile.MinimumClearance()

	circuit, err := nesc.CircuitOf(p.Kind, 0)
	if err != nil {
		return nil, err
	}
	required, ruleID, err := nesc.GroundClearance(p.Cover, circuit)
	if err != nil {
		return nil, err
	}
	return &Prediction{
		Proposal: p, SagFeet: sag, LowPointAt: at,
		Clearance: clearance, Required: required, RuleID: ruleID,
		Clears: nesc.Meets(clearance, required),
	}, nil
}

// Finding turns a failed prediction into a finding, and reports whether there
// was one to turn.
func (p *Prediction) Finding(span string) (Finding, bool) {
	if p.Clears {
		return Finding{}, false
	}
	rule := nesc.MustRule(p.RuleID)
	return Finding{
		RuleID: p.RuleID, Severity: rule.Severity,
		Span: span, Subject: p.Proposal.AttachmentID, Against: string(p.Proposal.Cover),
		Owner: p.Proposal.Owner, Blame: BlameApplicant,
		Measured: p.Clearance, Required: p.Required,
		Detail: fmt.Sprintf("%s is predicted to sag %.2f feet and clear %s over %s: %s",
			p.Proposal.AttachmentID, p.SagFeet, p.Clearance, p.Proposal.Cover.Name(),
			nesc.DescribeDeficit(p.Clearance, p.Required)),
	}, true
}

// LowestUsableHeight returns the attachment height the proposal would need at
// both ends to clear its ground cover, working the sag backwards. It is what
// the make ready planner asks for when a proposal fails: not "it is short by
// eight tenths" but "put it at 19.4".
func LowestUsableHeight(p Proposal) (unit.Height, error) {
	pred, err := Predict(p)
	if err != nil {
		return 0, err
	}
	lowest := p.NearHeight
	if p.FarHeight < lowest {
		lowest = p.FarHeight
	}
	if pred.Clears {
		return lowest, nil
	}
	return lowest.Add(pred.Required.Sub(pred.Clearance)), nil
}
