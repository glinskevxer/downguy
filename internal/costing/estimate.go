package costing

import (
	"fmt"
	"sort"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/makeready"
	"github.com/glinskevxer/downguy/internal/unit"
)

// Recipe is what one make ready action consumes: how many hours of which crew,
// and what material. It is data rather than a switch statement because the
// estimating group edits it far more often than anybody edits this package.
type Recipe struct {
	Action    makeready.Action
	CrewCode  Code
	CrewHours int
	Materials []Consumption
	Traffic   bool
	Mobilise  bool
}

// Consumption is a quantity of one priced code.
type Consumption struct {
	Code     Code
	Quantity int
}

// HoursScale is the denominator crew hours are held in. Hours are quoted in
// tenths, because half of a make ready item is under an hour and rounding
// every one of them up to a full crew hour doubles a large estimate.
const HoursScale = 10

var recipes = map[makeready.Action]Recipe{
	makeready.ActionRaise: {
		Action: makeready.ActionRaise, CrewCode: CodeCrewHourComm, CrewHours: 12,
		Materials: []Consumption{{Code: CodeThroughBolt, Quantity: 1}},
	},
	makeready.ActionLower: {
		Action: makeready.ActionLower, CrewCode: CodeCrewHourComm, CrewHours: 12,
		Materials: []Consumption{{Code: CodeThroughBolt, Quantity: 1}},
	},
	makeready.ActionTransfer: {
		Action: makeready.ActionTransfer, CrewCode: CodeCrewHourComm, CrewHours: 18,
		Materials: []Consumption{{Code: CodeThroughBolt, Quantity: 1}},
	},
	makeready.ActionRelocateEquipment: {
		Action: makeready.ActionRelocateEquipment, CrewCode: CodeCrewHourComm, CrewHours: 25,
		Materials: []Consumption{{Code: CodeStandoffBracket, Quantity: 1}, {Code: CodeThroughBolt, Quantity: 2}},
	},
	makeready.ActionRemove: {
		Action: makeready.ActionRemove, CrewCode: CodeCrewHourComm, CrewHours: 15,
	},
	makeready.ActionResag: {
		Action: makeready.ActionResag, CrewCode: CodeCrewHourComm, CrewHours: 20,
	},
	makeready.ActionAddGuy: {
		Action: makeready.ActionAddGuy, CrewCode: CodeCrewHourLine, CrewHours: 40,
		Materials: []Consumption{
			{Code: CodeAnchorRod, Quantity: 1},
			{Code: CodeGuyStrand, Quantity: 45},
			{Code: CodeGuyGuard, Quantity: 1},
		},
		Mobilise: true,
	},
	makeready.ActionReplacePole: {
		Action: makeready.ActionReplacePole, CrewCode: CodeCrewHourDigger, CrewHours: 80,
		Materials: []Consumption{
			{Code: CodePole45Class3, Quantity: 1},
			{Code: CodeDisposal, Quantity: 1},
		},
		Traffic: true, Mobilise: true,
	},
}

// RecipeFor returns the recipe for an action.
func RecipeFor(a makeready.Action) (Recipe, bool) {
	r, ok := recipes[a]
	return r, ok
}

// Line is one priced line of an estimate.
type Line struct {
	ItemID      string
	Code        Code
	Description string
	Quantity    int
	Scale       int
	UnitPrice   unit.Money
	Amount      unit.Money
	Owner       string
}

// Estimate is the priced make ready for one plan.
type Estimate struct {
	Application string
	Priced      calendar.Date
	Lines       []Line
	Unpriced    []string
	Total       unit.Money
}

