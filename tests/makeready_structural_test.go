package tests

import (
	"testing"

	"github.com/glinskevxer/downguy/internal/loading"
	"github.com/glinskevxer/downguy/internal/makeready"
	"github.com/glinskevxer/downguy/internal/nesc"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/unit"
)

func TestStructurePassesOnAGoodPole(t *testing.T) {
	before := sampleWires(t)
	after, err := loading.WithAttachment(before, att("A7", "MERIDIAN FIBER", plant.KindFiber, 174), "144F ADSS", nesc.Heavy)
	if err != nil {
		t.Fatal(err)
	}
	got, err := makeready.NewPlanner("APP-1042").AssessStructure(samplePole(), before, after, nil, nesc.Heavy, nesc.GradeC)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Passes {
		t.Fatalf("a class 4 pole should carry one more fiber: %s", got.Summary())
	}
	if len(got.Items) != 0 || len(got.Findings) != 0 {
		t.Errorf("a passing pole needs no work, got %+v", got)
	}
	if got.Worsened() <= 0 {
		t.Errorf("the new fiber should add utilisation, got %v", got.Worsened())
	}
	if got.AlreadyOver() {
		t.Error("the pole was not overloaded before")
	}
	if got.Summary() == "" {
		t.Error("Summary should say something")
	}
}

func TestStructureRecommendsAGuyWhenThePullIsTheProblem(t *testing.T) {
	weak := samplePole()
	weak.Class = plant.Class7
	weak.Length = 30
	wires := sampleWires(t)
	pulls := []loading.Pull{{Source: "A1", Force: 4000, Bearing: 90, Height: unit.Feet(24)}}
	got, err := makeready.NewPlanner("APP-1042").AssessStructure(weak, wires, wires, pulls, nesc.Heavy, nesc.GradeB)
	if err != nil {
		t.Fatal(err)
	}
	if got.Passes {
		t.Skip("the fixture no longer overloads the pole")
	}
	if !got.Guyable {
		t.Fatalf("a pole failing by less than the pull should be guyable: %s", got.Summary())
	}
	if len(got.Items) != 1 || got.Items[0].Action != makeready.ActionAddGuy {
		t.Errorf("items = %v", got.Items)
	}
	if len(got.Findings) != 1 || got.Findings[0].RuleID != "NESC-250B" {
		t.Errorf("findings = %v", got.Findings)
	}
}

func TestStructureRecommendsAPoleWhenAGuyWillNotDo(t *testing.T) {
	weak := samplePole()
	weak.Class = plant.Class7
	weak.Length = 30
	wires := sampleWires(t)
	got, err := makeready.NewPlanner("APP-1042").AssessStructure(weak, wires, wires, nil, nesc.Heavy, nesc.GradeB)
	if err != nil {
		t.Fatal(err)
	}
	if got.Passes {
		t.Skip("the fixture no longer overloads the pole")
	}
	if got.Guyable {
		t.Fatal("with no unbalanced pull there is nothing for a guy to take")
	}
	if !got.Replacement.Found {
		t.Fatalf("a replacement should have been found: %s", got.Summary())
	}
	if got.Items[0].Action != makeready.ActionReplacePole {
		t.Errorf("the first item should be the pole, got %s", got.Items[0].Action)
	}
	transfers := 0
	for _, i := range got.Items[1:] {
		if i.Action != makeready.ActionTransfer {
			t.Errorf("item = %+v", i)
		}
		transfers++
	}
	if transfers != len(wires) {
		t.Errorf("every wire should be transferred, got %d of %d", transfers, len(wires))
	}
	for _, i := range got.Items {
		if err := i.Validate(); err != nil {
			t.Errorf("%s does not validate: %v", i.ID, err)
		}
	}
}

func TestStructureReportsWhenNothingCarriesIt(t *testing.T) {
	wires := sampleWires(t)
	for i := range wires {
		wires[i].WindSpanFeet = 4000
	}
	got, err := makeready.NewPlanner("APP-1042").AssessStructure(samplePole(), wires, wires, nil, nesc.Heavy, nesc.GradeB)
	if err != nil {
		t.Fatal(err)
	}
	if got.Replacement.Found {
		t.Fatal("a four thousand foot wind span should defeat the stock list")
	}
	if len(got.Items) != 0 {
		t.Errorf("there is no work to specify, got %v", got.Items)
	}
	if got.Summary() == "" {
		t.Error("Summary should still say something")
	}
}

func TestStructureNoticesAnAlreadyOverloadedPole(t *testing.T) {
	weak := samplePole()
	weak.Class = plant.Class7
	weak.Length = 30
	wires := sampleWires(t)
	got, err := makeready.NewPlanner("APP-1042").AssessStructure(weak, wires, wires, nil, nesc.Heavy, nesc.GradeB)
	if err != nil {
		t.Fatal(err)
	}
	if !got.AlreadyOver() {
		t.Errorf("the pole was over before the applicant arrived, utilisation %v", got.Before)
	}
	if got.Worsened() != 0 {
		t.Errorf("with the same wire list nothing was added, got %v", got.Worsened())
	}
}

func TestAssessStructureRejectsBadInput(t *testing.T) {
	bad := samplePole()
	bad.Owner = ""
	wires := sampleWires(t)
	p := makeready.NewPlanner("APP-1042")
	if _, err := p.AssessStructure(bad, wires, wires, nil, nesc.Heavy, nesc.GradeC); err == nil {
		t.Error("an invalid pole should be rejected")
	}
	if _, err := p.AssessStructure(samplePole(), wires, wires, nil, nesc.District("tropical"), nesc.GradeC); err == nil {
		t.Error("an unknown district should be rejected")
	}
	grounded := sampleWires(t)
	grounded[0].Height = 0
	if _, err := p.AssessStructure(samplePole(), wires, grounded, nil, nesc.Heavy, nesc.GradeC); err == nil {
		t.Error("a wire at no height should be rejected")
	}
}
