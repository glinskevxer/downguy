package tests

import (
	"math"
	"testing"

	"github.com/glinskevxer/downguy/internal/loading"
	"github.com/glinskevxer/downguy/internal/nesc"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/unit"
)

func sampleWires(t *testing.T) []loading.Wire {
	t.Helper()
	primary := sparrow(t)
	messenger, err := sagtenLookup(t, "3/8 EHS MESSENGER")
	if err != nil {
		t.Fatal(err)
	}
	return []loading.Wire{
		{AttachmentID: "A1", Owner: "SUMMIT VALLEY ELECTRIC", Conductor: primary, WindSpanFeet: 230, Height: unit.Tenths(335)},
		{AttachmentID: "A2", Owner: "SUMMIT VALLEY ELECTRIC", Conductor: primary, WindSpanFeet: 230, Height: unit.Tenths(285)},
		{AttachmentID: "A3", Owner: "RIDGEPORT CABLEVISION", Conductor: messenger, WindSpanFeet: 230, Height: unit.Tenths(200)},
	}
}

func TestConditionFactorTable(t *testing.T) {
	cases := map[plant.Condition]float64{
		plant.ConditionNew:        1.00,
		plant.ConditionSound:      1.00,
		plant.ConditionServiceabe: 0.85,
		plant.ConditionRestorable: 0.70,
		plant.ConditionReject:     0.00,
		plant.ConditionUnknown:    0.75,
	}
	for c, want := range cases {
		got, ok := loading.ConditionFactor(c)
		if !ok || got != want {
			t.Errorf("ConditionFactor(%s) = %v ok=%v, want %v", c, got, ok, want)
		}
	}
	if _, ok := loading.ConditionFactor(plant.Condition("mint")); ok {
		t.Error("an unknown condition should report false")
	}
}

func TestAnalyseSumsEveryComponent(t *testing.T) {
	a, err := loading.Analyse(samplePole(), sampleWires(t), nesc.Heavy, nesc.GradeC)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Components) != 4 {
		t.Fatalf("components = %d", len(a.Components))
	}
	total := 0.0
	for _, c := range a.Components {
		if c.Factored <= 0 {
			t.Errorf("component %s %s = %v", c.Source, c.Kind, c.Factored)
		}
		total += c.Factored
	}
	if math.Abs(total-a.FactoredMoment) > 1e-9 {
		t.Errorf("factored moment %v does not match the sum %v", a.FactoredMoment, total)
	}
	for i := 1; i < len(a.Components); i++ {
		if a.Components[i].Factored > a.Components[i-1].Factored {
			t.Fatal("components should come back worst first")
		}
	}
	worst, ok := a.Worst()
	if !ok || worst.Factored != a.Components[0].Factored {
		t.Errorf("worst = %+v", worst)
	}
	empty := loading.Analysis{}
	if _, ok := empty.Worst(); ok {
		t.Error("an empty analysis has no worst component")
	}
}

func TestAnalyseAppliesTheGradeFactors(t *testing.T) {
	wires := sampleWires(t)
	c, err := loading.Analyse(samplePole(), wires, nesc.Heavy, nesc.GradeC)
	if err != nil {
		t.Fatal(err)
	}
	b, err := loading.Analyse(samplePole(), wires, nesc.Heavy, nesc.GradeB)
	if err != nil {
		t.Fatal(err)
	}
	if b.FactoredMoment <= c.FactoredMoment {
		t.Error("grade B factors the load up harder")
	}
	if b.FactoredCapacty >= c.FactoredCapacty {
		t.Error("grade B knocks the capacity down harder")
	}
	if b.Utilisation <= c.Utilisation {
		t.Error("the same pole is worse off under grade B")
	}
	if b.Capacity != c.Capacity {
		t.Error("the unfactored capacity does not depend on the grade")
	}
}

