package costing

import (
	"fmt"
	"sort"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/unit"
)

// Adjustment is a percentage applied on top of the priced lines. Overheads and
// contingency are quoted as rates rather than as amounts so that a revised
// estimate recomputes them instead of carrying a stale figure.
type Adjustment struct {
	Name  string
	Rate  unit.BasisPoints
	Basis Basis
}

// Basis says what an adjustment is a percentage of.
type Basis string

// The two bases an adjustment can take.
const (
	// BasisDirect applies to the priced lines alone.
	BasisDirect Basis = "direct"
	// BasisRunning applies to the priced lines plus every adjustment already
	// applied, which is how a tax on a marked up total works.
	BasisRunning Basis = "running"
)

// KnownBasis reports whether the basis is one the roll up handles.
func KnownBasis(b Basis) bool { return b == BasisDirect || b == BasisRunning }

// StandardAdjustments returns the adjustments this utility applies to a make
// ready estimate, in the order they are applied.
func StandardAdjustments() []Adjustment {
	return []Adjustment{
		{Name: "contingency", Rate: unit.Percent(10), Basis: BasisDirect},
		{Name: "engineering overhead", Rate: unit.BasisPoints(1250), Basis: BasisDirect},
		{Name: "administrative overhead", Rate: unit.BasisPoints(575), Basis: BasisRunning},
	}
}

// AppliedAdjustment is one adjustment with the amount it came to.
type AppliedAdjustment struct {
	Adjustment
	On     unit.Money
	Amount unit.Money
}

// Quotation is a priced estimate with its adjustments rolled up, and the date
// it stops being honoured.
type Quotation struct {
	Estimate    *Estimate
	Direct      unit.Money
	Adjustments []AppliedAdjustment
	Total       unit.Money
	Issued      calendar.Date
	GoodUntil   calendar.Date
}

// QuoteValidDays is how long an estimate is honoured before it has to be
// repriced. A make ready quote that outlives its book is how a utility ends up
// doing the work at last year's prices.
const QuoteValidDays = 90

// Quote rolls the adjustments onto an estimate.
func Quote(e *Estimate, adjustments []Adjustment, issued calendar.Date) (*Quotation, error) {
	if e == nil {
		return nil, fmt.Errorf("%w: no estimate to quote", ErrNoPrice)
	}
	if issued.IsZero() {
		return nil, fmt.Errorf("%w: a quotation needs an issue date", ErrNoPrice)
	}
	q := &Quotation{
		Estimate: e, Direct: e.Total, Total: e.Total,
		Issued: issued, GoodUntil: issued.AddDays(QuoteValidDays),
	}
	for _, a := range adjustments {
		if !KnownBasis(a.Basis) {
			return nil, fmt.Errorf("%w: %s is applied on a %q basis", ErrNoPrice, a.Name, a.Basis)
		}
		if a.Rate < 0 {
			return nil, fmt.Errorf("%w: %s is %s", ErrNoPrice, a.Name, a.Rate)
		}
		on := q.Direct
		if a.Basis == BasisRunning {
			on = q.Total
		}
		amount := a.Rate.Of(on)
		q.Adjustments = append(q.Adjustments, AppliedAdjustment{Adjustment: a, On: on, Amount: amount})
		q.Total = q.Total.Add(amount)
	}
	return q, nil
}

// Expired reports whether the quotation is past the day it is honoured to.
func (q *Quotation) Expired(on calendar.Date) bool {
	return !q.GoodUntil.IsZero() && on.After(q.GoodUntil)
}

// AdjustmentTotal returns everything added on top of the direct cost.
func (q *Quotation) AdjustmentTotal() unit.Money { return q.Total.Sub(q.Direct) }

// Named returns one applied adjustment by name.
func (q *Quotation) Named(name string) (AppliedAdjustment, bool) {
	for _, a := range q.Adjustments {
		if a.Name == name {
			return a, true
		}
	}
	return AppliedAdjustment{}, false
}

// Revision is a later quotation against an earlier one, so that a change in
// scope can be shown as a difference rather than as a new number.
type Revision struct {
	From      *Quotation
	To        *Quotation
	Delta     unit.Money
	NewItems  []string
	GoneItems []string
}

// Revise compares two quotations and reports what moved.
func Revise(from, to *Quotation) (*Revision, error) {
	if from == nil || to == nil {
		return nil, fmt.Errorf("%w: a revision needs two quotations", ErrNoPrice)
	}
	r := &Revision{From: from, To: to, Delta: to.Total.Sub(from.Total)}
	was := itemSet(from.Estimate)
	now := itemSet(to.Estimate)
	for id := range now {
		if !was[id] {
			r.NewItems = append(r.NewItems, id)
		}
	}
	for id := range was {
		if !now[id] {
			r.GoneItems = append(r.GoneItems, id)
		}
	}
	sort.Strings(r.NewItems)
	sort.Strings(r.GoneItems)
	return r, nil
}

func itemSet(e *Estimate) map[string]bool {
	out := map[string]bool{}
	if e == nil {
		return out
	}
	for _, l := range e.Lines {
		out[l.ItemID] = true
	}
	return out
}

// Cheaper reports whether the revision brings the price down.
func (r *Revision) Cheaper() bool { return r.Delta < 0 }

// String prints the revision the way a change letter opens.
func (r *Revision) String() string {
	direction := "up"
	if r.Cheaper() {
		direction = "down"
	}
	return fmt.Sprintf("%s to %s, %s by %s: %d items added, %d removed",
		r.From.Total, r.To.Total, direction, r.Delta.Sub(0).Abs(), len(r.NewItems), len(r.GoneItems))
}
