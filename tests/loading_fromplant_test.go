package tests

import (
	"testing"

	"github.com/glinskevxer/downguy/internal/loading"
	"github.com/glinskevxer/downguy/internal/nesc"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/unit"
)

func sampleAssignment() loading.Assignment {
	return loading.Assignment{
		"A1": "2 ACSR SPARROW",
		"A2": "2 ACSR SPARROW",
		"A3": "3/8 EHS MESSENGER",
		"A4": "1/4 EHS MESSENGER",
	}
}

func TestBuildWiresFromThePlantRecords(t *testing.T) {
	a := sampleAssembly()
	line := sampleLine(t)
	wires, err := loading.BuildWires(a, line, sampleAssignment(), nesc.Heavy)
	if err != nil {
		t.Fatal(err)
	}
	if len(wires) != 4 {
		t.Fatalf("wires = %d", len(wires))
	}
	for i := 1; i < len(wires); i++ {
		if wires[i].Height > wires[i-1].Height {
			t.Fatal("wires should come back highest first")
		}
	}
	// The first pole of the section carries one span, so the wind span is half
	// of it.
	if wires[0].WindSpanFeet != 100 {
		t.Errorf("wind span = %v, want 100", wires[0].WindSpanFeet)
	}
	for _, w := range wires {
		if w.Tension <= 0 {
			t.Errorf("%s has no tension", w.AttachmentID)
		}
		if w.Owner == "" {
			t.Errorf("%s has no owner", w.AttachmentID)
		}
	}
}

func TestBuildWiresSkipsFacilitiesThatDoNotLeaveThePole(t *testing.T) {
	a := sampleAssembly()
	a.Add(att("A9", "RIDGEPORT CABLEVISION", plant.KindRiser, 120))
	assign := sampleAssignment()
	wires, err := loading.BuildWires(a, sampleLine(t), assign, nesc.Heavy)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range wires {
		if w.AttachmentID == "A9" {
			t.Error("a riser is not a span facility")
		}
	}
	if len(wires) != 4 {
		t.Errorf("wires = %d", len(wires))
	}
}

func TestBuildWiresUsesTheAverageOfTwoSpans(t *testing.T) {
	line := sampleLine(t)
	p := samplePole()
	p.ID = "NW-441830"
	a := plant.NewAssembly(p, []plant.Attachment{
		{ID: "A1", Pole: "NW-441830", Owner: "SUMMIT VALLEY ELECTRIC", Kind: plant.KindPrimary,
			Height: unit.Tenths(335), Voltage: 7620},
	})
	wires, err := loading.BuildWires(a, line, loading.Assignment{"A1": "2 ACSR SPARROW"}, nesc.Heavy)
	if err != nil {
		t.Fatal(err)
	}
	if wires[0].WindSpanFeet != 230 {
		t.Errorf("wind span = %v, want 230", wires[0].WindSpanFeet)
	}
}

func TestBuildWiresRejectsWhatItCannotResolve(t *testing.T) {
	a := sampleAssembly()
	line := sampleLine(t)
	if _, err := loading.BuildWires(nil, line, sampleAssignment(), nesc.Heavy); err == nil {
		t.Error("no assembly should be rejected")
	}
	if _, err := loading.BuildWires(a, nil, sampleAssignment(), nesc.Heavy); err == nil {
		t.Error("no line should be rejected")
	}
	short := sampleAssignment()
	delete(short, "A3")
	if _, err := loading.BuildWires(a, line, short, nesc.Heavy); err == nil {
		t.Error("an unassigned attachment should be rejected")
	}
	wrong := sampleAssignment()
	wrong["A3"] = "baling wire"
	if _, err := loading.BuildWires(a, line, wrong, nesc.Heavy); err == nil {
		t.Error("an uncatalogued conductor should be rejected")
	}
	if _, err := loading.BuildWires(a, line, sampleAssignment(), nesc.District("tropical")); err == nil {
		t.Error("an unknown district should be rejected")
	}
	foreign := plant.NewAssembly(plant.Pole{
		ID: "SE-1129043", Length: 40, Class: plant.Class4, Species: plant.SouthernPine,
		SetDepth: unit.Feet(6), Owner: "SUMMIT VALLEY ELECTRIC", Condition: plant.ConditionSound,
	}, nil)
	if _, err := loading.BuildWires(foreign, line, sampleAssignment(), nesc.Heavy); err == nil {
		t.Error("a pole not on the line should be rejected")
	}
	bare := plant.NewAssembly(samplePole(), []plant.Attachment{
		{ID: "A9", Pole: "NW-441829", Owner: "RIDGEPORT CABLEVISION", Kind: plant.KindRiser, Height: unit.Tenths(120)},
	})
	if _, err := loading.BuildWires(bare, line, loading.Assignment{"A9": "625 COAX"}, nesc.Heavy); err == nil {
		t.Error("a pole with nothing in the span should be rejected")
	}
}

func TestAnalyseAssemblyRunsStraightOffThePlant(t *testing.T) {
	got, err := loading.AnalyseAssembly(sampleAssembly(), sampleLine(t), sampleAssignment(), nesc.Heavy, nesc.GradeC)
	if err != nil {
		t.Fatal(err)
	}
	if got.Pole.ID != "NW-441829" {
		t.Errorf("analysed %s", got.Pole.ID)
	}
	if got.FactoredMoment <= 0 || got.FactoredCapacty <= 0 {
		t.Errorf("analysis = %+v", got)
	}
	short := sampleAssignment()
	delete(short, "A1")
	if _, err := loading.AnalyseAssembly(sampleAssembly(), sampleLine(t), short, nesc.Heavy, nesc.GradeC); err == nil {
		t.Error("an unassigned attachment should be rejected")
	}
}

