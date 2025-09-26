package tests

import (
	"testing"

	"github.com/glinskevxer/downguy/internal/clearance"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/survey"
	"github.com/glinskevxer/downguy/internal/unit"
)

func ruleEngine(rules ...clearance.PoleRule) *clearance.Engine {
	e := clearance.Empty()
	for _, r := range rules {
		e.RegisterPole(r)
	}
	return e
}

func TestEquipmentClearsWhenThereIsRoomOverIt(t *testing.T) {
	a := sampleAssembly()
	a.Add(att("A6", "RIDGEPORT CABLEVISION", plant.KindPowerSupply, 140))
	s := sampleSheet()
	s.Measurements = append(s.Measurements, measure("A6", 140, survey.MethodStick))
	got, err := ruleEngine(clearance.EquipmentRule{}).CheckPole(poleContext(t, s, a))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("four and a half feet under the cable clears thirty inches, got %v", got)
	}
}

func TestEquipmentTooCloseToTheCableAboveIt(t *testing.T) {
	a := sampleAssembly()
	a.Add(att("A6", "RIDGEPORT CABLEVISION", plant.KindPowerSupply, 168))
	s := sampleSheet()
	s.Measurements = append(s.Measurements, measure("A6", 168, survey.MethodStick))
	got, err := ruleEngine(clearance.EquipmentRule{}).CheckPole(poleContext(t, s, a))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("findings = %v", got)
	}
	f := got[0]
	if f.RuleID != "NESC-238A" {
		t.Errorf("rule = %s", f.RuleID)
	}
	if f.Subject != "A6" || f.Against != "A4" {
		t.Errorf("subject %s against %s", f.Subject, f.Against)
	}
	if f.Measured.Tenths() != 18 {
		t.Errorf("measured = %s", f.Measured)
	}
	if f.Required.Tenths() != unit.Inches(30).Tenths() {
		t.Errorf("required = %s", f.Required)
	}
}

func TestOnlyBoxesAreCheckedAsEquipment(t *testing.T) {
	a := sampleAssembly()
	a.Add(att("A6", "RIDGEPORT CABLEVISION", plant.KindCommDrop, 168))
	s := sampleSheet()
	s.Measurements = append(s.Measurements, measure("A6", 168, survey.MethodStick))
	got, err := ruleEngine(clearance.EquipmentRule{}).CheckPole(poleContext(t, s, a))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("a drop is not equipment, got %v", got)
	}
}

func TestEquipmentAtTheTopOfThePoleHasNothingAboveIt(t *testing.T) {
	a := plant.NewAssembly(samplePole(), []plant.Attachment{
		att("A6", "RIDGEPORT CABLEVISION", plant.KindEquipmentBox, 200),
	})
	s := survey.PoleSheet{
		ID: "SV-4", Pole: "NW-441829", Taken: sampleSheet().Taken, Crew: "CREW 41",
		Condition:    plant.ConditionSound,
		Measurements: []survey.Measurement{measure("A6", 200, survey.MethodStick)},
	}
	got, err := ruleEngine(clearance.EquipmentRule{}).CheckPole(poleContext(t, s, a))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("nothing above means nothing to clear, got %v", got)
	}
}

func TestEquipmentLeavesBlameToTheComparison(t *testing.T) {
	a := sampleAssembly()
	a.Add(att("A6", "MERIDIAN FIBER", plant.KindEquipmentBox, 168))
	s := sampleSheet()
	s.Measurements = append(s.Measurements, measure("A6", 168, survey.MethodStick))
	e := ruleEngine(clearance.EquipmentRule{})
	before, err := e.CheckPole(poleContext(t, sampleSheet(), sampleAssembly()))
	if err != nil {
		t.Fatal(err)
	}
	after, err := e.CheckPole(poleContext(t, s, a))
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 1 || after[0].Blame != clearance.BlameUnattributed {
		t.Fatalf("findings = %v", after)
	}
	attributed := clearance.Attribute(before, after)
	if attributed[0].Blame != clearance.BlameApplicant {
		t.Errorf("blame = %s", attributed[0].Blame)
	}
}

func TestARiserClearsWhenNothingSharesItsFace(t *testing.T) {
	a := sampleAssembly()
	a.Add(onFace(att("A6", "RIDGEPORT CABLEVISION", plant.KindRiser, 100), plant.FaceStreet))
	s := sampleSheet()
	s.Measurements = append(s.Measurements, measure("A6", 100, survey.MethodStick))
	got, err := ruleEngine(clearance.VerticalRunRule{}).CheckPole(poleContext(t, s, a))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("nothing else is on the street face, got %v", got)
	}
}

func TestARiserTooCloseToSomethingOnItsOwnFace(t *testing.T) {
	a := sampleAssembly()
	a.Add(onFace(att("A6", "RIDGEPORT CABLEVISION", plant.KindRiser, 100), plant.FaceStreet))
	a.Add(onFace(att("A7", "NORTHVALE TELEPHONE", plant.KindEquipmentBox, 101), plant.FaceStreet))
	s := sampleSheet()
	s.Measurements = append(s.Measurements,
		measure("A6", 100, survey.MethodStick),
		measure("A7", 101, survey.MethodStick),
	)
	got, err := ruleEngine(clearance.VerticalRunRule{}).CheckPole(poleContext(t, s, a))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("findings = %v", got)
	}
	f := got[0]
	if f.RuleID != "NESC-239E" || f.Subject != "A6" || f.Against != "A7" {
		t.Errorf("finding = %+v", f)
	}
	if f.Measured.Tenths() != 1 {
		t.Errorf("measured = %s", f.Measured)
	}
}

func TestARiserWithNoRecordedFaceIsNotChecked(t *testing.T) {
	a := sampleAssembly()
	a.Add(att("A6", "RIDGEPORT CABLEVISION", plant.KindRiser, 186))
	s := sampleSheet()
	s.Measurements = append(s.Measurements, measure("A6", 186, survey.MethodStick))
	got, err := ruleEngine(clearance.VerticalRunRule{}).CheckPole(poleContext(t, s, a))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("without a face there is nothing to measure against, got %v", got)
	}
}

func TestBothRulesRunTogether(t *testing.T) {
	a := sampleAssembly()
	a.Add(onFace(att("A6", "RIDGEPORT CABLEVISION", plant.KindPowerSupply, 168), plant.FaceStreet))
	a.Add(onFace(att("A7", "RIDGEPORT CABLEVISION", plant.KindRiser, 169), plant.FaceStreet))
	s := sampleSheet()
	s.Measurements = append(s.Measurements,
		measure("A6", 168, survey.MethodStick),
		measure("A7", 169, survey.MethodStick),
	)
	e := ruleEngine(clearance.EquipmentRule{}, clearance.VerticalRunRule{})
	ids := e.PoleRuleIDs()
	if len(ids) != 2 || ids[0] != "NESC-238A" || ids[1] != "NESC-239E" {
		t.Errorf("registered = %v", ids)
	}
	got, err := e.CheckPole(poleContext(t, s, a))
	if err != nil {
		t.Fatal(err)
	}
	rules := got.Rules()
	if len(rules) != 2 {
		t.Errorf("both rules should fire, got %v", got)
	}
}
