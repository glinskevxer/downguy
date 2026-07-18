package report

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/glinskevxer/downguy/internal/clearance"
	"github.com/glinskevxer/downguy/internal/loading"
	"github.com/glinskevxer/downguy/internal/makeready"
	"github.com/glinskevxer/downguy/internal/unit"
)

// The json shapes are declared here rather than by tagging the domain types,
// because the domain types are free to change and the wire format is not.
// Everything that leaves this package as json goes through a struct in this
// file, so a rename inside the engine cannot silently break a consumer.

// FindingJSON is one clearance finding on the wire.
type FindingJSON struct {
	Rule     string `json:"rule"`
	Severity string `json:"severity"`
	Pole     string `json:"pole,omitempty"`
	Span     string `json:"span,omitempty"`
	Subject  string `json:"subject"`
	Against  string `json:"against,omitempty"`
	Owner    string `json:"owner,omitempty"`
	Blame    string `json:"blame"`
	Measured string `json:"measured_ft"`
	Required string `json:"required_ft"`
	Deficit  string `json:"deficit_ft"`
	Detail   string `json:"detail,omitempty"`
}

// FindingsJSON turns a finding set into its wire shape.
func FindingsJSON(findings clearance.Findings) []FindingJSON {
	out := make([]FindingJSON, 0, len(findings))
	for _, f := range findings {
		out = append(out, FindingJSON{
			Rule: f.RuleID, Severity: string(f.Severity),
			Pole: f.Pole, Span: f.Span, Subject: f.Subject, Against: f.Against,
			Owner: f.Owner, Blame: string(f.Blame),
			Measured: unit.Decimal(f.Measured), Required: unit.Decimal(f.Required),
			Deficit: unit.Decimal(f.Deficit()), Detail: f.Detail,
		})
	}
	return out
}

// WorkItemJSON is one make ready item on the wire.
type WorkItemJSON struct {
	ID      string   `json:"id"`
	Action  string   `json:"action"`
	Pole    string   `json:"pole"`
	Span    string   `json:"span,omitempty"`
	Subject string   `json:"subject,omitempty"`
	Owner   string   `json:"owner,omitempty"`
	From    string   `json:"from_ft,omitempty"`
	To      string   `json:"to_ft,omitempty"`
	Because []string `json:"because,omitempty"`
	Detail  string   `json:"detail,omitempty"`
}

// WorkItemToJSON turns one work item into its wire shape. It is exported
// because a plan and a crew packet describe the same item, and two renderings
// of one item is how a consumer ends up parsing two shapes.
func WorkItemToJSON(item makeready.Item) WorkItemJSON {
	out := WorkItemJSON{
		ID: item.ID, Action: string(item.Action), Pole: item.Pole, Span: item.Span,
		Subject: item.Subject, Owner: item.Owner, Because: item.Because, Detail: item.Detail,
	}
	if item.FromHeight != 0 || item.ToHeight != 0 {
		out.From = unit.Decimal(item.FromHeight)
		out.To = unit.Decimal(item.ToHeight)
	}
	return out
}

// PlanJSON is a make ready plan on the wire.
//
// Findings carries the attributed findings the plan was worked out from. It is
// on the document rather than left to a second command because the cost
// allocation keys on them: without it a caller has to run the clearance check
// again and hope it attributes the same way, and the unattributed findings a
// plain check writes cannot be billed at all.
type PlanJSON struct {
	Application string         `json:"application"`
	Items       []WorkItemJSON `json:"items"`
	Findings    []FindingJSON  `json:"findings,omitempty"`
	Unresolved  []FindingJSON  `json:"unresolved,omitempty"`
}

// PlanWithFindings turns a plan into its wire shape, carrying the attributed
// findings it was worked out from.
func PlanWithFindings(plan *makeready.Plan, findings clearance.Findings) PlanJSON {
	out := PlanToJSON(plan)
	out.Findings = FindingsJSON(findings)
	if len(out.Findings) == 0 {
		out.Findings = nil
	}
	return out
}

// PlanToJSON turns a plan into its wire shape.
func PlanToJSON(plan *makeready.Plan) PlanJSON {
	if plan == nil {
		return PlanJSON{Items: []WorkItemJSON{}}
	}
	out := PlanJSON{Application: plan.Application, Items: make([]WorkItemJSON, 0, len(plan.Items))}
	for _, item := range plan.Items {
		out.Items = append(out.Items, WorkItemToJSON(item))
	}
	out.Unresolved = FindingsJSON(plan.Unresolved)
	if len(out.Unresolved) == 0 {
		out.Unresolved = nil
	}
	return out
}

// ComponentJSON is one contribution to a groundline moment.
type ComponentJSON struct {
	Source   string  `json:"source"`
	Kind     string  `json:"kind"`
	Moment   float64 `json:"moment_ft_lb"`
	Factored float64 `json:"factored_ft_lb"`
}

// AnalysisJSON is a loading check on the wire.
type AnalysisJSON struct {
	Pole        string          `json:"pole"`
	District    string          `json:"district"`
	Grade       string          `json:"grade"`
	Components  []ComponentJSON `json:"components"`
	Factored    float64         `json:"factored_moment_ft_lb"`
	Capacity    float64         `json:"factored_capacity_ft_lb"`
	Utilisation int             `json:"utilisation_percent"`
	Passes      bool            `json:"passes"`
}

// AnalysisToJSON turns a loading check into its wire shape.
func AnalysisToJSON(a *loading.Analysis) AnalysisJSON {
	if a == nil {
		return AnalysisJSON{Components: []ComponentJSON{}}
	}
	out := AnalysisJSON{
		Pole: string(a.Pole.ID), District: string(a.District), Grade: string(a.Grade),
		Components: make([]ComponentJSON, 0, len(a.Components)),
		Factored:   unit.RoundToWhole(a.FactoredMoment), Capacity: unit.RoundToWhole(a.FactoredCapacty),
		Utilisation: a.UtilisationPercent(), Passes: a.Passes(),
	}
	for _, c := range a.Components {
		out.Components = append(out.Components, ComponentJSON{
			Source: c.Source, Kind: c.Kind,
			Moment: unit.RoundToWhole(c.Moment), Factored: unit.RoundToWhole(c.Factored),
		})
	}
	return out
}

// WriteJSON writes any value as indented json with a trailing newline, which
// is what a command line tool piping into another one wants.
func WriteJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("report: writing json: %w", err)
	}
	return nil
}
