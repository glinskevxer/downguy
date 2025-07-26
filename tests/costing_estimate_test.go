package tests

import (
	"testing"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/costing"
	"github.com/glinskevxer/downguy/internal/makeready"
	"github.com/glinskevxer/downguy/internal/unit"
)

func planWith(items ...makeready.Item) *makeready.Plan {
	return &makeready.Plan{Application: "APP-1042", Items: makeready.Order(items)}
}

func raise(id, pole, owner string) makeready.Item {
	return makeready.Item{
		ID: id, Action: makeready.ActionRaise, Pole: pole, Subject: "A3", Owner: owner,
		FromHeight: unit.Tenths(200), ToHeight: unit.Tenths(212), Because: []string{"x"},
	}
}

func TestRecipeCoverageForEveryAction(t *testing.T) {
	for _, a := range makeready.Actions() {
		r, ok := costing.RecipeFor(a)
		if !ok {
			t.Errorf("%s has no recipe", a)
			continue
		}
		if r.CrewHours <= 0 {
			t.Errorf("%s takes %d tenths of an hour", a, r.CrewHours)
		}
		if r.Action != a {
			t.Errorf("%s carries action %s", a, r.Action)
		}
	}
	if _, ok := costing.RecipeFor(makeready.Action("pray")); ok {
		t.Error("praying has no recipe")
	}
}

func TestPriceASingleRaise(t *testing.T) {
	on := calendar.MustNew(2025, 6, 1)
	got, err := costing.Price(planWith(raise("MR-0001", "NW-441829", "RIDGEPORT CABLEVISION")), costing.Standard(), on)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Lines) != 2 {
		t.Fatalf("lines = %v", got.Lines)
	}
	crew := got.Lines[0]
	if crew.Code != costing.CodeCrewHourComm {
		t.Errorf("first line = %s", crew.Code)
	}
	// 1.2 hours of a comm crew at $210.00
	if crew.Amount.AsCents() != 25200 {
		t.Errorf("crew amount = %s", crew.Amount)
	}
	if crew.Scale != costing.HoursScale {
		t.Errorf("scale = %d", crew.Scale)
	}
	bolt := got.Lines[1]
	if bolt.Code != costing.CodeThroughBolt || bolt.Amount.AsCents() != 1450 {
		t.Errorf("material line = %+v", bolt)
	}
	if got.Total.AsCents() != 26650 {
		t.Errorf("total = %s", got.Total)
	}
	if got.TotalForItem("MR-0001") != got.Total {
		t.Errorf("item total = %s", got.TotalForItem("MR-0001"))
	}
	if len(got.ForItem("MR-9999")) != 0 {
		t.Error("an unknown item has no lines")
	}
	if got.String() == "" {
		t.Error("String should say something")
	}
}

func TestPriceRepricesAgainstTheBookVersion(t *testing.T) {
	plan := planWith(raise("MR-0001", "NW-441829", "RIDGEPORT CABLEVISION"))
	b := costing.Standard()
	old, err := costing.Price(plan, b, calendar.MustNew(2025, 6, 1))
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := costing.Price(plan, b, calendar.MustNew(2026, 6, 1))
	if err != nil {
		t.Fatal(err)
	}
	if fresh.Total <= old.Total {
		t.Errorf("the 2026 book is dearer: %s against %s", fresh.Total, old.Total)
	}
	if old.Priced.String() != "2025-06-01" {
		t.Errorf("priced on %s", old.Priced)
	}
}

func TestPoleReplacementCarriesTrafficAndMobilisationOnce(t *testing.T) {
	plan := planWith(
		makeready.Item{ID: "MR-0001", Action: makeready.ActionReplacePole, Pole: "NW-441829",
			Owner: "SUMMIT VALLEY ELECTRIC", Because: []string{"x"}, Detail: "45/3"},
		makeready.Item{ID: "MR-0002", Action: makeready.ActionAddGuy, Pole: "NW-441829",
			Owner: "SUMMIT VALLEY ELECTRIC", Because: []string{"x"}},
	)
	got, err := costing.Price(plan, costing.Standard(), calendar.MustNew(2025, 6, 1))
	if err != nil {
		t.Fatal(err)
	}
	mob, traffic := 0, 0
	for _, l := range got.Lines {
		switch l.Code {
		case costing.CodeMobilisation:
			mob++
		case costing.CodeTrafficControl:
			traffic++
		}
	}
	if mob != 1 {
		t.Errorf("one trip to the pole, not %d", mob)
	}
	if traffic != 1 {
		t.Errorf("one traffic control day, not %d", traffic)
	}
}

