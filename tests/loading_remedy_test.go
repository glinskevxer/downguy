package tests

import (
	"testing"

	"github.com/glinskevxer/downguy/internal/loading"
	"github.com/glinskevxer/downguy/internal/nesc"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/unit"
)

func TestStockLengthsAreOrderedAndCopied(t *testing.T) {
	got := loading.StockLengths()
	if len(got) == 0 {
		t.Fatal("the yard carries nothing")
	}
	for i := 1; i < len(got); i++ {
		if got[i] <= got[i-1] {
			t.Fatalf("stock lengths are not ordered: %v", got)
		}
	}
	got[0] = 999
	if loading.StockLengths()[0] == 999 {
		t.Error("StockLengths should hand back a copy")
	}
}

func TestFindReplacementForAnOverloadedPole(t *testing.T) {
	weak := samplePole()
	weak.Class = plant.Class7
	weak.Length = 30
	wires := sampleWires(t)
	before, err := loading.Analyse(weak, wires, nesc.Heavy, nesc.GradeB)
	if err != nil {
		t.Fatal(err)
	}
	if before.Passes() {
		t.Skip("the fixture no longer overloads the pole")
	}
	got, err := loading.FindReplacement(weak, wires, nesc.Heavy, nesc.GradeB)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Found {
		t.Fatalf("no replacement found: %s", got.Reason)
	}
	if got.Length < weak.Length {
		t.Errorf("a replacement is never shorter, got %d for a %d foot pole", got.Length, weak.Length)
	}
	if got.Species != weak.Species {
		t.Errorf("the species should not change, got %s", got.Species)
	}
	if got.Utilisation > 1 {
		t.Errorf("the recommended pole is itself overloaded at %v", got.Utilisation)
	}
	check := weak
	check.Length = got.Length
	check.Class = got.Class
	check.Condition = plant.ConditionNew
	check.SetDepth = plant.StandardSetDepth(got.Length)
	after, err := loading.Analyse(check, wires, nesc.Heavy, nesc.GradeB)
	if err != nil {
		t.Fatal(err)
	}
	if !after.Passes() {
		t.Errorf("the recommended pole does not actually pass: %s", after)
	}
}

func TestFindReplacementPrefersTheSmallestThatPasses(t *testing.T) {
	weak := samplePole()
	weak.Class = plant.Class7
	weak.Length = 30
	wires := sampleWires(t)
	got, err := loading.FindReplacement(weak, wires, nesc.Heavy, nesc.GradeB)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Found {
		t.Fatalf("no replacement: %s", got.Reason)
	}
	// Anything weaker at the same length must fail, or the search picked the
	// wrong pole.
	classes := plant.Classes()
	rank, ok := plant.Rank(got.Class)
	if !ok {
		t.Fatalf("recommended class %s is not in the list", got.Class)
	}
	existing, ok := plant.Rank(weak.Class)
	if !ok {
		t.Fatal("the existing class is not in the list")
	}
	if rank > existing {
		t.Errorf("the search recommended class %s, weaker than the %s coming out", got.Class, weak.Class)
	}
	for i := existing; i > rank; i-- {
		weaker := weak
		weaker.Length = got.Length
		weaker.Class = classes[i]
		weaker.Condition = plant.ConditionNew
		weaker.SetDepth = plant.StandardSetDepth(got.Length)
		if _, err := plant.ButtCircumference(weaker.Class, weaker.Length); err != nil {
			continue
		}
		a, err := loading.Analyse(weaker, wires, nesc.Heavy, nesc.GradeB)
		if err == nil && a.Passes() {
			t.Errorf("a class %s at %d feet also passes, so the search should have chosen it",
				weaker.Class, weaker.Length)
		}
	}
}

func TestFindReplacementOnASoundPoleReturnsTheSameSize(t *testing.T) {
	worn := samplePole()
	worn.Condition = plant.ConditionRestorable
	wires := sampleWires(t)
	got, err := loading.FindReplacement(worn, wires, nesc.Heavy, nesc.GradeC)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Found {
		t.Fatalf("no replacement: %s", got.Reason)
	}
	if !got.SameSize(worn) {
		t.Errorf("a sound pole of this size carries the load, so the remedy is the wood, got %s", got)
	}
	if got.String() == "" {
		t.Error("String should say something")
	}
}

func TestFindReplacementGivesUpWhenNothingCarriesIt(t *testing.T) {
	p := samplePole()
	wires := sampleWires(t)
	for i := range wires {
		wires[i].WindSpanFeet = 4000
	}
	got, err := loading.FindReplacement(p, wires, nesc.Heavy, nesc.GradeB)
	if err != nil {
		t.Fatal(err)
	}
	if got.Found {
		t.Errorf("a four thousand foot wind span should defeat the stock list, got %s", got)
	}
	if got.String() != "no stock pole carries this load" {
		t.Errorf("String = %q", got.String())
	}
	if got.SameSize(p) {
		t.Error("a failed search matches nothing")
	}
}

func TestFindReplacementRejectsBadInput(t *testing.T) {
	bad := samplePole()
	bad.Owner = ""
	if _, err := loading.FindReplacement(bad, sampleWires(t), nesc.Heavy, nesc.GradeC); err == nil {
		t.Error("an invalid pole should be rejected")
	}
	if _, err := loading.FindReplacement(samplePole(), sampleWires(t), nesc.District("tropical"), nesc.GradeC); err == nil {
		t.Error("an unknown district should be rejected")
	}
	if _, err := loading.FindReplacement(samplePole(), sampleWires(t), nesc.Heavy, nesc.Grade("Z")); err == nil {
		t.Error("an unknown grade should be rejected")
	}
}

func TestNeedsGuyingSeparatesAPullFromAnOversizeLoad(t *testing.T) {
	weak := samplePole()
	weak.Class = plant.Class7
	weak.Length = 30
	wires := sampleWires(t)
	pulls := []loading.Pull{{Source: "A1", Force: 4000, Bearing: 90, Height: unit.Feet(24)}}
	guyable, err := loading.NeedsGuying(weak, wires, pulls, nesc.Heavy, nesc.GradeB)
	if err != nil {
		t.Fatal(err)
	}
	if !guyable {
		t.Error("a pole failing by less than the pull should be answerable with a guy")
	}
	none, err := loading.NeedsGuying(weak, wires, nil, nesc.Heavy, nesc.GradeB)
	if err != nil {
		t.Fatal(err)
	}
	if none {
		t.Error("with no unbalanced pull there is nothing for a guy to take off")
	}
	sound, err := loading.NeedsGuying(samplePole(), wires, pulls, nesc.Heavy, nesc.GradeC)
	if err != nil {
		t.Fatal(err)
	}
	if sound {
		t.Error("a pole that already passes does not need guying")
	}
	bad := samplePole()
	bad.Length = 95
	if _, err := loading.NeedsGuying(bad, wires, pulls, nesc.Heavy, nesc.GradeC); err == nil {
		t.Error("an untabulated pole should be rejected")
	}
}
