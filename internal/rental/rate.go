package rental

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/unit"
)

// ErrBadRate is returned when an annual rate cannot be worked out.
var ErrBadRate = errors.New("rental: cannot compute the rate")

// Formula is which of the two rate methods applies to an attacher. Which one
// applies is a question about what the attacher is, not about what it has on
// the pole, and getting it wrong is worth several dollars a pole a year across
// a whole system.
type Formula string

// The two formulas.
const (
	// FormulaCable is the older method, charging for occupied space over
	// usable space only.
	FormulaCable Formula = "cable"
	// FormulaTelecom is the method that also charges a share of the unusable
	// space, divided among the attaching entities.
	FormulaTelecom Formula = "telecom"
)

// KnownFormula reports whether the formula is one the rate maker handles.
func KnownFormula(f Formula) bool { return f == FormulaCable || f == FormulaTelecom }

// ParseFormula reads a formula name.
func ParseFormula(text string) (Formula, error) {
	t := Formula(strings.ToLower(strings.TrimSpace(text)))
	if KnownFormula(t) {
		return t, nil
	}
	return "", fmt.Errorf("%w: unknown formula %q", ErrBadRate, text)
}

// Inputs are everything an annual rate is worked out from. Holding them on one
// record means a rate can be recomputed years later from what was filed rather
// than from what the system happens to hold now.
type Inputs struct {
	Year       calendar.RateYear
	Formula    Formula
	Space      Space
	Occupied   unit.Height
	Attachers  int
	PoleCost   unit.Money
	SpaceShare unit.BasisPoints
}

// Rate is the finished annual rate for one attachment.
type Rate struct {
	Inputs Inputs
	Annual unit.Money
}

// Compute works out the annual rate per attachment for a year.
func Compute(year calendar.RateYear, f Formula, s Space, occupied unit.Height, attachers int) (*Rate, error) {
	if !KnownFormula(f) {
		return nil, fmt.Errorf("%w: %q", ErrBadRate, f)
	}
	cost, err := AnnualPoleCost(year)
	if err != nil {
		return nil, err
	}
	var share unit.BasisPoints
	switch f {
	case FormulaCable:
		share, err = s.SpaceFactorCable(occupied)
	case FormulaTelecom:
		share, err = s.SpaceFactorTelecom(occupied, attachers)
	}
	if err != nil {
		return nil, err
	}
	in := Inputs{
		Year: year, Formula: f, Space: s, Occupied: occupied,
		Attachers: attachers, PoleCost: cost, SpaceShare: share,
	}
	return &Rate{Inputs: in, Annual: share.Of(cost)}, nil
}

// Standard returns the rate for the presumptive pole, which is what a tariff
// sheet publishes.
func Standard(year calendar.RateYear, f Formula, attachers int) (*Rate, error) {
	return Compute(year, f, StandardSpace(), StandardOccupied, attachers)
}

// Monthly returns a twelfth of the annual rate, rounded down, which is how a
// prorated bill is presented even though the rate itself is annual.
func (r *Rate) Monthly() unit.Money { return unit.Cents(r.Annual.AsCents() / 12) }

// Prorated returns what the attachment costs for the part of the rate year it
// was actually up, rounded to the nearest cent.
func (r *Rate) Prorated(active calendar.Range) unit.Money {
	num, den := calendar.ProrationOf(r.Inputs.Year, active)
	if den <= 0 || num <= 0 {
		return 0
	}
	if num >= den {
		return r.Annual
	}
	cents := r.Annual.AsCents()*int64(num*2) + int64(den)
	return unit.Cents(cents / int64(den*2))
}

// Explain writes the working, which is what goes in a rate filing and what an
// attacher asks for when it disputes a bill.
func (r *Rate) Explain() []string {
	return []string{
		fmt.Sprintf("rate year %s under the %s formula", r.Inputs.Year, r.Inputs.Formula),
		fmt.Sprintf("presumed pole %s, usable %s, unusable %s",
			r.Inputs.Space.Height, r.Inputs.Space.Usable, r.Inputs.Space.Unusable),
		fmt.Sprintf("occupied %s across %d attaching entities", r.Inputs.Occupied, r.Inputs.Attachers),
		fmt.Sprintf("space factor %s", r.Inputs.SpaceShare),
		fmt.Sprintf("annual cost of a bare pole %s", r.Inputs.PoleCost),
		fmt.Sprintf("annual rate %s", r.Annual),
	}
}

// String prints the rate the way a tariff sheet lists it.
func (r *Rate) String() string {
	return fmt.Sprintf("%s %s: %s per attachment per year", r.Inputs.Year, r.Inputs.Formula, r.Annual)
}

// Schedule is the published rate for every formula in a year.
type Schedule struct {
	Year      calendar.RateYear
	Attachers int
	Rates     map[Formula]*Rate
}

// Publish builds the schedule for a year.
func Publish(year calendar.RateYear, attachers int) (*Schedule, error) {
	s := &Schedule{Year: year, Attachers: attachers, Rates: map[Formula]*Rate{}}
	for _, f := range Formulas() {
		r, err := Standard(year, f, attachers)
		if err != nil {
			return nil, err
		}
		s.Rates[f] = r
	}
	return s, nil
}

// Formulas returns the formulas, sorted.
func Formulas() []Formula {
	out := []Formula{FormulaCable, FormulaTelecom}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// For returns the rate under one formula and reports whether the schedule
// carries it.
func (s *Schedule) For(f Formula) (*Rate, bool) {
	r, ok := s.Rates[f]
	return r, ok
}
