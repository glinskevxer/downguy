package tests

import (
	"testing"

	"github.com/glinskevxer/downguy/internal/clearance"
	"github.com/glinskevxer/downguy/internal/makeready"
	"github.com/glinskevxer/downguy/internal/nesc"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/unit"
)

func planFinding(rule, subject, against string, measured, required int) clearance.Finding {
	return clearance.Finding{
		RuleID: rule, Severity: nesc.SeverityCorrect,
		Pole: "NW-441829", Subject: subject, Against: against,
		Owner: "RIDGEPORT CABLEVISION", Blame: clearance.BlameApplicant,
		Measured: unit.Tenths(measured), Required: unit.Tenths(required),
	}
}

func planner() *makeready.Planner {
	p := makeready.NewPlanner("APP-1042")
	p.Know(sampleAssembly())
	return p
}

func TestPlannerLowersWhenThereIsRoomUnderneath(t *testing.T) {
	// A3 sits at 200 with A4 at 186, so there are 1.4 feet under it and the
	// planner may drop it 0.4 rather than lift the neutral.
	got := planner().Plan(clearance.Findings{planFinding("NESC-235C1", "A3", "A2", 85, 89)})
	if len(got.Items) != 1 {
		t.Fatalf("items = %v", got.Items)
	}
	i := got.Items[0]
	if i.Action != makeready.ActionLower {
		t.Errorf("action = %s", i.Action)
	}
	if i.Subject != "A3" || i.FromHeight.Tenths() != 200 || i.ToHeight.Tenths() != 196 {
		t.Errorf("item = %+v", i)
	}
	if err := got.Validate(); err != nil {
		t.Errorf("the plan does not validate: %v", err)
	}
}

func TestPlannerRaisesWhenThereIsNoRoom(t *testing.T) {
	a := plant.NewAssembly(samplePole(), []plant.Attachment{
		att("A2", "SUMMIT VALLEY ELECTRIC", plant.KindNeutral, 285),
		att("A3", "RIDGEPORT CABLEVISION", plant.KindCommCable, 200),
		att("A4", "NORTHVALE TELEPHONE", plant.KindCommCable, 192),
	})
	p := makeready.NewPlanner("APP-1042")
	p.Know(a)
	got := p.Plan(clearance.Findings{planFinding("NESC-235C1", "A3", "A2", 85, 130)})
	if len(got.Items) != 1 {
		t.Fatalf("items = %v", got.Items)
	}
	i := got.Items[0]
	if i.Action != makeready.ActionRaise {
		t.Errorf("action = %s", i.Action)
	}
	if i.Subject != "A2" {
		t.Errorf("with nowhere to drop the cable the neutral goes up, got %s", i.Subject)
	}
	if i.Owner != "SUMMIT VALLEY ELECTRIC" {
		t.Errorf("owner = %q", i.Owner)
	}
	if i.ToHeight.Tenths() != 285+45 {
		t.Errorf("to = %s", i.ToHeight)
	}
}

func TestPlannerRaisesAGroundClearanceFinding(t *testing.T) {
	f := planFinding("NESC-232A", "A3", "road", 148, 155)
	f.Span = "SS-330"
	got := planner().Plan(clearance.Findings{f})
	if len(got.Items) != 1 {
		t.Fatalf("items = %v", got.Items)
	}
	i := got.Items[0]
	if i.Action != makeready.ActionRaise || i.Span != "SS-330" {
		t.Errorf("item = %+v", i)
	}
	// The lift is worked from the pole height, not from the mid span reading.
	if i.FromHeight.Tenths() != 200 || i.ToHeight.Tenths() != 207 {
		t.Errorf("item = %+v", i)
	}
}

func TestPlannerWorksFromTheMeasurementWhenThePoleIsUnknown(t *testing.T) {
	p := makeready.NewPlanner("APP-1042")
	f := planFinding("NESC-232A", "A3", "road", 148, 155)
	got := p.Plan(clearance.Findings{f})
	if len(got.Items) != 1 {
		t.Fatalf("items = %v", got.Items)
	}
	if got.Items[0].FromHeight.Tenths() != 148 {
		t.Errorf("from = %s", got.Items[0].FromHeight)
	}
}

