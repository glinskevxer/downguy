package tests

import (
	"testing"

	"github.com/glinskevxer/downguy/internal/clearance"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/survey"
	"github.com/glinskevxer/downguy/internal/unit"
)

func midspanEngine() *clearance.Engine {
	e := clearance.Empty()
	e.RegisterSpan(clearance.MidspanRule{})
	return e
}

func TestMidspanClearsOnAGoodSpan(t *testing.T) {
	got, err := midspanEngine().CheckSpan(spanContext(sampleSpanSheet()))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("twelve feet between the primary and the cable clears, got %v", got)
	}
}

func TestMidspanFailsWhenSupplyClosesOnCommunication(t *testing.T) {
	sheet := sampleSpanSheet()
	// Let the primary sag down onto the cable at the road crossing.
	sheet.Stations[0].Measurements[0].Height = unit.Tenths(196)
	got, err := midspanEngine().CheckSpan(spanContext(sheet))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("findings = %v", got)
	}
	f := got[0]
	if f.RuleID != "NESC-235E" {
		t.Errorf("rule = %s", f.RuleID)
	}
	if f.Subject != "A3" || f.Against != "A1" {
		t.Errorf("subject %s against %s", f.Subject, f.Against)
	}
	if f.Required.Tenths() != unit.Inches(30).Tenths() {
		t.Errorf("required = %s, want the supply figure", f.Required)
	}
	if f.Measured.Tenths() != 20 {
		t.Errorf("measured = %s", f.Measured)
	}
	if f.Span != "SS-330" {
		t.Errorf("span = %q", f.Span)
	}
}

func TestMidspanUsesTheCommunicationFigureBetweenTwoCables(t *testing.T) {
	sheet := sampleSpanSheet()
	sheet.Stations[0].Measurements = []survey.Measurement{
		measure("A3", 176, survey.MethodLaser),
		measure("A4", 172, survey.MethodLaser),
	}
	got, err := midspanEngine().CheckSpan(spanContext(sheet))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("findings = %v", got)
	}
	if got[0].Required.Tenths() != unit.Inches(12).Tenths() {
		t.Errorf("required = %s, want the communication figure", got[0].Required)
	}
	if got[0].Owner != "NORTHVALE TELEPHONE" {
		t.Errorf("owner = %q", got[0].Owner)
	}
}

func TestMidspanIgnoresOneAttachersOwnPair(t *testing.T) {
	a := plant.NewAssembly(samplePole(), []plant.Attachment{
		att("A3", "RIDGEPORT CABLEVISION", plant.KindCommCable, 200),
		att("A4", "ridgeport cablevision", plant.KindFiber, 186),
	})
	sheet := sampleSpanSheet()
	sheet.Stations = []survey.Station{{
		Feet: 100, Cover: plant.CoverField, Measurements: []survey.Measurement{
			measure("A3", 176, survey.MethodLaser),
			measure("A4", 174, survey.MethodLaser),
		},
	}}
	c := spanContext(sheet)
	c.Near = a
	got, err := midspanEngine().CheckSpan(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("one attacher's own two cables are its own business, got %v", got)
	}
}

func TestMidspanBlamesTheApplicant(t *testing.T) {
	a := sampleAssembly()
	a.Add(att("A5", "MERIDIAN FIBER", plant.KindFiber, 174))
	sheet := sampleSpanSheet()
	sheet.Stations = []survey.Station{{
		Feet: 100, Cover: plant.CoverField, Measurements: []survey.Measurement{
			measure("A4", 170, survey.MethodLaser),
			measure("A5", 168, survey.MethodLaser),
		},
	}}
	c := spanContext(sheet)
	c.Near = a
	got, err := midspanEngine().CheckSpan(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("findings = %v", got)
	}
	if got[0].Blame != clearance.BlameApplicant || got[0].Owner != "MERIDIAN FIBER" {
		t.Errorf("finding = %+v", got[0])
	}
}

func TestMidspanNeedsTwoResolvableReadings(t *testing.T) {
	sheet := sampleSpanSheet()
	sheet.Stations = []survey.Station{{
		Feet: 100, Cover: plant.CoverField, Measurements: []survey.Measurement{
			measure("A3", 176, survey.MethodLaser),
			measure("A9", 175, survey.MethodLaser),
		},
	}}
	got, err := midspanEngine().CheckSpan(spanContext(sheet))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("only one reading resolves, so there is no pair, got %v", got)
	}
}

func TestTightestPairAcrossTheSpan(t *testing.T) {
	sheet := sampleSpanSheet()
	gap, at, ok := clearance.TightestPair(spanContext(sheet))
	if !ok {
		t.Fatal("the fixture has pairs at every station")
	}
	if at != 100 {
		t.Errorf("tightest at %d feet, want the road crossing", at)
	}
	if gap.Tenths() != 120 {
		t.Errorf("tightest gap = %s", gap)
	}
	single := sampleSpanSheet()
	single.Stations = []survey.Station{{
		Feet: 100, Cover: plant.CoverField, Measurements: []survey.Measurement{
			measure("A3", 176, survey.MethodLaser),
		},
	}}
	if _, _, ok := clearance.TightestPair(spanContext(single)); ok {
		t.Error("one reading is not a pair")
	}
}

func TestStandardEngineCarriesBothSpanRules(t *testing.T) {
	ids := clearance.New().SpanRuleIDs()
	if len(ids) != 2 || ids[0] != "NESC-232A" || ids[1] != "NESC-235E" {
		t.Errorf("span rules = %v", ids)
	}
}
