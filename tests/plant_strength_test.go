package tests

import (
	"errors"
	"math"
	"testing"

	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/unit"
)

func TestFibreStressTable(t *testing.T) {
	cases := map[plant.Species]int{
		plant.SouthernPine:    8000,
		plant.DouglasFir:      8000,
		plant.WesternLarch:    8400,
		plant.LodgepolePine:   6600,
		plant.WesternRedCedar: 6000,
	}
	for s, want := range cases {
		got, ok := plant.FibreStress(s)
		if !ok || got != want {
			t.Errorf("FibreStress(%s) = %d ok=%v, want %d", s, got, ok, want)
		}
	}
	if _, ok := plant.FibreStress(plant.Species("BAL")); ok {
		t.Error("an untabulated species should report false")
	}
}

func TestClassTopCircumference(t *testing.T) {
	cases := map[plant.Class]int{
		plant.ClassH1: 29,
		plant.Class1:  27,
		plant.Class4:  21,
		plant.Class7:  15,
		plant.Class10: 12,
	}
	for c, want := range cases {
		got, ok := plant.TopCircumference(c)
		if !ok || got != want {
			t.Errorf("TopCircumference(%s) = %d ok=%v, want %d", c, got, ok, want)
		}
	}
	if _, ok := plant.TopCircumference(plant.Class("Q")); ok {
		t.Error("unknown class should report false")
	}
}

func TestClassOrdering(t *testing.T) {
	if !plant.StrongerThan(plant.Class1, plant.Class4) {
		t.Error("class 1 is stronger than class 4")
	}
	if plant.StrongerThan(plant.Class5, plant.Class2) {
		t.Error("class 5 is not stronger than class 2")
	}
	if plant.StrongerThan(plant.Class("Q"), plant.Class4) {
		t.Error("an unknown class is never stronger")
	}
	up, ok := plant.Stronger(plant.Class4)
	if !ok || up != plant.Class3 {
		t.Errorf("next up from 4 = %s ok=%v", up, ok)
	}
	if _, ok := plant.Stronger(plant.ClassH6); ok {
		t.Error("nothing is stronger than H6 in stock")
	}
	if _, ok := plant.Stronger(plant.Class("Q")); ok {
		t.Error("unknown class has no successor")
	}
	all := plant.Classes()
	if len(all) != 15 || all[0] != plant.ClassH6 || all[len(all)-1] != plant.Class10 {
		t.Errorf("class order = %v", all)
	}
	if r, ok := plant.Rank(plant.Class1); !ok || r != 6 {
		t.Errorf("rank of class 1 = %d ok=%v", r, ok)
	}
}

func TestButtCircumferenceTable(t *testing.T) {
	got, err := plant.ButtCircumference(plant.Class4, 40)
	if err != nil || got != 31 {
		t.Errorf("class 4 at 40 ft = %d %v", got, err)
	}
	got, err = plant.ButtCircumference(plant.Class1, 45)
	if err != nil || got != 39 {
		t.Errorf("class 1 at 45 ft = %d %v", got, err)
	}
	if _, err := plant.ButtCircumference(plant.Class("Q"), 40); !errors.Is(err, plant.ErrNoTable) {
		t.Errorf("unknown class error = %v", err)
	}
	if _, err := plant.ButtCircumference(plant.Class4, 95); !errors.Is(err, plant.ErrNoTable) {
		t.Errorf("untabulated length error = %v", err)
	}
}

func TestSectionModulusOfARound(t *testing.T) {
	got := plant.SectionModulus(31)
	want := math.Pow(31, 3) / (32 * math.Pi * math.Pi)
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("modulus = %v, want %v", got, want)
	}
	if plant.SectionModulus(0) != 0 {
		t.Error("a zero section has no modulus")
	}
	if plant.SectionModulus(40) <= plant.SectionModulus(31) {
		t.Error("a fatter pole has a larger modulus")
	}
}

func TestGroundlineCapacity(t *testing.T) {
	p := plant.Pole{
		ID: "NW-441829", Length: 40, Class: plant.Class4,
		Species: plant.SouthernPine, SetDepth: unit.Feet(6),
		Owner: "SUMMIT VALLEY ELECTRIC", Condition: plant.ConditionSound,
	}
	got, err := plant.GroundlineCapacity(p)
	if err != nil {
		t.Fatal(err)
	}
	want := 8000 * plant.SectionModulus(31) / 12
	if math.Abs(got-want) > 1e-6 {
		t.Errorf("capacity = %v, want %v", got, want)
	}
	cedar := p
	cedar.Species = plant.WesternRedCedar
	weaker, err := plant.GroundlineCapacity(cedar)
	if err != nil {
		t.Fatal(err)
	}
	if weaker >= got {
		t.Error("cedar at the same class should carry less moment than pine")
	}
	bad := p
	bad.Species = plant.Species("BAL")
	if _, err := plant.GroundlineCapacity(bad); !errors.Is(err, plant.ErrNoTable) {
		t.Errorf("unknown species error = %v", err)
	}
	bad = p
	bad.Length = 95
	if _, err := plant.GroundlineCapacity(bad); !errors.Is(err, plant.ErrNoTable) {
		t.Errorf("untabulated length error = %v", err)
	}
}

func TestCircumferenceTapersUpThePole(t *testing.T) {
	p := plant.Pole{
		ID: "NW-441829", Length: 40, Class: plant.Class4,
		Species: plant.SouthernPine, SetDepth: unit.Feet(6),
		Owner: "SUMMIT VALLEY ELECTRIC", Condition: plant.ConditionSound,
	}
	atGrade, err := plant.CircumferenceAt(p, 0)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(atGrade-31) > 1e-9 {
		t.Errorf("at grade = %v, want 31", atGrade)
	}
	atTop, err := plant.CircumferenceAt(p, 34)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(atTop-21) > 1e-9 {
		t.Errorf("at top = %v, want 21", atTop)
	}
	mid, err := plant.CircumferenceAt(p, 17)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(mid-26) > 1e-9 {
		t.Errorf("mid pole = %v, want 26", mid)
	}
	below, err := plant.CircumferenceAt(p, -5)
	if err != nil || math.Abs(below-31) > 1e-9 {
		t.Errorf("below grade should clamp to the groundline, got %v %v", below, err)
	}
	above, err := plant.CircumferenceAt(p, 99)
	if err != nil || math.Abs(above-21) > 1e-9 {
		t.Errorf("above the top should clamp to the tip, got %v %v", above, err)
	}
	sunk := p
	sunk.SetDepth = unit.Feet(40)
	if _, err := plant.CircumferenceAt(sunk, 5); err == nil {
		t.Error("a pole with no height above grade should be rejected")
	}
}