// Price walks a plan and prices every item against the book as it stood on a
// day. Items with no recipe are listed rather than silently costing nothing.
func Price(plan *makeready.Plan, b *Book, on calendar.Date) (*Estimate, error) {
	if plan == nil {
		return nil, fmt.Errorf("%w: no plan to price", ErrNoPrice)
	}
	if b == nil {
		return nil, fmt.Errorf("%w: no book to price against", ErrNoPrice)
	}
	e := &Estimate{Application: plan.Application, Priced: on}
	poles := map[string]bool{}
	mobilised := map[string]bool{}
	trafficked := map[string]bool{}

	for _, item := range plan.Items {
		poles[item.Pole] = true
		r, ok := RecipeFor(item.Action)
		if !ok {
			e.Unpriced = append(e.Unpriced, item.ID)
			continue
		}
		line, err := crewLine(b, on, item, r)
		if err != nil {
			return nil, err
		}
		e.Lines = append(e.Lines, line)

		for _, m := range r.Materials {
			price, err := b.Price(m.Code, on)
			if err != nil {
				return nil, err
			}
			entry, err := b.Entry(m.Code, on)
			if err != nil {
				return nil, err
			}
			e.Lines = append(e.Lines, Line{
				ItemID: item.ID, Code: m.Code, Description: entry.Description,
				Quantity: m.Quantity, Scale: 1, UnitPrice: price,
				Amount: price.Times(m.Quantity), Owner: item.Owner,
			})
		}
		if r.Mobilise && !mobilised[item.Pole] {
			mobilised[item.Pole] = true
			line, err := flatLine(b, on, item, CodeMobilisation)
			if err != nil {
				return nil, err
			}
			e.Lines = append(e.Lines, line)
		}
		if r.Traffic && !trafficked[item.Pole] {
			trafficked[item.Pole] = true
			line, err := flatLine(b, on, item, CodeTrafficControl)
			if err != nil {
				return nil, err
			}
			e.Lines = append(e.Lines, line)
		}
	}

	for _, l := range e.Lines {
		e.Total = e.Total.Add(l.Amount)
	}
	sort.SliceStable(e.Lines, func(i, j int) bool {
		if e.Lines[i].ItemID != e.Lines[j].ItemID {
			return e.Lines[i].ItemID < e.Lines[j].ItemID
		}
		return e.Lines[i].Code < e.Lines[j].Code
	})
	sort.Strings(e.Unpriced)
	return e, nil
}

func crewLine(b *Book, on calendar.Date, item makeready.Item, r Recipe) (Line, error) {
	price, err := b.Price(r.CrewCode, on)
	if err != nil {
		return Line{}, err
	}
	entry, err := b.Entry(r.CrewCode, on)
	if err != nil {
		return Line{}, err
	}
	amount := unit.Cents(price.AsCents() * int64(r.CrewHours) / HoursScale)
	return Line{
		ItemID: item.ID, Code: r.CrewCode, Description: entry.Description,
		Quantity: r.CrewHours, Scale: HoursScale, UnitPrice: price,
		Amount: amount, Owner: item.Owner,
	}, nil
}

func flatLine(b *Book, on calendar.Date, item makeready.Item, code Code) (Line, error) {
	price, err := b.Price(code, on)
	if err != nil {
		return Line{}, err
	}
	entry, err := b.Entry(code, on)
	if err != nil {
		return Line{}, err
	}
	return Line{
		ItemID: item.ID, Code: code, Description: entry.Description,
		Quantity: 1, Scale: 1, UnitPrice: price, Amount: price, Owner: item.Owner,
	}, nil
}

// ForItem returns the lines belonging to one work item.
func (e *Estimate) ForItem(id string) []Line {
	var out []Line
	for _, l := range e.Lines {
		if l.ItemID == id {
			out = append(out, l)
		}
	}
	return out
}

// TotalForItem returns what one work item costs.
func (e *Estimate) TotalForItem(id string) unit.Money {
	var total unit.Money
	for _, l := range e.ForItem(id) {
		total = total.Add(l.Amount)
	}
	return total
}

// ByOwner totals the estimate by attacher.
func (e *Estimate) ByOwner() map[string]unit.Money {
	out := map[string]unit.Money{}
	for _, l := range e.Lines {
		key := l.Owner
		if key == "" {
			key = "UNASSIGNED"
		}
		out[key] = out[key].Add(l.Amount)
	}
	return out
}

// String prints the estimate total the way a cover sheet does.
func (e *Estimate) String() string {
	return fmt.Sprintf("%s priced %s: %d lines, %s", e.Application, e.Priced, len(e.Lines), e.Total)
}
