package tests

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/clearance"
	"github.com/glinskevxer/downguy/internal/ingest"
	"github.com/glinskevxer/downguy/internal/loading"
	"github.com/glinskevxer/downguy/internal/nesc"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/rental"
	"github.com/glinskevxer/downguy/internal/sagten"
	"github.com/glinskevxer/downguy/internal/store"
	"github.com/glinskevxer/downguy/internal/survey"
	"github.com/glinskevxer/downguy/internal/unit"
)

// The numbers that matter here are the ones a system wide sweep multiplies by
// sixty thousand poles. A clearance check that takes a millisecond is a minute
// of wall clock across the whole system, and a sag solve inside it is the part
// that could quietly become ten.

func BenchmarkSolveTension(b *testing.B) {
	c, err := sagten.Lookup("2 ACSR SPARROW")
	if err != nil {
		b.Fatal(err)
	}
	from, err := sagten.InitialCondition(c, 225, nesc.Heavy)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := sagten.SolveTension(c, 225, from, from.LoadPerFoot, unit.TempMaxOperating); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBuildSagTable(b *testing.B) {
	c, err := sagten.Lookup("4/0 ACSR PENGUIN")
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := sagten.BuildTable(c, nesc.Heavy, 225, 225); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGroundlineAnalysis(b *testing.B) {
	wires := benchWires(b)
	pole := benchPole()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := loading.Analyse(pole, wires, nesc.Heavy, nesc.GradeC); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFindReplacement(b *testing.B) {
	wires := benchWires(b)
	pole := benchPole()
	pole.Class = plant.Class7
	pole.Length = 30
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := loading.FindReplacement(pole, wires, nesc.Heavy, nesc.GradeB); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkClearancePoleCheck(b *testing.B) {
	assembly := benchAssembly()
	sheet := benchSheet()
	applied, err := survey.Apply(sheet, assembly)
	if err != nil {
		b.Fatal(err)
	}
	engine := clearance.New()
	ctx := clearance.PoleContext{Applied: applied, District: nesc.Heavy, Grade: nesc.GradeC}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := engine.CheckPole(ctx); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSurveyApply(b *testing.B) {
	assembly := benchAssembly()
	sheet := benchSheet()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := survey.Apply(sheet, assembly); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkImportAttachments(b *testing.B) {
	body := benchAttachmentCSV(200)
	b.SetBytes(int64(len(body)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := ingest.Attachments(strings.NewReader(body))
		if err != nil {
			b.Fatal(err)
		}
		if !result.Clean() {
			b.Fatalf("the fixture should parse clean, got %v", result.Problems)
		}
	}
}

func BenchmarkExportAttachments(b *testing.B) {
	result, err := ingest.Attachments(strings.NewReader(benchAttachmentCSV(200)))
	if err != nil {
		b.Fatal(err)
	}
	var buf bytes.Buffer
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		if err := ingest.WriteAttachments(&buf, result.Rows); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRentalRate(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if _, err := rental.Standard(calendar.RateYear(2025), rental.FormulaTelecom, 5); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkStoreAssemblyRoundTrip(b *testing.B) {
	ctx := context.Background()
	db, err := store.Open(ctx, store.Memory)
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	loader := store.NewPlant(db)
	if err := loader.SaveAssembly(ctx, benchAssembly()); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := loader.Assembly(ctx, "NW-441829"); err != nil {
			b.Fatal(err)
		}
	}
}

func benchPole() plant.Pole {
	return plant.Pole{
		ID: "NW-441829", Length: 40, Class: plant.Class4, Species: plant.SouthernPine,
		SetDepth: unit.Feet(6), InstalledIn: 1998,
		Owner: "SUMMIT VALLEY ELECTRIC", Condition: plant.ConditionSound,
	}
}

func benchAttachment(id, owner string, kind plant.Kind, tenths int) plant.Attachment {
	a := plant.Attachment{
		ID: id, Pole: "NW-441829", Owner: owner, Kind: kind,
		Height: unit.Tenths(tenths), Authorized: true,
		InForce: calendar.Range{From: calendar.MustNew(2019, 1, 1)},
	}
	if kind == plant.KindPrimary {
		a.Voltage = 7620
	}
	return a
}

func benchAssembly() *plant.Assembly {
	return plant.NewAssembly(benchPole(), []plant.Attachment{
		benchAttachment("A1", "SUMMIT VALLEY ELECTRIC", plant.KindPrimary, 335),
		benchAttachment("A2", "SUMMIT VALLEY ELECTRIC", plant.KindNeutral, 285),
		benchAttachment("A3", "RIDGEPORT CABLEVISION", plant.KindCommCable, 200),
		benchAttachment("A4", "NORTHVALE TELEPHONE", plant.KindCommCable, 186),
	})
}

func benchSheet() survey.PoleSheet {
	return survey.PoleSheet{
		ID: "SV-1", Pole: "NW-441829", Taken: calendar.MustNew(2025, 5, 6),
		Crew: "CREW 41", Condition: plant.ConditionSound,
		Measurements: []survey.Measurement{
			{Subject: "A1", Height: unit.Tenths(335), Method: survey.MethodLaser, Crew: "CREW 41"},
			{Subject: "A2", Height: unit.Tenths(285), Method: survey.MethodLaser, Crew: "CREW 41"},
			{Subject: "A3", Height: unit.Tenths(200), Method: survey.MethodStick, Crew: "CREW 41"},
			{Subject: "A4", Height: unit.Tenths(186), Method: survey.MethodStick, Crew: "CREW 41"},
		},
	}
}

func benchWires(b *testing.B) []loading.Wire {
	b.Helper()
	sparrow, err := sagten.Lookup("2 ACSR SPARROW")
	if err != nil {
		b.Fatal(err)
	}
	messenger, err := sagten.Lookup("3/8 EHS MESSENGER")
	if err != nil {
		b.Fatal(err)
	}
	return []loading.Wire{
		{AttachmentID: "A1", Conductor: sparrow, WindSpanFeet: 230, Height: unit.Tenths(335)},
		{AttachmentID: "A2", Conductor: sparrow, WindSpanFeet: 230, Height: unit.Tenths(285)},
		{AttachmentID: "A3", Conductor: messenger, WindSpanFeet: 230, Height: unit.Tenths(200)},
	}
}

func benchAttachmentCSV(rows int) string {
	var b strings.Builder
	b.WriteString("attachment_id,pole_id,owner,kind,height,authorized,in_force_from,voltage\n")
	for i := 0; i < rows; i++ {
		b.WriteString("A")
		b.WriteString(itoaTest(i))
		b.WriteString(",NW-441829,RIDGEPORT CABLEVISION,comm_cable,20.0,y,2019-04-01,\n")
	}
	return b.String()
}
