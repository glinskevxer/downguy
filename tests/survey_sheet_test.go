package tests

import (
	"testing"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/survey"
	"github.com/glinskevxer/downguy/internal/unit"
)

func measure(subject string, tenths int, m survey.Method) survey.Measurement {
	return survey.Measurement{
		Subject: subject, Height: unit.Tenths(tenths), Method: m, Crew: "CREW 41",
	}
}

func sampleSheet() survey.PoleSheet {
	return survey.PoleSheet{
		ID:    "SV-2201",
		Pole:  "NW-441829",
		Taken: calendar.MustNew(2025, 5, 6),
		Crew:  "CREW 41",
		Measurements: []survey.Measurement{
			measure("A3", 200, survey.MethodStick),
			measure("A1", 335, survey.MethodLaser),
			measure("A4", 186, survey.MethodStick),
			measure("A2", 285, survey.MethodLaser),
		},
		Condition:  plant.ConditionSound,
		GroundLine: unit.Tenths(0),
	}
}

func TestSheetValidateAccepts(t *testing.T) {
	if err := sampleSheet().Validate(); err != nil {
		t.Fatalf("a good sheet failed: %v", err)
	}
}

func TestSheetValidateRejects(t *testing.T) {
	cases := map[string]func(*survey.PoleSheet){
		"no id":       func(s *survey.PoleSheet) { s.ID = " " },
		"bad pole":    func(s *survey.PoleSheet) { s.Pole = "nope" },
		"no date":     func(s *survey.PoleSheet) { s.Taken = calendar.Zero },
		"no crew":     func(s *survey.PoleSheet) { s.Crew = "" },
		"bad grade":   func(s *survey.PoleSheet) { s.Condition = plant.Condition("mint") },
		"no readings": func(s *survey.PoleSheet) { s.Measurements = nil },
		"bad reading": func(s *survey.PoleSheet) { s.Measurements[0].Height = 0 },
		"read twice": func(s *survey.PoleSheet) {
			s.Measurements = append(s.Measurements, measure("A3", 201, survey.MethodStick))
		},
	}
	for name, mutate := range cases {
		s := sampleSheet()
		s.Measurements = append([]survey.Measurement{}, s.Measurements...)
		mutate(&s)
		if err := s.Validate(); err == nil {
			t.Errorf("%s should have been rejected", name)
		}
	}
}

func TestSheetLookups(t *testing.T) {
	s := sampleSheet()
	m, ok := s.For("A2")
	if !ok || m.Height.Tenths() != 285 {
		t.Errorf("For(A2) = %v ok=%v", m, ok)
	}
	if _, ok := s.For("A9"); ok {
		t.Error("For should miss")
	}
	subjects := s.Subjects()
	want := []string{"A1", "A2", "A3", "A4"}
	for i := range want {
		if subjects[i] != want[i] {
			t.Errorf("subject %d = %q, want %q", i, subjects[i], want[i])
		}
	}
}

func TestSheetMissing(t *testing.T) {
	s := sampleSheet()
	got := s.Missing([]string{"A1", "A7", "A2", "A9"})
	if len(got) != 2 || got[0] != "A7" || got[1] != "A9" {
		t.Errorf("missing = %v", got)
	}
	if got := s.Missing([]string{"A1", "A2"}); got != nil {
		t.Errorf("nothing should be missing, got %v", got)
	}
	if got := s.Missing(nil); got != nil {
		t.Errorf("asking for nothing misses nothing, got %v", got)
	}
}

func TestSheetUnreliable(t *testing.T) {
	s := sampleSheet()
	if got := s.Unreliable(); len(got) != 0 {
		t.Errorf("a sheet of stick and laser readings should be clean, got %v", got)
	}
	s.Measurements = append(s.Measurements, measure("A5", 150, survey.MethodEstimated))
	s.Measurements = append(s.Measurements, measure("A0", 340, survey.MethodPhoto))
	got := s.Unreliable()
	if len(got) != 2 {
		t.Fatalf("unreliable = %v", got)
	}
	if got[0].Subject != "A0" || got[1].Subject != "A5" {
		t.Errorf("unreliable should be sorted by subject, got %v", got)
	}
}

func TestSheetOrderedRunsTopDown(t *testing.T) {
	s := sampleSheet()
	ordered := s.Ordered()
	want := []string{"A1", "A2", "A3", "A4"}
	for i := range want {
		if ordered[i].Subject != want[i] {
			t.Errorf("position %d = %q, want %q", i, ordered[i].Subject, want[i])
		}
	}
	if len(s.Measurements) != 4 || s.Measurements[0].Subject != "A3" {
		t.Error("Ordered should not disturb the sheet")
	}
}

func TestSheetAgeAndStaleness(t *testing.T) {
	s := sampleSheet()
	asOf := calendar.MustNew(2025, 6, 5)
	if got := s.Age(asOf); got != 30 {
		t.Errorf("age = %d, want 30", got)
	}
	if s.Stale(asOf) {
		t.Error("a month old survey is not stale")
	}
	old := calendar.MustNew(2026, 1, 1)
	if !s.Stale(old) {
		t.Errorf("a %d day old survey is stale", s.Age(old))
	}
	edge := s.Taken.AddDays(survey.StaleAfterDays)
	if s.Stale(edge) {
		t.Error("the last good day is not stale")
	}
	if s.Stale(edge.AddDays(1)) != true {
		t.Error("the day after is")
	}
	s.Taken = calendar.Zero
	if got := s.Age(asOf); got != 0 {
		t.Errorf("an undated sheet has no age, got %d", got)
	}
	if got := sampleSheet().Age(calendar.Zero); got != 0 {
		t.Errorf("no reference date, no age, got %d", got)
	}
}

func TestSheetString(t *testing.T) {
	want := "SV-2201 NW-441829 on 2025-05-06 by CREW 41, 4 readings"
	if got := sampleSheet().String(); got != want {
		t.Errorf("String = %q, want %q", got, want)
	}
}
