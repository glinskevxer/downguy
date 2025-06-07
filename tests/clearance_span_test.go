package tests

import (
	"testing"

	"github.com/glinskevxer/downguy/internal/clearance"
	"github.com/glinskevxer/downguy/internal/nesc"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/survey"
	"github.com/glinskevxer/downguy/internal/unit"
)

func spanContext(sheet survey.SpanSheet) clearance.SpanContext {
	return clearance.SpanContext{
		Sheet: sheet, Near: sampleAssembly(),
		District: nesc.Heavy, Applicant: "MERIDIAN FIBER",
	}
}

func groundEngine() *clearance.Engine {
	e := clearance.Empty()
	e.RegisterSpan(clearance.GroundClearanceRule{})
	return e
}

func TestSpanContextValidate(t *testing.T) {
	c := spanContext(sampleSpanSheet())
	if err := c.Validate(); err != nil {
		t.Fatalf("a good span context failed: %v", err)
	}
	bad := c
	bad.Near = nil
	if err := bad.Validate(); err == nil {
		t.Error("no near assembly should be rejected")
	}
	bad = c
	bad.Sheet.FromPole = "NW-441830"
	if err := bad.Validate(); err == nil {
		t.Error("a sheet for another pole should be rejected")
	}
	bad = c
	bad.District = nesc.District("tropical")
	if err := bad.Validate(); err == nil {
		t.Error("an unknown district should be rejected")
	}
	bad = c
	bad.Sheet.Crew = ""
	if err := bad.Validate(); err == nil {
		t.Error("an invalid sheet should be rejected")
	}
}

func TestSpanContextResolvesFacilities(t *testing.T) {
	c := spanContext(sampleSpanSheet())
	att, ok := c.Facility("A1")
	if !ok || att.Kind != plant.KindPrimary {
		t.Errorf("A1 = %+v ok=%v", att, ok)
	}
	if _, ok := c.Facility("A9"); ok {
		t.Error("A9 is not on the pole")
	}
	empty := clearance.SpanContext{}
	if _, ok := empty.Facility("A1"); ok {
		t.Error("with no assembly nothing resolves")
	}
	fiber := att
	fiber.Owner = "  meridian fiber "
	if !c.IsApplicant(fiber) {
		t.Error("the applicant's own facility should be recognised whatever the spacing")
	}
	if c.IsApplicant(att) {
		t.Error("the utility's primary is not the applicant's")
	}
}

func TestGroundClearanceClearsOnAGoodSpan(t *testing.T) {
	got, err := groundEngine().CheckSpan(spanContext(sampleSpanSheet()))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("the fixture span should clear, got %v", got)
	}
}

func TestGroundClearanceFailsOverARoad(t *testing.T) {
	sheet := sampleSpanSheet()
	// Drop the cable at the road crossing to under fifteen and a half feet.
	sheet.Stations[0].Measurements[1].Height = unit.Tenths(148)
	got, err := groundEngine().CheckSpan(spanContext(sheet))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("findings = %v", got)
	}
	f := got[0]
	if f.RuleID != "NESC-232A" {
		t.Errorf("rule = %s", f.RuleID)
	}
	if f.Subject != "A3" || f.Against != string(plant.CoverRoad) {
		t.Errorf("subject %s against %s", f.Subject, f.Against)
	}
	if f.Required.Tenths() != 155 {
		t.Errorf("required = %s", f.Required)
	}
	if f.Span != "SS-330" {
		t.Errorf("span = %q", f.Span)
	}
	if f.Owner != "RIDGEPORT CABLEVISION" {
		t.Errorf("owner = %q", f.Owner)
	}
}

func TestGroundClearanceUsesTheCoverAtEachStation(t *testing.T) {
	sheet := sampleSpanSheet()
	// 12.5 feet clears an open field for communication and fails a road.
	for i := range sheet.Stations {
		sheet.Stations[i].Measurements[1].Height = unit.Tenths(125)
	}
	got, err := groundEngine().CheckSpan(spanContext(sheet))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("the field station should clear and the other two fail, got %v", got)
	}
	for _, f := range got {
		if f.Against == string(plant.CoverField) {
			t.Error("the field station should not have fired")
		}
	}
}

func TestGroundClearanceFindingRisesWithVoltage(t *testing.T) {
	sheet := sampleSpanSheet()
	sheet.Stations[0].Measurements[0].Height = unit.Tenths(170)
	got, err := groundEngine().CheckSpan(spanContext(sheet))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("findings = %v", got)
	}
	if got[0].Subject != "A1" || got[0].Required.Tenths() != 185 {
		t.Errorf("finding = %+v", got[0])
	}
}

func TestGroundClearanceSkipsWhatItCannotResolve(t *testing.T) {
	sheet := sampleSpanSheet()
	sheet.Stations[0].Measurements = append(sheet.Stations[0].Measurements,
		measure("A9", 30, survey.MethodLaser))
	got, err := groundEngine().CheckSpan(spanContext(sheet))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("a reading of something not on the pole should be skipped, got %v", got)
	}
}

func TestAMarginalReadingIsNotAViolation(t *testing.T) {
	sheet := sampleSpanSheet()
	// One tenth under the requirement, read by laser with a one tenth band.
	sheet.Stations[0].Measurements[1].Height = unit.Tenths(154)
	got, err := groundEngine().CheckSpan(spanContext(sheet))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("a reading inside its own tolerance is not a violation, got %v", got)
	}
	sheet.Stations[0].Measurements[1].Height = unit.Tenths(153)
	got, err = groundEngine().CheckSpan(spanContext(sheet))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Errorf("two tenths under a one tenth band is, got %v", got)
	}
}

func TestStandardEngineRegistersBothSides(t *testing.T) {
	e := clearance.New()
	if len(e.PoleRuleIDs()) != 5 {
		t.Errorf("pole rules = %v", e.PoleRuleIDs())
	}
	if len(e.SpanRuleIDs()) < 1 {
		t.Errorf("span rules = %v", e.SpanRuleIDs())
	}
	for _, id := range append(e.PoleRuleIDs(), e.SpanRuleIDs()...) {
		if _, err := nesc.LookupRule(id); err != nil {
			t.Errorf("%s is registered but not catalogued", id)
		}
	}
	if len(clearance.Empty().SpanRuleIDs()) != 0 {
		t.Error("an empty engine carries no span rules")
	}
}

func TestCheckSpanRejectsABadContext(t *testing.T) {
	c := spanContext(sampleSpanSheet())
	c.Near = nil
	if _, err := groundEngine().CheckSpan(c); err == nil {
		t.Error("a bad context should be rejected")
	}
}
