package tests

import (
	"testing"

	"github.com/glinskevxer/downguy/internal/clearance"
	"github.com/glinskevxer/downguy/internal/nesc"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/survey"
	"github.com/glinskevxer/downguy/internal/unit"
)

func climbEngine() *clearance.Engine {
	e := clearance.Empty()
	e.RegisterPole(clearance.ClimbingSpaceRule{})
	return e
}

func onFace(a plant.Attachment, f plant.Face) plant.Attachment {
	a.Face = f
	return a
}

func TestFaceParsingAndNames(t *testing.T) {
	for _, in := range []string{"climbing", "CLIMBING", " Climbing "} {
		got, err := plant.ParseFace(in)
		if err != nil || got != plant.FaceClimbing {
			t.Errorf("ParseFace(%q) = %q %v", in, got, err)
		}
	}
	got, err := plant.ParseFace("  ")
	if err != nil || got != plant.FaceUnknown {
		t.Errorf("an empty face should read as unknown, got %q %v", got, err)
	}
	if _, err := plant.ParseFace("underside"); err == nil {
		t.Error("there is no underside")
	}
	if plant.FaceClimbing.Name() != "climbing face" {
		t.Errorf("name = %q", plant.FaceClimbing.Name())
	}
	if plant.FaceUnknown.Name() != "face not recorded" {
		t.Errorf("unknown name = %q", plant.FaceUnknown.Name())
	}
	if plant.Face("underside").Name() != "unknown face underside" {
		t.Errorf("bad name = %q", plant.Face("underside").Name())
	}
	if !plant.KnownFace(plant.FaceStreet) || plant.KnownFace(plant.FaceUnknown) {
		t.Error("known face check")
	}
}

func TestOnlyTheClimbingFaceObstructs(t *testing.T) {
	if !plant.FaceClimbing.Obstructs() {
		t.Error("the climbing face obstructs")
	}
	for _, f := range []plant.Face{plant.FaceStreet, plant.FaceField, plant.FaceRoad, plant.FaceUnknown} {
		if f.Obstructs() {
			t.Errorf("%s does not obstruct", f)
		}
	}
}

func TestAttachmentValidateRejectsABadFace(t *testing.T) {
	a := sampleAttachment()
	a.Face = plant.Face("underside")
	if err := a.Validate(); err == nil {
		t.Error("an unknown face should be rejected")
	}
	a.Face = plant.FaceField
	if err := a.Validate(); err != nil {
		t.Errorf("a real face should be accepted: %v", err)
	}
	a.Face = plant.FaceUnknown
	if err := a.Validate(); err != nil {
		t.Errorf("an unrecorded face should be accepted: %v", err)
	}
}

func TestClimbingSpaceClearOnAnOrdinaryPole(t *testing.T) {
	got, err := climbEngine().CheckPole(context(t, sampleSheet(), sampleAssembly()))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("nothing is bolted to the climbing face, got %v", got)
	}
}

func TestAPowerSupplyOnTheClimbingFaceFails(t *testing.T) {
	a := sampleAssembly()
	a.Add(onFace(att("A6", "RIDGEPORT CABLEVISION", plant.KindPowerSupply, 140), plant.FaceClimbing))
	s := sampleSheet()
	s.Measurements = append(s.Measurements, measure("A6", 140, survey.MethodStick))
	got, err := climbEngine().CheckPole(context(t, s, a))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("findings = %v", got)
	}
	f := got[0]
	if f.RuleID != "NESC-236A" || f.Severity != nesc.SeverityStop {
		t.Errorf("finding = %+v", f)
	}
	if f.Subject != "A6" || f.Owner != "RIDGEPORT CABLEVISION" {
		t.Errorf("subject %s owner %q", f.Subject, f.Owner)
	}
	if f.Required.Tenths() != unit.Inches(30).Tenths() {
		t.Errorf("required = %s", f.Required)
	}
	if f.Blame != clearance.BlameUnattributed {
		t.Errorf("a rule does not decide blame, got %s", f.Blame)
	}
}

func TestAWireAcrossTheClimbingFaceIsFine(t *testing.T) {
	a := sampleAssembly()
	a.Add(onFace(att("A6", "MERIDIAN FIBER", plant.KindFiber, 174), plant.FaceClimbing))
	s := sampleSheet()
	s.Measurements = append(s.Measurements, measure("A6", 174, survey.MethodStick))
	got, err := climbEngine().CheckPole(context(t, s, a))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("a wire is climbed past, not around, got %v", got)
	}
}

func TestAnObstructionBelowTheFloorIsNotAViolation(t *testing.T) {
	a := sampleAssembly()
	a.Add(onFace(att("A6", "RIDGEPORT CABLEVISION", plant.KindEquipmentBox, 60), plant.FaceClimbing))
	s := sampleSheet()
	s.Measurements = append(s.Measurements, measure("A6", 60, survey.MethodStick))
	got, err := climbEngine().CheckPole(context(t, s, a))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("six feet up is below the climbing square, got %v", got)
	}
	s.Measurements[len(s.Measurements)-1].Height = clearance.ClimbingSpaceFloor
	got, err = climbEngine().CheckPole(context(t, s, a))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Errorf("the floor itself is inside the square, got %v", got)
	}
}

func TestClimbingSpaceOnTheApplicantsOwnBoxIsAttributedByComparison(t *testing.T) {
	a := sampleAssembly()
	a.Add(onFace(att("A6", "MERIDIAN FIBER", plant.KindEquipmentBox, 150), plant.FaceClimbing))
	s := sampleSheet()
	s.Measurements = append(s.Measurements, measure("A6", 150, survey.MethodStick))
	before, err := climbEngine().CheckPole(context(t, sampleSheet(), sampleAssembly()))
	if err != nil {
		t.Fatal(err)
	}
	after, err := climbEngine().CheckPole(context(t, s, a))
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 1 {
		t.Fatalf("findings = %v", after)
	}
	attributed := clearance.Attribute(before, after)
	if attributed[0].Blame != clearance.BlameApplicant {
		t.Errorf("blame = %s", attributed[0].Blame)
	}
	if attributed[0].Owner != "MERIDIAN FIBER" {
		t.Errorf("owner = %q", attributed[0].Owner)
	}
}

func TestObstructionsOnAFace(t *testing.T) {
	a := sampleAssembly()
	a.Add(onFace(att("A6", "RIDGEPORT CABLEVISION", plant.KindEquipmentBox, 150), plant.FaceClimbing))
	a.Add(onFace(att("A7", "RIDGEPORT CABLEVISION", plant.KindPowerSupply, 120), plant.FaceClimbing))
	a.Add(onFace(att("A8", "SUMMIT VALLEY ELECTRIC", plant.KindTransformer, 250), plant.FaceStreet))
	climbing := clearance.ObstructionsOn(a, plant.FaceClimbing)
	if len(climbing) != 2 {
		t.Fatalf("climbing face = %v", climbing)
	}
	if climbing[0].ID != "A6" || climbing[1].ID != "A7" {
		t.Errorf("obstructions should come back top down, got %v", climbing)
	}
	if got := len(clearance.ObstructionsOn(a, plant.FaceStreet)); got != 1 {
		t.Errorf("street face = %d", got)
	}
	if got := len(clearance.ObstructionsOn(a, plant.FaceRoad)); got != 0 {
		t.Errorf("road face = %d", got)
	}
}
