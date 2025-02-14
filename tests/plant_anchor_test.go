package tests

import (
	"math"
	"testing"

	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/unit"
)

func sampleAnchor() plant.Anchor {
	return plant.Anchor{
		ID: "ANC-11", Pole: "NW-441829", Lead: 20, Bearing: 270,
		Guys: []plant.Guy{
			{ID: "G1", Height: unit.Tenths(300), Size: "3/8 EHS"},
		},
	}
}

func TestGuyStrengthTable(t *testing.T) {
	cases := map[string]int{
		"3/8 EHS":  15400,
		"1/4 EHS":  6650,
		"1/2 EHS":  26900,
		"3/8 cs":   10800,
		" 3/8 EHS": 15400,
	}
	for size, want := range cases {
		got, ok := plant.GuyStrength(size)
		if !ok || got != want {
			t.Errorf("GuyStrength(%q) = %d ok=%v, want %d", size, got, ok, want)
		}
	}
	if _, ok := plant.GuyStrength("baling wire"); ok {
		t.Error("an unstocked size should report false")
	}
	sizes := plant.GuySizes()
	if len(sizes) != 8 {
		t.Errorf("stocked sizes = %v", sizes)
	}
	for i := 1; i < len(sizes); i++ {
		if sizes[i] < sizes[i-1] {
			t.Fatalf("sizes are not sorted: %v", sizes)
		}
	}
}

func TestAnchorValidateAccepts(t *testing.T) {
	if err := sampleAnchor().Validate(); err != nil {
		t.Fatalf("a good anchor failed: %v", err)
	}
}

func TestAnchorValidateRejects(t *testing.T) {
	cases := map[string]func(*plant.Anchor){
		"no id":         func(a *plant.Anchor) { a.ID = "" },
		"bad pole":      func(a *plant.Anchor) { a.Pole = "nope" },
		"short lead":    func(a *plant.Anchor) { a.Lead = 2 },
		"long lead":     func(a *plant.Anchor) { a.Lead = 90 },
		"bad bearing":   func(a *plant.Anchor) { a.Bearing = 400 },
		"neg bearing":   func(a *plant.Anchor) { a.Bearing = -1 },
		"no guys":       func(a *plant.Anchor) { a.Guys = nil },
		"guy no id":     func(a *plant.Anchor) { a.Guys[0].ID = "" },
		"guy no height": func(a *plant.Anchor) { a.Guys[0].Height = 0 },
		"guy bad size":  func(a *plant.Anchor) { a.Guys[0].Size = "baling wire" },
		"dup guy": func(a *plant.Anchor) {
			a.Guys = append(a.Guys, plant.Guy{ID: "G1", Height: unit.Tenths(280), Size: "1/4 EHS"})
		},
	}
	for name, mutate := range cases {
		a := sampleAnchor()
		a.Guys = append([]plant.Guy{}, a.Guys...)
		mutate(&a)
		if err := a.Validate(); err == nil {
			t.Errorf("%s should have been rejected", name)
		}
	}
}

func TestAnchorLeadAngle(t *testing.T) {
	a := sampleAnchor()
	a.Lead = 30
	a.Guys[0].Height = unit.Feet(30)
	got := a.LeadAngle(a.Guys[0])
	if math.Abs(got-math.Pi/4) > 1e-9 {
		t.Errorf("a one to one lead should be 45 degrees, got %v radians", got)
	}
	a.Lead = 15
	shallow := a.LeadAngle(a.Guys[0])
	if shallow >= got {
		t.Error("a shorter lead gives a shallower angle")
	}
}

func TestAnchorHoldingCapacity(t *testing.T) {
	a := sampleAnchor()
	a.Lead = 30
	a.Guys[0].Height = unit.Feet(30)
	got, err := a.HoldingCapacity()
	if err != nil {
		t.Fatal(err)
	}
	want := 15400 * math.Sin(math.Pi/4)
	if math.Abs(got-want) > 1e-6 {
		t.Errorf("holding capacity = %v, want %v", got, want)
	}
	a.Guys = append(a.Guys, plant.Guy{ID: "G2", Height: unit.Feet(30), Size: "3/8 EHS"})
	doubled, err := a.HoldingCapacity()
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(doubled-2*want) > 1e-6 {
		t.Errorf("two guys = %v, want %v", doubled, 2*want)
	}
	bad := sampleAnchor()
	bad.Lead = 1
	if _, err := bad.HoldingCapacity(); err == nil {
		t.Error("an invalid anchor should not report a capacity")
	}
}

func TestAnchorLeadRatio(t *testing.T) {
	a := sampleAnchor()
	a.Lead = 30
	a.Guys[0].Height = unit.Feet(30)
	if got := a.LeadRatio(); math.Abs(got-1) > 1e-9 {
		t.Errorf("lead ratio = %v, want 1", got)
	}
	a.Lead = 15
	if got := a.LeadRatio(); math.Abs(got-0.5) > 1e-9 {
		t.Errorf("lead ratio = %v, want 0.5", got)
	}
	a.Guys = append(a.Guys, plant.Guy{ID: "G2", Height: unit.Feet(20), Size: "1/4 EHS"})
	if got := a.LeadRatio(); math.Abs(got-0.5) > 1e-9 {
		t.Errorf("the ratio should follow the highest guy, got %v", got)
	}
	empty := plant.Anchor{ID: "ANC-0", Pole: "NW-441829", Lead: 20}
	if got := empty.LeadRatio(); got != 0 {
		t.Errorf("an anchor with no guys has no ratio, got %v", got)
	}
}
