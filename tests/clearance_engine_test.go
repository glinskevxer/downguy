package tests

import (
	"testing"

	"github.com/glinskevxer/downguy/internal/clearance"
	"github.com/glinskevxer/downguy/internal/nesc"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/survey"
	"github.com/glinskevxer/downguy/internal/unit"
)

func context(t *testing.T, sheet survey.PoleSheet, a *plant.Assembly) clearance.PoleContext {
	t.Helper()
	applied, err := survey.Apply(sheet, a)
	if err != nil {
		t.Fatal(err)
	}
	return clearance.PoleContext{
		Applied: applied, District: nesc.Heavy, Grade: nesc.GradeC,
		Applicant: "MERIDIAN FIBER",
	}
}

func TestContextAccessors(t *testing.T) {
	c := context(t, sampleSheet(), sampleAssembly())
	if c.PoleID() != "NW-441829" {
		t.Errorf("pole = %q", c.PoleID())
	}
	if c.Assembly() == nil || len(c.Assembly().Attachments) != 4 {
		t.Error("the context should carry the surveyed assembly")
	}
	att, _ := c.Assembly().Find("A3")
	if c.IsApplicant(att) {
		t.Error("A3 belongs to the cable company, not the applicant")
	}
	att.Owner = "MERIDIAN FIBER"
	if !c.IsApplicant(att) {
		t.Error("a facility owned by the applicant is the applicant's")
	}
	c.Applicant = ""
	if c.IsApplicant(att) {
		t.Error("with no applicant named nothing is the applicant's")
	}
}

func TestContextValidate(t *testing.T) {
	c := context(t, sampleSheet(), sampleAssembly())
	if err := c.Validate(); err != nil {
		t.Fatalf("a good context failed: %v", err)
	}
	empty := clearance.PoleContext{District: nesc.Heavy, Grade: nesc.GradeC}
	if err := empty.Validate(); err == nil {
		t.Error("no assembly should be rejected")
	}
	bad := context(t, sampleSheet(), sampleAssembly())
	bad.District = nesc.District("tropical")
	if err := bad.Validate(); err == nil {
		t.Error("an unknown district should be rejected")
	}
	bad = context(t, sampleSheet(), sampleAssembly())
	bad.Grade = nesc.Grade("Z")
	if err := bad.Validate(); err == nil {
		t.Error("an unknown grade should be rejected")
	}
}

func TestEngineRegistration(t *testing.T) {
	e := clearance.New()
	ids := e.PoleRuleIDs()
	if len(ids) == 0 {
		t.Fatal("the standard engine should carry rules")
	}
	for _, id := range ids {
		if _, err := nesc.LookupRule(id); err != nil {
			t.Errorf("%s is registered but not catalogued", id)
		}
	}
	empty := clearance.Empty()
	if len(empty.PoleRuleIDs()) != 0 {
		t.Error("an empty engine carries nothing")
	}
	empty.RegisterPole(clearance.SafetyZoneRule{})
	if len(empty.PoleRuleIDs()) != 1 {
		t.Error("registration should add a rule")
	}
}

func TestSafetyZonePassesOnAWellBuiltPole(t *testing.T) {
	got, err := clearance.New().CheckPole(context(t, sampleSheet(), sampleAssembly()))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("an 8.5 foot safety zone should be clean, got %v", got)
	}
}

func TestSafetyZoneFailsWhenCommunicationIsTooHigh(t *testing.T) {
	s := sampleSheet()
	// Lift the cable to within two feet of the neutral.
	s.Measurements[0].Height = unit.Tenths(265)
	got, err := clearance.New().CheckPole(context(t, s, sampleAssembly()))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("findings = %v", got)
	}
	f := got[0]
	if f.RuleID != "NESC-235C1" {
		t.Errorf("rule = %s", f.RuleID)
	}
	if f.Severity != nesc.SeverityStop {
		t.Errorf("severity = %s", f.Severity)
	}
	if f.Subject != "A3" || f.Against != "A2" {
		t.Errorf("subject %s against %s", f.Subject, f.Against)
	}
	if f.Measured.Tenths() != 20 {
		t.Errorf("measured = %s", f.Measured)
	}
	if f.Required.Tenths() != unit.Inches(30).Tenths() {
		t.Errorf("required = %s, want the reduced neutral figure", f.Required)
	}
	if f.Owner != "RIDGEPORT CABLEVISION" {
		t.Errorf("owner = %q", f.Owner)
	}
	if f.Deficit().Tenths() != unit.Inches(30).Tenths()-20 {
		t.Errorf("deficit = %s", f.Deficit())
	}
}

func TestSafetyZoneUsesTheFullFigureWhenTheLowestSupplyIsNotTheNeutral(t *testing.T) {
	a := sampleAssembly()
	// Put a streetlight below the neutral, so the lowest supply facility is no
	// longer an effectively grounded neutral and the full forty inches apply.
	a.Add(att("A5", "SUMMIT VALLEY ELECTRIC", plant.KindStreetlight, 232))
	s := sampleSheet()
	s.Measurements = append(s.Measurements, measure("A5", 232, survey.MethodLaser))
	s.Measurements[0].Height = unit.Tenths(202)
	got, err := clearance.New().CheckPole(context(t, s, a))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("findings = %v", got)
	}
	if got[0].Against != "A5" {
		t.Errorf("the finding should be against the streetlight, got %s", got[0].Against)
	}
	if got[0].Required.Tenths() != unit.Inches(40).Tenths() {
		t.Errorf("required = %s, want the full forty inches", got[0].Required)
	}
}

func TestSafetyZoneDoesNotApplyToASingleUsePole(t *testing.T) {
	supplyOnly := plant.NewAssembly(samplePole(), []plant.Attachment{
		att("A1", "SUMMIT VALLEY ELECTRIC", plant.KindPrimary, 335),
	})
	s := survey.PoleSheet{
		ID: "SV-1", Pole: "NW-441829", Taken: sampleSheet().Taken, Crew: "CREW 41",
		Condition: plant.ConditionSound,
		Measurements: []survey.Measurement{
			measure("A1", 335, survey.MethodLaser),
		},
	}
	got, err := clearance.New().CheckPole(context(t, s, supplyOnly))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("a supply only pole has no safety zone to violate, got %v", got)
	}
}

func TestCheckPoleRejectsABadContext(t *testing.T) {
	c := context(t, sampleSheet(), sampleAssembly())
	c.District = nesc.District("tropical")
	if _, err := clearance.New().CheckPole(c); err == nil {
		t.Error("an unknown district should be rejected")
	}
}