func TestAnalyseDeratesADeterioratedPole(t *testing.T) {
	wires := sampleWires(t)
	sound, err := loading.Analyse(samplePole(), wires, nesc.Heavy, nesc.GradeC)
	if err != nil {
		t.Fatal(err)
	}
	worn := samplePole()
	worn.Condition = plant.ConditionRestorable
	tired, err := loading.Analyse(worn, wires, nesc.Heavy, nesc.GradeC)
	if err != nil {
		t.Fatal(err)
	}
	if tired.FactoredCapacty >= sound.FactoredCapacty {
		t.Error("a restorable pole carries less than a sound one")
	}
	if tired.FactoredMoment != sound.FactoredMoment {
		t.Error("the load does not change with the pole condition")
	}
	rejected := samplePole()
	rejected.Condition = plant.ConditionReject
	dead, err := loading.Analyse(rejected, wires, nesc.Heavy, nesc.GradeC)
	if err != nil {
		t.Fatal(err)
	}
	if dead.Passes() {
		t.Error("a reject pole carries nothing and cannot pass")
	}
	if dead.FactoredCapacty != 0 || dead.Utilisation != 0 {
		t.Errorf("a reject pole = %+v", dead)
	}
}

func TestAnalysePassAndHeadroom(t *testing.T) {
	a, err := loading.Analyse(samplePole(), sampleWires(t), nesc.Heavy, nesc.GradeC)
	if err != nil {
		t.Fatal(err)
	}
	if a.Passes() != (a.Headroom() >= 0) {
		t.Errorf("pass %v does not agree with headroom %v", a.Passes(), a.Headroom())
	}
	if math.Abs(a.Headroom()-(a.FactoredCapacty-a.FactoredMoment)) > 1e-9 {
		t.Errorf("headroom = %v", a.Headroom())
	}
	weak := samplePole()
	weak.Class = plant.Class7
	weak.Length = 30
	over, err := loading.Analyse(weak, sampleWires(t), nesc.Heavy, nesc.GradeB)
	if err != nil {
		t.Fatal(err)
	}
	if over.Passes() {
		t.Errorf("a class 7 pole under grade B should not carry three wires, got %s", over)
	}
	if over.Headroom() >= 0 {
		t.Errorf("headroom = %v", over.Headroom())
	}
}

func TestUtilisationPercentRoundsUp(t *testing.T) {
	cases := []struct {
		u    float64
		want int
	}{
		{0, 0},
		{0.5, 50},
		{0.501, 51},
		{0.999, 100},
		{1.0, 100},
		{1.001, 101},
	}
	for _, c := range cases {
		a := loading.Analysis{Utilisation: c.u}
		if got := a.UtilisationPercent(); got != c.want {
			t.Errorf("UtilisationPercent(%v) = %d, want %d", c.u, got, c.want)
		}
	}
}

func TestAnalyseRejectsWhatItCannotWorkWith(t *testing.T) {
	wires := sampleWires(t)
	bad := samplePole()
	bad.Owner = ""
	if _, err := loading.Analyse(bad, wires, nesc.Heavy, nesc.GradeC); err == nil {
		t.Error("an invalid pole should be rejected")
	}
	if _, err := loading.Analyse(samplePole(), wires, nesc.District("tropical"), nesc.GradeC); err == nil {
		t.Error("an unknown district should be rejected")
	}
	if _, err := loading.Analyse(samplePole(), wires, nesc.Heavy, nesc.Grade("Z")); err == nil {
		t.Error("an unknown grade should be rejected")
	}
	untabulated := samplePole()
	untabulated.Length = 95
	if _, err := loading.Analyse(untabulated, wires, nesc.Heavy, nesc.GradeC); err == nil {
		t.Error("an untabulated pole should be rejected")
	}
	grounded := wires
	grounded[0].Height = 0
	if _, err := loading.Analyse(samplePole(), grounded, nesc.Heavy, nesc.GradeC); err == nil {
		t.Error("a wire at no height should be rejected")
	}
}

func TestAnalysisString(t *testing.T) {
	a, err := loading.Analyse(samplePole(), sampleWires(t), nesc.Heavy, nesc.GradeC)
	if err != nil {
		t.Fatal(err)
	}
	got := a.String()
	if got == "" || got[:9] != "NW-441829" {
		t.Errorf("String = %q", got)
	}
}
