package tests

import (
	"testing"

	"github.com/glinskevxer/downguy/internal/clearance"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/survey"
	"github.com/glinskevxer/downguy/internal/unit"
)

func commEngine() *clearance.Engine {
	e := clearance.Empty()
	e.RegisterPole(clearance.CommSeparationRule{})
	return e
}

func TestSameAttacherFolding(t *testing.T) {
	if !clearance.SameAttacher("MERIDIAN FIBER", "  meridian   fiber ") {
		t.Error("the same company written two ways is one company")
	}
	if clearance.SameAttacher("MERIDIAN FIBER", "NORTHVALE TELEPHONE") {
		t.Error("two companies are two companies")
	}
	if clearance.SameAttacher("", "") {
		t.Error("two blanks are not the same attacher")
	}
	if clearance.SameAttacher("  ", "MERIDIAN FIBER") {
		t.Error("a blank matches nothing")
	}
}

func TestCommSeparationPassesOnAWellBuiltPole(t *testing.T) {
	got, err := commEngine().CheckPole(context(t, sampleSheet(), sampleAssembly()))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("eight tenths between two attachers clears twelve inches, got %v", got)
	}
}

func TestCommSeparationFailsBetweenTwoAttachers(t *testing.T) {
	s := sampleSheet()
	s.Measurements[2].Height = unit.Tenths(196)
	got, err := commEngine().CheckPole(context(t, s, sampleAssembly()))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("findings = %v", got)
	}
	f := got[0]
	if f.RuleID != "NESC-235C2" {
		t.Errorf("rule = %s", f.RuleID)
	}
	if f.Subject != "A4" || f.Against != "A3" {
		t.Errorf("subject %s against %s", f.Subject, f.Against)
	}
	if f.Required.Tenths() != unit.Inches(12).Tenths() {
		t.Errorf("required = %s", f.Required)
	}
	if f.Measured.Tenths() != 4 {
		t.Errorf("measured = %s", f.Measured)
	}
	if f.Owner != "NORTHVALE TELEPHONE" {
		t.Errorf("owner = %q", f.Owner)
	}
	if f.Blame != clearance.BlamePreExisting {
		t.Errorf("blame = %s", f.Blame)
	}
}

func TestOneAttacherMaySitCloserToItself(t *testing.T) {
	a := plant.NewAssembly(samplePole(), []plant.Attachment{
		att("A1", "SUMMIT VALLEY ELECTRIC", plant.KindPrimary, 335),
		att("A3", "RIDGEPORT CABLEVISION", plant.KindCommCable, 200),
		att("A4", "ridgeport  cablevision", plant.KindFiber, 196),
	})
	s := survey.PoleSheet{
		ID: "SV-2", Pole: "NW-441829", Taken: sampleSheet().Taken, Crew: "CREW 41",
		Condition: plant.ConditionSound,
		Measurements: []survey.Measurement{
			measure("A1", 335, survey.MethodLaser),
			measure("A3", 200, survey.MethodLaser),
			measure("A4", 196, survey.MethodLaser),
		},
	}
	got, err := commEngine().CheckPole(context(t, s, a))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("four tenths clears the four inch same owner figure, got %v", got)
	}
	s.Measurements[2].Height = unit.Tenths(198)
	got, err = commEngine().CheckPole(context(t, s, a))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("two tenths does not, got %v", got)
	}
	if got[0].Required.Tenths() != unit.Inches(4).Tenths() {
		t.Errorf("required = %s, want the same owner figure", got[0].Required)
	}
}

func TestCommSeparationOnlyComparesNeighbours(t *testing.T) {
	a := sampleAssembly()
	a.Add(att("A5", "MERIDIAN FIBER", plant.KindFiber, 174))
	s := sampleSheet()
	s.Measurements = append(s.Measurements, measure("A5", 174, survey.MethodLaser))
	got, err := commEngine().CheckPole(context(t, s, a))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("three facilities a foot apart all clear, got %v", got)
	}
	// Now bunch the bottom two. Only that pair should be reported, not the
	// cable three feet above them.
	s.Measurements[len(s.Measurements)-1].Height = unit.Tenths(182)
	got, err = commEngine().CheckPole(context(t, s, a))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("findings = %v", got)
	}
	if got[0].Subject != "A5" || got[0].Against != "A4" {
		t.Errorf("finding = %s against %s", got[0].Subject, got[0].Against)
	}
}

func TestCommSeparationBlamesTheApplicant(t *testing.T) {
	a := sampleAssembly()
	a.Add(att("A5", "MERIDIAN FIBER", plant.KindFiber, 182))
	s := sampleSheet()
	s.Measurements = append(s.Measurements, measure("A5", 182, survey.MethodLaser))
	got, err := commEngine().CheckPole(context(t, s, a))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("findings = %v", got)
	}
	if got[0].Blame != clearance.BlameApplicant {
		t.Errorf("blame = %s", got[0].Blame)
	}
	if got[0].Owner != "MERIDIAN FIBER" {
		t.Errorf("owner = %q", got[0].Owner)
	}
}

func TestCommSeparationNeedsTwoFacilities(t *testing.T) {
	a := plant.NewAssembly(samplePole(), []plant.Attachment{
		att("A1", "SUMMIT VALLEY ELECTRIC", plant.KindPrimary, 335),
		att("A3", "RIDGEPORT CABLEVISION", plant.KindCommCable, 200),
	})
	s := survey.PoleSheet{
		ID: "SV-3", Pole: "NW-441829", Taken: sampleSheet().Taken, Crew: "CREW 41",
		Condition: plant.ConditionSound,
		Measurements: []survey.Measurement{
			measure("A1", 335, survey.MethodLaser),
			measure("A3", 200, survey.MethodLaser),
		},
	}
	got, err := commEngine().CheckPole(context(t, s, a))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("one communication facility has nothing to be separated from, got %v", got)
	}
}
