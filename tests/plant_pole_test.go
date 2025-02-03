package tests

import (
	"errors"
	"testing"

	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/unit"
)

func TestParsePoleID(t *testing.T) {
	good := map[string]string{
		"NW-441829":   "NW-441829",
		"nw-441829":   "NW-441829",
		" se-1129043": "SE-1129043",
	}
	for in, want := range good {
		got, err := plant.ParsePoleID(in)
		if err != nil {
			t.Errorf("ParsePoleID(%q): %v", in, err)
			continue
		}
		if string(got) != want {
			t.Errorf("ParsePoleID(%q) = %q, want %q", in, got, want)
		}
	}
	for _, in := range []string{"", "441829", "N-441829", "NWX-441829", "NW-44182", "NW-11290431", "NW-4418X9", "12-441829"} {
		if _, err := plant.ParsePoleID(in); err == nil {
			t.Errorf("ParsePoleID(%q) should have failed", in)
		} else if !errors.Is(err, plant.ErrBadPole) {
			t.Errorf("ParsePoleID(%q) gave %v", in, err)
		}
	}
}

func TestPoleIDDistrict(t *testing.T) {
	id, err := plant.ParsePoleID("SE-1129043")
	if err != nil {
		t.Fatal(err)
	}
	if id.District() != "SE" {
		t.Errorf("district = %q", id.District())
	}
	if id.String() != "SE-1129043" {
		t.Errorf("String = %q", id.String())
	}
	if plant.PoleID("X").District() != "" {
		t.Error("a short id has no district")
	}
}

func TestSpeciesLookup(t *testing.T) {
	if !plant.KnownSpecies(plant.SouthernPine) || plant.KnownSpecies(plant.Species("XXX")) {
		t.Error("known species check")
	}
	if plant.SouthernPine.Name() != "southern yellow pine" {
		t.Errorf("name = %q", plant.SouthernPine.Name())
	}
	if plant.Species("ZZZ").Name() != "unknown species ZZZ" {
		t.Errorf("unknown name = %q", plant.Species("ZZZ").Name())
	}
	got, err := plant.ParseSpecies("douglas fir")
	if err != nil || got != plant.DouglasFir {
		t.Errorf("ParseSpecies long name = %q %v", got, err)
	}
	got, err = plant.ParseSpecies(" wrc ")
	if err != nil || got != plant.WesternRedCedar {
		t.Errorf("ParseSpecies code = %q %v", got, err)
	}
	if _, err := plant.ParseSpecies("balsa"); err == nil {
		t.Error("balsa is not a pole species")
	}
}

func TestConditionOrdering(t *testing.T) {
	if !plant.ConditionReject.WorseThan(plant.ConditionSound) {
		t.Error("reject is worse than sound")
	}
	if plant.ConditionNew.WorseThan(plant.ConditionSound) {
		t.Error("new is not worse than sound")
	}
	if !plant.KnownCondition(plant.ConditionRestorable) || plant.KnownCondition(plant.Condition("mint")) {
		t.Error("known condition check")
	}
}

func TestStandardSetDepth(t *testing.T) {
	cases := []struct {
		length int
		tenths int
	}{
		{25, 60},
		{30, 60},
		{35, 65},
		{40, 70},
		{45, 75},
		{50, 70 + 10},
		{60, 90},
	}
	for _, c := range cases {
		if got := plant.StandardSetDepth(c.length).Tenths(); got != c.tenths {
			t.Errorf("StandardSetDepth(%d) = %d tenths, want %d", c.length, got, c.tenths)
		}
	}
}

func samplePole() plant.Pole {
	return plant.Pole{
		ID:          "NW-441829",
		Length:      40,
		Class:       plant.Class4,
		Species:     plant.SouthernPine,
		SetDepth:    unit.Feet(6),
		InstalledIn: 1998,
		Owner:       "SUMMIT VALLEY ELECTRIC",
		Condition:   plant.ConditionSound,
	}
}

func TestPoleValidateAccepts(t *testing.T) {
	if err := samplePole().Validate(); err != nil {
		t.Fatalf("a good pole failed validation: %v", err)
	}
}

func TestPoleValidateRejects(t *testing.T) {
	cases := map[string]func(*plant.Pole){
		"bad id":        func(p *plant.Pole) { p.ID = "nope" },
		"short":         func(p *plant.Pole) { p.Length = 15 },
		"tall":          func(p *plant.Pole) { p.Length = 130 },
		"odd length":    func(p *plant.Pole) { p.Length = 42 },
		"bad class":     func(p *plant.Pole) { p.Class = plant.Class("Q") },
		"bad species":   func(p *plant.Pole) { p.Species = plant.Species("BAL") },
		"bad condition": func(p *plant.Pole) { p.Condition = plant.Condition("mint") },
		"no set depth":  func(p *plant.Pole) { p.SetDepth = 0 },
		"buried":        func(p *plant.Pole) { p.SetDepth = unit.Feet(40) },
		"no owner":      func(p *plant.Pole) { p.Owner = "" },
	}
	for name, mutate := range cases {
		p := samplePole()
		mutate(&p)
		if err := p.Validate(); err == nil {
			t.Errorf("%s should have been rejected", name)
		}
	}
}

func TestPoleDerivedValues(t *testing.T) {
	p := samplePole()
	if got := p.AboveGrade().Tenths(); got != 340 {
		t.Errorf("above grade = %d tenths", got)
	}
	if got := p.Age(2025); got != 27 {
		t.Errorf("age = %d", got)
	}
	p.InstalledIn = 0
	if got := p.Age(2025); got != 0 {
		t.Errorf("unknown install year should give age 0, got %d", got)
	}
	if got := samplePole().String(); got != "NW-441829 40/4 SPF" {
		t.Errorf("String = %q", got)
	}
}
