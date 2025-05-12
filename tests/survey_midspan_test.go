package tests

import (
	"testing"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/survey"
	"github.com/glinskevxer/downguy/internal/unit"
)

func sampleSpanSheet() survey.SpanSheet {
	return survey.SpanSheet{
		ID: "SS-330", Span: "SP-1", FromPole: "NW-441829", ToPole: "NW-441830",
		Taken: calendar.MustNew(2025, 5, 6), Crew: "CREW 41",
		AirTemp: 68, Length: unit.SpanFeet(200),
		Stations: []survey.Station{
			{Feet: 100, Cover: plant.CoverRoad, Measurements: []survey.Measurement{
				measure("A1", 296, survey.MethodLaser),
				measure("A3", 176, survey.MethodLaser),
			}},
			{Feet: 40, Cover: plant.CoverField, Measurements: []survey.Measurement{
				measure("A1", 312, survey.MethodLaser),
				measure("A3", 188, survey.MethodLaser),
			}},
			{Feet: 160, Cover: plant.CoverDriveway, Measurements: []survey.Measurement{
				measure("A1", 302, survey.MethodLaser),
				measure("A3", 172, survey.MethodLaser),
			}},
		},
	}
}

func TestSpanSheetValidateAccepts(t *testing.T) {
	if err := sampleSpanSheet().Validate(); err != nil {
		t.Fatalf("a good span sheet failed: %v", err)
	}
}

func TestSpanSheetValidateRejects(t *testing.T) {
	cases := map[string]func(*survey.SpanSheet){
		"no id":       func(s *survey.SpanSheet) { s.ID = "" },
		"bad from":    func(s *survey.SpanSheet) { s.FromPole = "nope" },
		"bad to":      func(s *survey.SpanSheet) { s.ToPole = "nope" },
		"no date":     func(s *survey.SpanSheet) { s.Taken = calendar.Zero },
		"no crew":     func(s *survey.SpanSheet) { s.Crew = " " },
		"bad length":  func(s *survey.SpanSheet) { s.Length = unit.SpanFeet(4) },
		"no stations": func(s *survey.SpanSheet) { s.Stations = nil },
		"off the end": func(s *survey.SpanSheet) { s.Stations[0].Feet = 400 },
		"behind":      func(s *survey.SpanSheet) { s.Stations[0].Feet = -10 },
		"twice":       func(s *survey.SpanSheet) { s.Stations[1].Feet = s.Stations[0].Feet },
		"bad cover":   func(s *survey.SpanSheet) { s.Stations[0].Cover = plant.Cover("swamp") },
		"empty":       func(s *survey.SpanSheet) { s.Stations[0].Measurements = nil },
		"bad reading": func(s *survey.SpanSheet) { s.Stations[0].Measurements[0].Crew = "" },
	}
	for name, mutate := range cases {
		s := sampleSpanSheet()
		s.Stations = append([]survey.Station{}, s.Stations...)
		for i := range s.Stations {
			s.Stations[i].Measurements = append([]survey.Measurement{}, s.Stations[i].Measurements...)
		}
		mutate(&s)
		if err := s.Validate(); err == nil {
			t.Errorf("%s should have been rejected", name)
		}
	}
}

func TestSpanSheetOrdersStationsFromTheNearPole(t *testing.T) {
	s := sampleSpanSheet()
	ordered := s.Ordered()
	want := []int{40, 100, 160}
	for i := range want {
		if ordered[i].Feet != want[i] {
			t.Errorf("station %d at %d feet, want %d", i, ordered[i].Feet, want[i])
		}
	}
	if s.Stations[0].Feet != 100 {
		t.Error("Ordered should not disturb the sheet")
	}
	st, ok := s.At(160)
	if !ok || st.Cover != plant.CoverDriveway {
		t.Errorf("At(160) = %+v ok=%v", st, ok)
	}
	if _, ok := s.At(75); ok {
		t.Error("no station at 75 feet")
	}
}

func TestLowestFindsTheTightestStationNotTheMiddle(t *testing.T) {
	s := sampleSpanSheet()
	m, st, ok := s.Lowest("A3")
	if !ok {
		t.Fatal("A3 was read three times")
	}
	if m.Height.Tenths() != 172 {
		t.Errorf("lowest A3 = %d tenths, want 172", m.Height.Tenths())
	}
	if st.Feet != 160 || st.Cover != plant.CoverDriveway {
		t.Errorf("lowest A3 at %d feet over %s", st.Feet, st.Cover)
	}
	m, st, ok = s.Lowest("A1")
	if !ok || m.Height.Tenths() != 296 || st.Feet != 100 {
		t.Errorf("lowest A1 = %d tenths at %d feet", m.Height.Tenths(), st.Feet)
	}
	if _, _, ok := s.Lowest("A9"); ok {
		t.Error("A9 was never read")
	}
}

func TestSpanSheetCoversAndSubjects(t *testing.T) {
	s := sampleSpanSheet()
	covers := s.Covers()
	if len(covers) != 3 {
		t.Fatalf("covers = %v", covers)
	}
	for i := 1; i < len(covers); i++ {
		if covers[i] < covers[i-1] {
			t.Fatalf("covers are not sorted: %v", covers)
		}
	}
	subjects := s.Subjects()
	if len(subjects) != 2 || subjects[0] != "A1" || subjects[1] != "A3" {
		t.Errorf("subjects = %v", subjects)
	}
}

func TestSpanSheetString(t *testing.T) {
	want := "SS-330 NW-441829 to NW-441830 on 2025-05-06, 3 stations over 200 ft"
	if got := sampleSpanSheet().String(); got != want {
		t.Errorf("String = %q, want %q", got, want)
	}
}
