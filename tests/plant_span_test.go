package tests

import (
	"errors"
	"testing"

	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/unit"
)

func sampleSpan() plant.SpanRecord {
	return plant.SpanRecord{
		ID: "SP-100", FromPole: "NW-441829", ToPole: "NW-441830",
		Length: unit.SpanFeet(225), Cover: plant.CoverRoad, Drop: unit.Tenths(-8),
	}
}

func TestCoverTable(t *testing.T) {
	if !plant.KnownCover(plant.CoverRoad) || plant.KnownCover(plant.Cover("swamp")) {
		t.Error("known cover check")
	}
	if plant.CoverRoad.Name() != "public street or road" {
		t.Errorf("name = %q", plant.CoverRoad.Name())
	}
	if plant.Cover("swamp").Name() != "unlisted ground cover swamp" {
		t.Errorf("unknown name = %q", plant.Cover("swamp").Name())
	}
	for _, in := range []string{"water_no_sail", "water no sail", "WATER-NO-SAIL"} {
		got, err := plant.ParseCover(in)
		if err != nil || got != plant.CoverWaterNoSail {
			t.Errorf("ParseCover(%q) = %q %v", in, got, err)
		}
	}
	if _, err := plant.ParseCover("swamp"); !errors.Is(err, plant.ErrBadSpan) {
		t.Errorf("unknown cover error = %v", err)
	}
}

func TestCoverTruckTraffic(t *testing.T) {
	trucks := []plant.Cover{plant.CoverRoad, plant.CoverDriveway, plant.CoverAlley, plant.CoverParking, plant.CoverRailroad}
	for _, c := range trucks {
		if !c.TruckTraffic() {
			t.Errorf("%s carries trucks", c)
		}
	}
	quiet := []plant.Cover{plant.CoverPedestrian, plant.CoverField, plant.CoverCultivated, plant.CoverWaterNoSail}
	for _, c := range quiet {
		if c.TruckTraffic() {
			t.Errorf("%s does not carry trucks", c)
		}
	}
}

func TestSpanValidateAccepts(t *testing.T) {
	if err := sampleSpan().Validate(); err != nil {
		t.Fatalf("a good span failed: %v", err)
	}
}

func TestSpanValidateRejects(t *testing.T) {
	cases := map[string]func(*plant.SpanRecord){
		"no id":     func(s *plant.SpanRecord) { s.ID = "" },
		"bad from":  func(s *plant.SpanRecord) { s.FromPole = "nope" },
		"bad to":    func(s *plant.SpanRecord) { s.ToPole = "nope" },
		"loop":      func(s *plant.SpanRecord) { s.ToPole = s.FromPole },
		"short":     func(s *plant.SpanRecord) { s.Length = unit.SpanFeet(4) },
		"long":      func(s *plant.SpanRecord) { s.Length = unit.SpanFeet(3000) },
		"bad cover": func(s *plant.SpanRecord) { s.Cover = plant.Cover("swamp") },
		"cliff":     func(s *plant.SpanRecord) { s.Drop = unit.Feet(400) },
	}
	for name, mutate := range cases {
		s := sampleSpan()
		mutate(&s)
		if err := s.Validate(); err == nil {
			t.Errorf("%s should have been rejected", name)
		}
	}
}

func TestSpanEndpoints(t *testing.T) {
	s := sampleSpan()
	if !s.Touches("NW-441829") || !s.Touches("NW-441830") {
		t.Error("the span touches both of its poles")
	}
	if s.Touches("SE-1129043") {
		t.Error("the span does not touch a pole it does not name")
	}
	other, ok := s.Other("NW-441829")
	if !ok || other != "NW-441830" {
		t.Errorf("other end = %s ok=%v", other, ok)
	}
	other, ok = s.Other("NW-441830")
	if !ok || other != "NW-441829" {
		t.Errorf("other end = %s ok=%v", other, ok)
	}
	if _, ok := s.Other("SE-1129043"); ok {
		t.Error("a pole not on the span has no far end")
	}
}

func TestSpanLevelAndString(t *testing.T) {
	s := sampleSpan()
	if !s.Level() {
		t.Error("a span dropping less than a foot should count as level")
	}
	s.Drop = unit.Tenths(-45)
	if s.Level() {
		t.Error("a four and a half foot drop is not level")
	}
	want := "SP-100 NW-441829 to NW-441830, 225 ft over road"
	if got := sampleSpan().String(); got != want {
		t.Errorf("String = %q, want %q", got, want)
	}
}
