package cliapp

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/glinskevxer/downguy/internal/costing"
	"github.com/glinskevxer/downguy/internal/makeready"
	"github.com/glinskevxer/downguy/internal/report"
	"github.com/glinskevxer/downguy/internal/unit"
)

func init() {
	Register(Command{
		Name:    "estimate",
		Summary: "price a make ready plan against the unit cost book",
		Usage:   estimateUsage(),
		Run:     runEstimate,
	})
	Register(Command{
		Name:    "costbook",
		Summary: "print the unit cost book as it stood on a day",
		Usage:   costbookUsage(),
		Run:     runCostbook,
	})
}

func estimateUsage() string {
	f := newEstimateFlags(nil)
	return f.Usage("estimate [options]", "estimate [options]",
		"Reads a make ready plan as json on standard input, which is what\n"+
			"downguy makeready -json writes, and prices it against the unit cost book\n"+
			"as the book stood on the pricing date.\n\n"+
			"With -quote the contingency and overhead are rolled on and the result is a\n"+
			"quotation with a date it stops being honoured.")
}

func newEstimateFlags(out io.Writer) *Flags {
	f := NewFlags("estimate", out).WithJSON()
	f.Set().String("on", "", "the date to price against, as yyyy-mm-dd")
	f.Set().Bool("quote", false, "roll the standard adjustments on and issue a quotation")
	return f
}

// planInput is the json a plan arrives as. It is read into the domain type
// rather than used directly, so that a hand written file with a missing field
// fails here rather than three packages later.
type planInput struct {
	Application string `json:"application"`
	Items       []struct {
		ID      string   `json:"id"`
		Action  string   `json:"action"`
		Pole    string   `json:"pole"`
		Span    string   `json:"span"`
		Subject string   `json:"subject"`
		Owner   string   `json:"owner"`
		From    string   `json:"from_ft"`
		To      string   `json:"to_ft"`
		Because []string `json:"because"`
		Detail  string   `json:"detail"`
	} `json:"items"`
}

// ReadPlan reads a make ready plan from json.
func ReadPlan(r io.Reader) (*makeready.Plan, error) {
	var input planInput
	if err := json.NewDecoder(r).Decode(&input); err != nil {
		return nil, fmt.Errorf("reading the plan: %w", err)
	}
	plan := &makeready.Plan{Application: input.Application}
	for _, item := range input.Items {
		action, err := makeready.ParseAction(item.Action)
		if err != nil {
			return nil, fmt.Errorf("item %s: %w", item.ID, err)
		}
		work := makeready.Item{
			ID: item.ID, Action: action, Pole: item.Pole, Span: item.Span,
			Subject: item.Subject, Owner: item.Owner,
			Because: item.Because, Detail: item.Detail,
		}
		if item.From != "" {
			height, err := unit.ParseHeight(item.From)
			if err != nil {
				return nil, fmt.Errorf("item %s: from_ft: %w", item.ID, err)
			}
			work.FromHeight = height
		}
		if item.To != "" {
			height, err := unit.ParseHeight(item.To)
			if err != nil {
				return nil, fmt.Errorf("item %s: to_ft: %w", item.ID, err)
			}
			work.ToHeight = height
		}
		if err := work.Validate(); err != nil {
			return nil, err
		}
		plan.Items = append(plan.Items, work)
	}
	plan.Items = makeready.Order(plan.Items)
	return plan, nil
}

// EstimateJSON is a priced plan on the wire.
type EstimateJSON struct {
	Application string     `json:"application"`
	PricedOn    string     `json:"priced_on"`
	Lines       []LineJSON `json:"lines"`
	Unpriced    []string   `json:"unpriced,omitempty"`
	Direct      string     `json:"direct_total"`
	Adjustments []AdjJSON  `json:"adjustments,omitempty"`
	Total       string     `json:"total"`
	GoodUntil   string     `json:"good_until,omitempty"`
}

// LineJSON is one priced line on the wire.
type LineJSON struct {
	Item        string `json:"item"`
	Code        string `json:"code"`
	Description string `json:"description"`
	Quantity    int    `json:"quantity"`
	Scale       int    `json:"scale"`
	UnitPrice   string `json:"unit_price"`
	Amount      string `json:"amount"`
	Owner       string `json:"owner,omitempty"`
}

// AdjJSON is one rolled up adjustment on the wire.
type AdjJSON struct {
	Name   string `json:"name"`
	Rate   string `json:"rate"`
	On     string `json:"applied_to"`
	Amount string `json:"amount"`
}

