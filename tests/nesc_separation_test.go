package tests

import (
	"testing"

	"github.com/glinskevxer/downguy/internal/nesc"
	"github.com/glinskevxer/downguy/internal/unit"
)

func TestSafetyZoneFullAndReduced(t *testing.T) {
	full, rule := nesc.SafetyZone(false)
	if full.Tenths() != unit.Inches(40).Tenths() {
		t.Errorf("full safety zone = %s", full)
	}
	if rule != "NESC-235C1" {
		t.Errorf("rule = %s", rule)
	}
	reduced, rule := nesc.SafetyZone(true)
	if reduced.Tenths() != unit.Inches(30).Tenths() {
		t.Errorf("reduced safety zone = %s", reduced)
	}
	if rule != "NESC-235C1" {
		t.Errorf("rule = %s", rule)
	}
	if reduced >= full {
		t.Error("the neutral case should be the smaller number")
	}
}

func TestCommunicationSeparation(t *testing.T) {
	apart, rule := nesc.CommunicationSeparation(false)
	if apart.Tenths() != unit.Inches(12).Tenths() || rule != "NESC-235C2" {
		t.Errorf("two attachers = %s %s", apart, rule)
	}
	same, _ := nesc.CommunicationSeparation(true)
	if same.Tenths() != unit.Inches(4).Tenths() {
		t.Errorf("one attacher = %s", same)
	}
	if same >= apart {
		t.Error("one attacher may sit closer to itself")
	}
}

func TestMidspanSeparation(t *testing.T) {
	supply, rule := nesc.MidspanSeparation(true)
	if supply.Tenths() != unit.Inches(30).Tenths() || rule != "NESC-235E" {
		t.Errorf("supply over comm = %s %s", supply, rule)
	}
	comm, _ := nesc.MidspanSeparation(false)
	if comm.Tenths() != unit.Inches(12).Tenths() {
		t.Errorf("comm over comm = %s", comm)
	}
	if comm >= supply {
		t.Error("supply is held further off at mid span")
	}
}

func TestFixedClearances(t *testing.T) {
	eq, rule := nesc.EquipmentClearance()
	if eq.Tenths() != unit.Inches(30).Tenths() || rule != "NESC-238A" {
		t.Errorf("equipment = %s %s", eq, rule)
	}
	run, rule := nesc.VerticalRunClearance()
	if run.Tenths() != unit.Inches(3).Tenths() || rule != "NESC-239E" {
		t.Errorf("vertical run = %s %s", run, rule)
	}
	climb, rule := nesc.ClimbingSpace()
	if climb.Tenths() != unit.Inches(30).Tenths() || rule != "NESC-236A" {
		t.Errorf("climbing space = %s %s", climb, rule)
	}
}

func TestEverySeparationCitesACatalogueRule(t *testing.T) {
	_, a := nesc.SafetyZone(false)
	_, b := nesc.CommunicationSeparation(false)
	_, c := nesc.MidspanSeparation(true)
	_, d := nesc.EquipmentClearance()
	_, e := nesc.VerticalRunClearance()
	_, f := nesc.ClimbingSpace()
	for _, id := range []string{a, b, c, d, e, f} {
		if _, err := nesc.LookupRule(id); err != nil {
			t.Errorf("%s is cited but not catalogued", id)
		}
	}
}

func TestDeficitNeverGoesNegative(t *testing.T) {
	required := unit.Inches(40)
	cases := []struct {
		measured int
		want     int
	}{
		{unit.Inches(40).Tenths(), 0},
		{unit.Inches(48).Tenths(), 0},
		{unit.Inches(36).Tenths(), unit.Inches(40).Tenths() - unit.Inches(36).Tenths()},
		{0, unit.Inches(40).Tenths()},
	}
	for _, c := range cases {
		got := nesc.Deficit(unit.Tenths(c.measured), required)
		if got.Tenths() != c.want {
			t.Errorf("Deficit(%d) = %d, want %d", c.measured, got.Tenths(), c.want)
		}
	}
}

func TestMeetsAndDescribe(t *testing.T) {
	required := unit.Inches(40)
	if !nesc.Meets(required, required) {
		t.Error("exactly the required gap meets it")
	}
	if nesc.Meets(unit.Inches(36), required) {
		t.Error("four inches short does not meet it")
	}
	if !nesc.Meets(unit.Inches(48), required) {
		t.Error("more than the required gap meets it")
	}
	short := nesc.DescribeDeficit(unit.Tenths(28), unit.Tenths(33))
	want := "2.8 ft measured against 3.3 ft required, short by 0.5 ft"
	if short != want {
		t.Errorf("DescribeDeficit short = %q, want %q", short, want)
	}
	ok := nesc.DescribeDeficit(unit.Tenths(40), unit.Tenths(33))
	if ok != "4.0 ft measured against 3.3 ft required" {
		t.Errorf("DescribeDeficit ok = %q", ok)
	}
}
