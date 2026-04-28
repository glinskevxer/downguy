package tests

import (
	"testing"

	"github.com/glinskevxer/downguy/internal/clearance"
	"github.com/glinskevxer/downguy/internal/nesc"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/survey"
)

func gradeEngine(grade nesc.Grade) *clearance.Engine {
	e := clearance.Empty()
	e.RegisterSpan(clearance.GradeRule{Grade: grade})
	return e
}

func railroadSheet() survey.SpanSheet {
	sheet := sampleSpanSheet()
	sheet.Stations = []survey.Station{{
		Feet: 100, Cover: plant.CoverRailroad, Measurements: []survey.Measurement{
			measure("A1", 296, survey.MethodLaser),
			measure("A3", 260, survey.MethodLaser),
		},
	}}
	return sheet
}

func TestGradeRuleFiresOverARailroadAtGradeC(t *testing.T) {
	t.Parallel()
	got, err := gradeEngine(nesc.GradeC).CheckSpan(spanContext(railroadSheet()))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("findings = %v", got)
	}
	f := got[0]
	if f.RuleID != "NESC-242" || f.Severity != nesc.SeverityStop {
		t.Errorf("finding = %+v", f)
	}
	if f.Span != "SS-330" {
		t.Errorf("span = %q", f.Span)
	}
	if f.Against != "crosses a railroad" {
		t.Errorf("against = %q", f.Against)
	}
	if f.Blame != clearance.BlameUnattributed {
		t.Errorf("blame = %s", f.Blame)
	}
}

func TestGradeRuleIsSatisfiedAtGradeB(t *testing.T) {
	t.Parallel()
	got, err := gradeEngine(nesc.GradeB).CheckSpan(spanContext(railroadSheet()))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("grade B is what a railroad crossing wants, got %v", got)
	}
}

func TestSupplyOverCommunicationWantsGradeB(t *testing.T) {
	t.Parallel()
	// The fixture span carries the primary and two communication cables over
	// an ordinary road, which is supply over communication.
	got, err := gradeEngine(nesc.GradeC).CheckSpan(spanContext(sampleSpanSheet()))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("findings = %v", got)
	}
	if got[0].Against != "carries supply over communication" {
		t.Errorf("against = %q", got[0].Against)
	}
	clean, err := gradeEngine(nesc.GradeB).CheckSpan(spanContext(sampleSpanSheet()))
	if err != nil {
		t.Fatal(err)
	}
	if len(clean) != 0 {
		t.Errorf("grade B satisfies it, got %v", clean)
	}
}

func TestGradeRuleIgnoresACommunicationOnlySpan(t *testing.T) {
	t.Parallel()
	sheet := sampleSpanSheet()
	sheet.Stations = []survey.Station{{
		Feet: 100, Cover: plant.CoverField, Measurements: []survey.Measurement{
			measure("A3", 200, survey.MethodLaser),
			measure("A4", 186, survey.MethodLaser),
		},
	}}
	got, err := gradeEngine(nesc.GradeN).CheckSpan(spanContext(sheet))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("a communication only span over a field asks for nothing, got %v", got)
	}
}

func TestGradeRuleWithNoGradeSetDoesNothing(t *testing.T) {
	t.Parallel()
	got, err := gradeEngine(nesc.Grade("")).CheckSpan(spanContext(railroadSheet()))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("with no grade to compare against there is nothing to say, got %v", got)
	}
	if nesc.KnownGradeForCheck(nesc.Grade("")) || !nesc.KnownGradeForCheck(nesc.GradeB) {
		t.Error("known grade check")
	}
}

func TestGradeRuleSkipsAReadingItCannotResolve(t *testing.T) {
	t.Parallel()
	sheet := sampleSpanSheet()
	sheet.Stations = []survey.Station{{
		Feet: 100, Cover: plant.CoverField, Measurements: []survey.Measurement{
			measure("A9", 200, survey.MethodLaser),
		},
	}}
	got, err := gradeEngine(nesc.GradeN).CheckSpan(spanContext(sheet))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("nothing on the span resolves, so nothing is known about it, got %v", got)
	}
}

func TestTheGradeRuleCitesACatalogueEntry(t *testing.T) {
	t.Parallel()
	rule, err := nesc.LookupRule(clearance.GradeRule{}.ID())
	if err == nil && rule.Section != "242" {
		t.Errorf("rule = %+v", rule)
	}
	catalogued, err := nesc.LookupRule("NESC-242")
	if err != nil {
		t.Fatalf("the rule the finding cites is not catalogued: %v", err)
	}
	if catalogued.Severity != nesc.SeverityStop || catalogued.Applies == "" {
		t.Errorf("rule = %+v", catalogued)
	}
}