func runEstimate(env *Env) int {
	f := newEstimateFlags(env.Stderr)
	if err := f.Parse(env.Args); err != nil {
		return CodeUsage
	}
	if len(f.Rest()) > 0 {
		return env.Fail(CodeUsage, "estimate reads its plan from standard input and takes no arguments")
	}
	on, err := requiredDateFlag(f, "on")
	if err != nil {
		return env.Fail(CodeUsage, "%v", err)
	}
	plan, err := ReadPlan(env.Stdin)
	if err != nil {
		return env.Fail(CodeUsage, "%v", err)
	}
	book := costing.Standard()
	estimate, err := costing.Price(plan, book, on)
	if err != nil {
		return env.Fail(CodeUsage, "%v", err)
	}

	wire := EstimateJSON{
		Application: estimate.Application, PricedOn: estimate.Priced.String(),
		Lines:    make([]LineJSON, 0, len(estimate.Lines)),
		Unpriced: estimate.Unpriced,
		Direct:   estimate.Total.String(), Total: estimate.Total.String(),
	}
	for _, l := range estimate.Lines {
		wire.Lines = append(wire.Lines, LineJSON{
			Item: l.ItemID, Code: string(l.Code), Description: l.Description,
			Quantity: l.Quantity, Scale: l.Scale,
			UnitPrice: l.UnitPrice.String(), Amount: l.Amount.String(), Owner: l.Owner,
		})
	}
	if flagBool(f, "quote") {
		quotation, err := costing.Quote(estimate, costing.StandardAdjustments(), on)
		if err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
		for _, a := range quotation.Adjustments {
			wire.Adjustments = append(wire.Adjustments, AdjJSON{
				Name: a.Name, Rate: a.Rate.String(),
				On: a.On.String(), Amount: a.Amount.String(),
			})
		}
		wire.Total = quotation.Total.String()
		wire.GoodUntil = quotation.GoodUntil.String()
	}

	if f.JSON() {
		if err := report.WriteJSON(env.Stdout, wire); err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
		return estimateCode(estimate)
	}

	env.Print("%s priced against the book of %s\n\n", wire.Application, wire.PricedOn)
	table := report.NewTable("item", "code", "description", "qty", "unit", "amount", "owner")
	table.RightAlign(3, 4, 5)
	for _, l := range estimate.Lines {
		quantity := unit.Whole(l.Quantity)
		if l.Scale > 1 {
			quantity = fmt.Sprintf("%d.%d", l.Quantity/l.Scale, l.Quantity%l.Scale)
		}
		table.Row(l.ItemID, string(l.Code), l.Description, quantity,
			l.UnitPrice.String(), l.Amount.String(), l.Owner)
	}
	if err := table.Render(env.Stdout); err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}
	env.Print("\ndirect cost %s\n", wire.Direct)
	for _, a := range wire.Adjustments {
		env.Print("%-24s %8s of %-12s %s\n", a.Name, a.Rate, a.On, a.Amount)
	}
	if wire.Total != wire.Direct {
		env.Print("total       %s\n", wire.Total)
	}
	if wire.GoodUntil != "" {
		env.Print("good until  %s\n", wire.GoodUntil)
	}
	for _, id := range estimate.Unpriced {
		fmt.Fprintf(env.Stderr, "no price for %s\n", id)
	}
	return estimateCode(estimate)
}

func estimateCode(e *costing.Estimate) int {
	if len(e.Unpriced) > 0 {
		return CodeFindings
	}
	return CodeOK
}

func costbookUsage() string {
	f := NewFlags("costbook", nil).WithJSON()
	f.Set().String("on", "", "the date to read the book at, as yyyy-mm-dd")
	return f.Usage("costbook [options]", "costbook [options]",
		"Prints every priced item in the unit cost book as it stood on a date.")
}

func runCostbook(env *Env) int {
	f := NewFlags("costbook", env.Stderr).WithJSON()
	f.Set().String("on", "", "the date to read the book at, as yyyy-mm-dd")
	if err := f.Parse(env.Args); err != nil {
		return CodeUsage
	}
	if len(f.Rest()) > 0 {
		return env.Fail(CodeUsage, "costbook takes no arguments")
	}
	on, err := requiredDateFlag(f, "on")
	if err != nil {
		return env.Fail(CodeUsage, "%v", err)
	}
	book := costing.Standard()
	type row struct {
		Code        string `json:"code"`
		Description string `json:"description"`
		Unit        string `json:"unit"`
		Price       string `json:"price"`
	}
	var rows []row
	for _, code := range book.Codes() {
		entry, err := book.Entry(code, on)
		if err != nil {
			return env.Fail(CodeUsage, "%v", err)
		}
		rows = append(rows, row{Code: string(entry.Code), Description: entry.Description,
			Unit: entry.Unit, Price: entry.Price.String()})
	}
	if f.JSON() {
		if err := report.WriteJSON(env.Stdout, rows); err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
		return CodeOK
	}
	env.Print("%s as of %s\n\n", book.Name, on)
	table := report.NewTable("code", "description", "unit", "price")
	table.RightAlign(3)
	for _, r := range rows {
		table.Row(r.Code, r.Description, r.Unit, r.Price)
	}
	return renderOrFail(env, table)
}