func TestWithAttachmentAddsTheApplicantsWire(t *testing.T) {
	wires, err := loading.BuildWires(sampleAssembly(), sampleLine(t), sampleAssignment(), nesc.Heavy)
	if err != nil {
		t.Fatal(err)
	}
	newcomer := att("A7", "MERIDIAN FIBER", plant.KindFiber, 186)
	after, err := loading.WithAttachment(wires, newcomer, "144F ADSS", nesc.Heavy)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(wires)+1 {
		t.Fatalf("after = %d wires", len(after))
	}
	if after[len(after)-1].AttachmentID != "A7" {
		t.Errorf("the new fiber should sort to the bottom, got %s", after[len(after)-1].AttachmentID)
	}
	if after[len(after)-1].WindSpanFeet != wires[0].WindSpanFeet {
		t.Error("the new wire takes the pole's wind span")
	}
	if len(wires) != 4 {
		t.Error("WithAttachment should not disturb the list it was given")
	}
	before, err := loading.Analyse(samplePole(), wires, nesc.Heavy, nesc.GradeC)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := loading.Analyse(samplePole(), after, nesc.Heavy, nesc.GradeC)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.FactoredMoment <= before.FactoredMoment {
		t.Error("adding a wire should raise the groundline moment")
	}
}

func TestWithAttachmentRejectsWhatCannotGoUp(t *testing.T) {
	wires, err := loading.BuildWires(sampleAssembly(), sampleLine(t), sampleAssignment(), nesc.Heavy)
	if err != nil {
		t.Fatal(err)
	}
	riser := att("A8", "MERIDIAN FIBER", plant.KindRiser, 120)
	if _, err := loading.WithAttachment(wires, riser, "144F ADSS", nesc.Heavy); err == nil {
		t.Error("a riser does not run into the span")
	}
	newcomer := att("A7", "MERIDIAN FIBER", plant.KindFiber, 186)
	if _, err := loading.WithAttachment(nil, newcomer, "144F ADSS", nesc.Heavy); err == nil {
		t.Error("an empty pole has no wind span to borrow")
	}
	if _, err := loading.WithAttachment(wires, newcomer, "baling wire", nesc.Heavy); err == nil {
		t.Error("an uncatalogued conductor should be rejected")
	}
	if _, err := loading.WithAttachment(wires, newcomer, "144F ADSS", nesc.District("tropical")); err == nil {
		t.Error("an unknown district should be rejected")
	}
}

func TestOwnersOfWires(t *testing.T) {
	wires, err := loading.BuildWires(sampleAssembly(), sampleLine(t), sampleAssignment(), nesc.Heavy)
	if err != nil {
		t.Fatal(err)
	}
	owners := loading.OwnersOf(wires)
	want := []string{"NORTHVALE TELEPHONE", "RIDGEPORT CABLEVISION", "SUMMIT VALLEY ELECTRIC"}
	if len(owners) != len(want) {
		t.Fatalf("owners = %v", owners)
	}
	for i := range want {
		if owners[i] != want[i] {
			t.Errorf("owner %d = %q, want %q", i, owners[i], want[i])
		}
	}
	if got := loading.OwnersOf(nil); got != nil {
		t.Errorf("no wires, no owners, got %v", got)
	}
}

func TestWiresCarryBothSpanFigures(t *testing.T) {
	line := sampleLine(t)
	wires, err := loading.BuildWires(sampleAssembly(), line, sampleAssignment(), nesc.Heavy)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range wires {
		if w.RulingSpanFeet != line.RulingSpan() {
			t.Errorf("%s carries ruling span %v, want %v", w.AttachmentID, w.RulingSpanFeet, line.RulingSpan())
		}
		if w.SpanFeet != 200 {
			t.Errorf("%s carries span %v, want the 200 foot span it lands on", w.AttachmentID, w.SpanFeet)
		}
	}
	if wires[0].SpanFeet == wires[0].RulingSpanFeet {
		t.Error("on this section the ruling span and the span differ, so a test that cannot tell them apart is not testing anything")
	}
}

func TestWireSagRisesWithTemperature(t *testing.T) {
	wires, err := loading.BuildWires(sampleAssembly(), sampleLine(t), sampleAssignment(), nesc.Heavy)
	if err != nil {
		t.Fatal(err)
	}
	w := wires[0]
	bare := w.Conductor.WeightPerFoot
	cold, err := w.Sag(nesc.Heavy, bare, 0)
	if err != nil {
		t.Fatal(err)
	}
	hot, err := w.Sag(nesc.Heavy, bare, unit.TempMaxOperating)
	if err != nil {
		t.Fatal(err)
	}
	if hot <= cold {
		t.Errorf("a hot wire sags more: hot %v, cold %v", hot, cold)
	}
	worst, err := w.MaxSag(nesc.Heavy)
	if err != nil {
		t.Fatal(err)
	}
	if worst < hot {
		t.Errorf("the worst sag %v is under the hot sag %v", worst, hot)
	}
	bad := w
	bad.RulingSpanFeet = 0
	if _, err := bad.Sag(nesc.Heavy, bare, 0); err == nil {
		t.Error("a wire with no ruling span cannot be sagged")
	}
	if _, err := bad.MaxSag(nesc.Heavy); err == nil {
		t.Error("and it has no worst sag either")
	}
	if _, err := w.MaxSag(nesc.District("tropical")); err == nil {
		t.Error("an unknown district should be rejected")
	}
}
