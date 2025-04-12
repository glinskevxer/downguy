package tests

import (
	"math"
	"testing"

	"github.com/glinskevxer/downguy/internal/loading"
	"github.com/glinskevxer/downguy/internal/nesc"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/unit"
)

func heavyLoading(t *testing.T) nesc.Loading {
	t.Helper()
	l, err := nesc.LoadingFor(nesc.Heavy)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func TestWindSpanIsHalfOfEachSide(t *testing.T) {
	if got := loading.WindSpan(200, 260); got != 230 {
		t.Errorf("wind span = %v, want 230", got)
	}
	if got := loading.WindSpan(200); got != 100 {
		t.Errorf("a dead end carries half a span, got %v", got)
	}
	if got := loading.WindSpan(); got != 0 {
		t.Errorf("no spans, no wind span, got %v", got)
	}
	if got := loading.WindSpan(200, 0, -5); got != 100 {
		t.Errorf("empty sides should be skipped, got %v", got)
	}
}

func TestWireWindScalesWithSpanAndDiameter(t *testing.T) {
	l := heavyLoading(t)
	c := sparrow(t)
	got, err := loading.WireWind(c, l, 230)
	if err != nil {
		t.Fatal(err)
	}
	want := l.WindLoadPerFoot(c.DiameterInches) * 230
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("wire wind = %v, want %v", got, want)
	}
	half, err := loading.WireWind(c, l, 115)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(half*2-got) > 1e-9 {
		t.Error("half the wind span carries half the load")
	}
	if zero, err := loading.WireWind(c, l, 0); err != nil || zero != 0 {
		t.Errorf("no wind span, no load: %v %v", zero, err)
	}
	if _, err := loading.WireWind(c, l, -1); err == nil {
		t.Error("a negative wind span should be rejected")
	}
	bad := c
	bad.DiameterInches = 0
	if _, err := loading.WireWind(bad, l, 230); err == nil {
		t.Error("an invalid conductor should be rejected")
	}
}

func TestWireMomentIsForceTimesHeight(t *testing.T) {
	l := heavyLoading(t)
	c := sparrow(t)
	force, err := loading.WireWind(c, l, 230)
	if err != nil {
		t.Fatal(err)
	}
	got, err := loading.WireMoment(c, l, 230, unit.Tenths(300))
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(got-force*30) > 1e-9 {
		t.Errorf("moment = %v, want %v", got, force*30)
	}
	lower, err := loading.WireMoment(c, l, 230, unit.Tenths(200))
	if err != nil {
		t.Fatal(err)
	}
	if lower >= got {
		t.Error("a lower attachment puts less moment on the groundline")
	}
	if _, err := loading.WireMoment(c, l, 230, 0); err == nil {
		t.Error("an attachment at no height should be rejected")
	}
}

func TestPoleWindActsAboveTheThirdPoint(t *testing.T) {
	l := heavyLoading(t)
	p := samplePole()
	force, arm, err := loading.PoleWind(p, l)
	if err != nil {
		t.Fatal(err)
	}
	if force <= 0 {
		t.Errorf("pole wind = %v", force)
	}
	above := p.AboveGrade().Float()
	if arm.Float() <= above/3 || arm.Float() >= above/2 {
		t.Errorf("the resultant of a tapered pole sits between a third and half way up, got %v of %v", arm.Float(), above)
	}
	moment, err := loading.PoleMoment(p, l)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(moment-force*arm.Float()) > 1e-9 {
		t.Errorf("pole moment = %v, want %v", moment, force*arm.Float())
	}
	sunk := p
	sunk.SetDepth = unit.Feet(40)
	if _, _, err := loading.PoleWind(sunk, l); err == nil {
		t.Error("a pole with nothing above grade should be rejected")
	}
	if _, err := loading.PoleMoment(sunk, l); err == nil {
		t.Error("a pole with nothing above grade has no moment")
	}
	untabulated := p
	untabulated.Length = 95
	if _, _, err := loading.PoleWind(untabulated, l); err == nil {
		t.Error("an untabulated pole should be rejected")
	}
}

func TestAngleTensionIsTwiceTheSineOfHalf(t *testing.T) {
	got, err := loading.AngleTension(2000, 60)
	if err != nil {
		t.Fatal(err)
	}
	want := 2 * 2000 * math.Sin(30*math.Pi/180)
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("angle tension = %v, want %v", got, want)
	}
	straight, err := loading.AngleTension(2000, 0)
	if err != nil {
		t.Fatal(err)
	}
	if straight != 0 {
		t.Errorf("a straight line pulls nothing sideways, got %v", straight)
	}
	back, err := loading.AngleTension(2000, 180)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(back-4000) > 1e-9 {
		t.Errorf("a dead turn pulls twice the tension, got %v", back)
	}
	for _, d := range []float64{-1, 181} {
		if _, err := loading.AngleTension(2000, d); err == nil {
			t.Errorf("deflection %v should be rejected", d)
		}
	}
	if _, err := loading.AngleTension(-1, 30); err == nil {
		t.Error("a negative tension should be rejected")
	}
}

func TestAngleMoment(t *testing.T) {
	force, err := loading.AngleTension(2000, 30)
	if err != nil {
		t.Fatal(err)
	}
	got, err := loading.AngleMoment(2000, 30, unit.Tenths(320))
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(got-force*32) > 1e-9 {
		t.Errorf("angle moment = %v, want %v", got, force*32)
	}
	if _, err := loading.AngleMoment(2000, 30, 0); err == nil {
		t.Error("an attachment at no height should be rejected")
	}
	if _, err := loading.AngleMoment(2000, 400, unit.Tenths(320)); err == nil {
		t.Error("an impossible deflection should be rejected")
	}
}

func TestPoleWindGrowsWithTheDistrict(t *testing.T) {
	p := samplePole()
	light, err := nesc.LoadingFor(nesc.Light)
	if err != nil {
		t.Fatal(err)
	}
	heavy := heavyLoading(t)
	lightForce, _, err := loading.PoleWind(p, light)
	if err != nil {
		t.Fatal(err)
	}
	heavyForce, _, err := loading.PoleWind(p, heavy)
	if err != nil {
		t.Fatal(err)
	}
	if lightForce <= heavyForce {
		t.Errorf("the light district blows harder on bare structure: light %v, heavy %v", lightForce, heavyForce)
	}
	if plantCheck := plant.StandardSetDepth(40); plantCheck != unit.Feet(7) {
		t.Errorf("set depth sanity = %s", plantCheck)
	}
}