func TestPlannerHandlesTheStructuralRules(t *testing.T) {
	cases := map[string]makeready.Action{
		"NESC-236A": makeready.ActionRelocateEquipment,
		"NESC-239E": makeready.ActionRelocateEquipment,
		"NESC-261A": makeready.ActionAddGuy,
	}
	for rule, want := range cases {
		got := planner().Plan(clearance.Findings{planFinding(rule, "A6", "", 0, 25)})
		if len(got.Items) != 1 {
			t.Fatalf("%s gave %v", rule, got.Items)
		}
		if got.Items[0].Action != want {
			t.Errorf("%s = %s, want %s", rule, got.Items[0].Action, want)
		}
		if len(got.Items[0].Because) != 1 {
			t.Errorf("%s cites nothing", rule)
		}
	}
}

func TestPlannerReportsWhatItCannotAnswer(t *testing.T) {
	got := planner().Plan(clearance.Findings{planFinding("NESC-250B", "", "", 0, 0)})
	if len(got.Items) != 0 {
		t.Errorf("items = %v", got.Items)
	}
	if len(got.Unresolved) != 1 {
		t.Errorf("unresolved = %v", got.Unresolved)
	}
	if !got.Empty() {
		t.Error("a plan with no items is empty")
	}
}

func TestPlannerSkipsAFindingWithNoDeficit(t *testing.T) {
	got := planner().Plan(clearance.Findings{planFinding("NESC-235C1", "A3", "A2", 100, 89)})
	if len(got.Items) != 0 || len(got.Unresolved) != 1 {
		t.Errorf("a finding that clears has no remedy, got %v %v", got.Items, got.Unresolved)
	}
}

func TestPlannerSkipsAnUnknownSubject(t *testing.T) {
	got := planner().Plan(clearance.Findings{planFinding("NESC-235C1", "A9", "A2", 85, 130)})
	if len(got.Items) != 0 || len(got.Unresolved) != 1 {
		t.Errorf("a subject not on the pole has no remedy, got %v", got.Items)
	}
	nowhere := makeready.NewPlanner("APP-1042")
	nowhere.Know(nil)
	got = nowhere.Plan(clearance.Findings{planFinding("NESC-235C1", "A3", "A2", 85, 130)})
	if len(got.Unresolved) != 1 {
		t.Errorf("an unknown pole has no remedy, got %v", got.Items)
	}
}

func TestPlanOrdersPoleWorkFirst(t *testing.T) {
	items := []makeready.Item{
		{ID: "MR-0003", Action: makeready.ActionRaise, Pole: "NW-441830", Subject: "A3", Because: []string{"x"}, FromHeight: unit.Tenths(200), ToHeight: unit.Tenths(210)},
		{ID: "MR-0002", Action: makeready.ActionAddGuy, Pole: "NW-441829", Because: []string{"x"}},
		{ID: "MR-0001", Action: makeready.ActionRaise, Pole: "NW-441829", Subject: "A4", Because: []string{"x"}, FromHeight: unit.Tenths(186), ToHeight: unit.Tenths(196)},
	}
	got := makeready.Order(items)
	if got[0].ID != "MR-0002" {
		t.Errorf("the guy comes before the lift on the same pole, got %s", got[0].ID)
	}
	if got[1].ID != "MR-0001" || got[2].ID != "MR-0003" {
		t.Errorf("order = %v", got)
	}
	if items[0].ID != "MR-0003" {
		t.Error("Order should work on a copy")
	}
}

func TestPlanSummaryAccessors(t *testing.T) {
	got := planner().Plan(clearance.Findings{
		planFinding("NESC-235C1", "A3", "A2", 85, 89),
		planFinding("NESC-236A", "A6", "", 0, 25),
	})
	if len(got.Poles()) != 1 || got.Poles()[0] != "NW-441829" {
		t.Errorf("poles = %v", got.Poles())
	}
	if len(got.Owners()) != 1 || got.Owners()[0] != "RIDGEPORT CABLEVISION" {
		t.Errorf("owners = %v", got.Owners())
	}
	if got.Empty() {
		t.Error("this plan has work in it")
	}
	if got.Application != "APP-1042" {
		t.Errorf("application = %q", got.Application)
	}
}

func TestPlanValidateCatchesADuplicateID(t *testing.T) {
	p := &makeready.Plan{Items: []makeready.Item{
		{ID: "MR-1", Action: makeready.ActionAddGuy, Pole: "NW-441829", Because: []string{"x"}},
		{ID: "MR-1", Action: makeready.ActionAddGuy, Pole: "NW-441829", Because: []string{"x"}},
	}}
	if err := p.Validate(); err == nil {
		t.Error("a repeated id should be rejected")
	}
	bad := &makeready.Plan{Items: []makeready.Item{{ID: "", Action: makeready.ActionAddGuy}}}
	if err := bad.Validate(); err == nil {
		t.Error("an invalid item should be rejected")
	}
}