func TestMobilisationIsChargedPerPole(t *testing.T) {
	plan := planWith(
		makeready.Item{ID: "MR-0001", Action: makeready.ActionAddGuy, Pole: "NW-441829", Owner: "A", Because: []string{"x"}},
		makeready.Item{ID: "MR-0002", Action: makeready.ActionAddGuy, Pole: "NW-441830", Owner: "A", Because: []string{"x"}},
	)
	got, err := costing.Price(plan, costing.Standard(), calendar.MustNew(2025, 6, 1))
	if err != nil {
		t.Fatal(err)
	}
	mob := 0
	for _, l := range got.Lines {
		if l.Code == costing.CodeMobilisation {
			mob++
		}
	}
	if mob != 2 {
		t.Errorf("two poles, two trips, got %d", mob)
	}
}

func TestPriceListsWhatItCannotPrice(t *testing.T) {
	plan := planWith(makeready.Item{
		ID: "MR-0001", Action: makeready.Action("pray"), Pole: "NW-441829", Because: []string{"x"},
	})
	got, err := costing.Price(plan, costing.Standard(), calendar.MustNew(2025, 6, 1))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Lines) != 0 {
		t.Errorf("lines = %v", got.Lines)
	}
	if len(got.Unpriced) != 1 || got.Unpriced[0] != "MR-0001" {
		t.Errorf("unpriced = %v", got.Unpriced)
	}
	if !got.Total.IsZero() {
		t.Errorf("total = %s", got.Total)
	}
}

func TestPriceRejectsMissingInput(t *testing.T) {
	on := calendar.MustNew(2025, 6, 1)
	if _, err := costing.Price(nil, costing.Standard(), on); err == nil {
		t.Error("no plan should be rejected")
	}
	if _, err := costing.Price(planWith(), nil, on); err == nil {
		t.Error("no book should be rejected")
	}
	if _, err := costing.Price(planWith(raise("MR-1", "NW-441829", "A")), costing.Standard(), calendar.MustNew(2024, 1, 1)); err == nil {
		t.Error("a date the book does not reach should be rejected")
	}
}

func TestEstimateByOwner(t *testing.T) {
	plan := planWith(
		raise("MR-0001", "NW-441829", "RIDGEPORT CABLEVISION"),
		raise("MR-0002", "NW-441829", "NORTHVALE TELEPHONE"),
		makeready.Item{ID: "MR-0003", Action: makeready.ActionRemove, Pole: "NW-441829", Subject: "A9", Because: []string{"x"}},
	)
	got, err := costing.Price(plan, costing.Standard(), calendar.MustNew(2025, 6, 1))
	if err != nil {
		t.Fatal(err)
	}
	byOwner := got.ByOwner()
	if len(byOwner) != 3 {
		t.Fatalf("by owner = %v", byOwner)
	}
	if byOwner["RIDGEPORT CABLEVISION"] != byOwner["NORTHVALE TELEPHONE"] {
		t.Error("two identical raises should cost the same")
	}
	if byOwner["UNASSIGNED"].IsZero() {
		t.Error("work with no owner should land in the unassigned bucket")
	}
	var total unit.Money
	for _, v := range byOwner {
		total = total.Add(v)
	}
	if total != got.Total {
		t.Errorf("the owner split %s does not add to the total %s", total, got.Total)
	}
}

func TestEstimateLinesAreSorted(t *testing.T) {
	plan := planWith(
		raise("MR-0002", "NW-441829", "A"),
		raise("MR-0001", "NW-441829", "A"),
	)
	got, err := costing.Price(plan, costing.Standard(), calendar.MustNew(2025, 6, 1))
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(got.Lines); i++ {
		a, b := got.Lines[i-1], got.Lines[i]
		if a.ItemID > b.ItemID || (a.ItemID == b.ItemID && a.Code > b.Code) {
			t.Fatalf("lines are not sorted: %v", got.Lines)
		}
	}
}
