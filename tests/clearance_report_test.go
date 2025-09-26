package tests

import (
	"testing"

	"github.com/glinskevxer/downguy/internal/clearance"
	"github.com/glinskevxer/downguy/internal/nesc"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/survey"
	"github.com/glinskevxer/downguy/internal/unit"
)

func builder() *clearance.Builder {
	return clearance.NewBuilder(clearance.New(), "APP-1042", "MERIDIAN FIBER", nesc.Heavy, nesc.GradeC)
}

func TestReportOnACleanApplication(t *testing.T) {
	b := builder()
	if err := b.AddPole(poleContext(t, sampleSheet(), sampleAssembly()), poleContext(t, sampleSheet(), sampleAssembly())); err != nil {
		t.Fatal(err)
	}
	if err := b.AddSpan(spanContext(sampleSpanSheet()), spanContext(sampleSpanSheet())); err != nil {
		t.Fatal(err)
	}
	r := b.Build()
	if !r.Clean() {
		t.Errorf("a clean application should be clean, got %v", r.Attribution.All())
	}
	if r.Blocking() {
		t.Error("nothing should be blocking")
	}
	if len(r.Poles) != 1 || r.Poles[0] != "NW-441829" {
		t.Errorf("poles = %v", r.Poles)
	}
	if len(r.Spans) != 1 || r.Spans[0] != "SS-330" {
		t.Errorf("spans = %v", r.Spans)
	}
	want := "APP-1042 for MERIDIAN FIBER: 1 poles, 1 spans, 0 caused, 0 worsened, 0 pre existing, 0 resolved, 0 skipped"
	if r.Summary() != want {
		t.Errorf("Summary = %q, want %q", r.Summary(), want)
	}
}

func TestReportBlamesTheApplicantForWhatItAdds(t *testing.T) {
	after := sampleAssembly()
	after.Add(att("A7", "MERIDIAN FIBER", plant.KindFiber, 182))
	afterSheet := sampleSheet()
	afterSheet.Measurements = append(afterSheet.Measurements, measure("A7", 182, survey.MethodLaser))

	b := builder()
	if err := b.AddPole(poleContext(t, sampleSheet(), sampleAssembly()), poleContext(t, afterSheet, after)); err != nil {
		t.Fatal(err)
	}
	r := b.Build()
	if r.Clean() {
		t.Fatal("adding a fiber four tenths under the telephone cable should fire")
	}
	if len(r.Attribution.Applicant) != 1 {
		t.Errorf("applicant findings = %v", r.Attribution.Applicant)
	}
	if len(r.Attribution.PreExisting) != 0 {
		t.Errorf("pre existing = %v", r.Attribution.PreExisting)
	}
	if r.Attribution.Applicant[0].RuleID != "NESC-235C2" {
		t.Errorf("rule = %s", r.Attribution.Applicant[0].RuleID)
	}
}

func TestReportKeepsAPreExistingViolationOffTheApplicant(t *testing.T) {
	before := sampleSheet()
	before.Measurements[2].Height = unit.Tenths(196)
	after := before
	after.Measurements = append([]survey.Measurement{}, before.Measurements...)

	b := builder()
	if err := b.AddPole(poleContext(t, before, sampleAssembly()), poleContext(t, after, sampleAssembly())); err != nil {
		t.Fatal(err)
	}
	r := b.Build()
	if len(r.Attribution.PreExisting) != 1 {
		t.Errorf("pre existing = %v", r.Attribution.All())
	}
	if len(r.Attribution.ApplicantsShare()) != 0 {
		t.Errorf("the applicant owes nothing here, got %v", r.Attribution.ApplicantsShare())
	}
}

func TestReportRecordsAPrediction(t *testing.T) {
	p := sampleProposal()
	p.Cover = plant.CoverRoad
	p.NearHeight = unit.Tenths(160)
	p.FarHeight = unit.Tenths(160)
	pred, err := clearance.Predict(p)
	if err != nil {
		t.Fatal(err)
	}
	b := builder()
	b.AddPrediction(pred, "SP-1")
	r := b.Build()
	if len(r.Attribution.Applicant) != 1 {
		t.Fatalf("applicant findings = %v", r.Attribution.All())
	}
	if r.Attribution.Applicant[0].Subject != "A7" {
		t.Errorf("subject = %s", r.Attribution.Applicant[0].Subject)
	}
	if len(r.Spans) != 1 || r.Spans[0] != "SP-1" {
		t.Errorf("spans = %v", r.Spans)
	}
}

func TestAClearingPredictionAddsNothing(t *testing.T) {
	pred, err := clearance.Predict(sampleProposal())
	if err != nil {
		t.Fatal(err)
	}
	b := builder()
	b.AddPrediction(pred, "SP-1")
	b.AddPrediction(nil, "SP-2")
	r := b.Build()
	if !r.Clean() {
		t.Errorf("a clearing prediction should add nothing, got %v", r.Attribution.All())
	}
	if len(r.Spans) != 1 || r.Spans[0] != "SP-1" {
		t.Errorf("a nil prediction is not a checked span, got %v", r.Spans)
	}
}

func TestReportSkips(t *testing.T) {
	b := builder()
	b.Skip("NW-441840", "no survey on file")
	b.Skip("NW-441835", "access denied by property owner")
	bad := poleContext(t, sampleSheet(), sampleAssembly())
	bad.District = nesc.District("tropical")
	if err := b.AddPole(bad, bad); err == nil {
		t.Error("a bad context should be an error")
	}
	r := b.Build()
	if len(r.Skipped) != 3 {
		t.Fatalf("skipped = %v", r.Skipped)
	}
	if r.Skipped[0].Where != "NW-441829" {
		t.Errorf("skips should be sorted, got %v", r.Skipped)
	}
	if len(r.Poles) != 0 {
		t.Errorf("a pole that could not be checked is not a checked pole, got %v", r.Poles)
	}
}

func TestBuilderSealsAfterBuild(t *testing.T) {
	b := builder()
	if err := b.AddPole(poleContext(t, sampleSheet(), sampleAssembly()), poleContext(t, sampleSheet(), sampleAssembly())); err != nil {
		t.Fatal(err)
	}
	first := b.Build()
	if err := b.AddPole(poleContext(t, sampleSheet(), sampleAssembly()), poleContext(t, sampleSheet(), sampleAssembly())); err == nil {
		t.Error("adding after building should be refused")
	}
	if err := b.AddSpan(spanContext(sampleSpanSheet()), spanContext(sampleSpanSheet())); err == nil {
		t.Error("adding a span after building should be refused")
	}
	second := b.Build()
	if first.Summary() != second.Summary() {
		t.Error("building twice should give the same report")
	}
}

func TestReportByOwner(t *testing.T) {
	after := sampleAssembly()
	after.Add(att("A7", "MERIDIAN FIBER", plant.KindFiber, 182))
	afterSheet := sampleSheet()
	afterSheet.Measurements = append(afterSheet.Measurements, measure("A7", 182, survey.MethodLaser))
	b := builder()
	if err := b.AddPole(poleContext(t, sampleSheet(), sampleAssembly()), poleContext(t, afterSheet, after)); err != nil {
		t.Fatal(err)
	}
	byOwner := b.Build().ByOwner()
	if len(byOwner["MERIDIAN FIBER"]) != 1 {
		t.Errorf("by owner = %v", byOwner)
	}
	if len(byOwner["nobody"]) != 0 {
		t.Errorf("by owner = %v", byOwner)
	}
}
