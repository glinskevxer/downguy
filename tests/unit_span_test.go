package tests

import (
	"errors"
	"testing"

	"github.com/glinskevxer/downguy/internal/unit"
)

func TestSpanBasics(t *testing.T) {
	s := unit.SpanFeet(225)
	if s.Feet() != 225 {
		t.Fatalf("Feet = %d, want 225", s.Feet())
	}
	if s.Half().Feet() != 112 {
		t.Fatalf("Half = %d, want 112", s.Half().Feet())
	}
	if s.String() != "225 ft" {
		t.Fatalf("String = %q", s.String())
	}
	if s.Float() != 225 {
		t.Fatalf("Float = %v", s.Float())
	}
}

func TestSpanValidRange(t *testing.T) {
	cases := []struct {
		feet int
		want bool
	}{
		{19, false},
		{20, true},
		{225, true},
		{1200, true},
		{1201, false},
		{0, false},
		{-40, false},
	}
	for _, c := range cases {
		if got := unit.SpanFeet(c.feet).Valid(); got != c.want {
			t.Errorf("SpanFeet(%d).Valid() = %v, want %v", c.feet, got, c.want)
		}
	}
}

func TestParseSpanForms(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"225", 225},
		{"225 ft", 225},
		{"225'", 225},
		{" 96 ", 96},
		{"224.4", 224},
		{"224.5", 225},
		{"224.9", 225},
	}
	for _, c := range cases {
		got, err := unit.ParseSpan(c.in)
		if err != nil {
			t.Errorf("ParseSpan(%q): %v", c.in, err)
			continue
		}
		if got.Feet() != c.want {
			t.Errorf("ParseSpan(%q) = %d, want %d", c.in, got.Feet(), c.want)
		}
	}
}

func TestParseSpanRejectsJunk(t *testing.T) {
	for _, in := range []string{"", "  ", "long", "12.", "12.x", "ft"} {
		if _, err := unit.ParseSpan(in); err == nil {
			t.Errorf("ParseSpan(%q) should have failed", in)
		} else if !errors.Is(err, unit.ErrBadSpan) {
			t.Errorf("ParseSpan(%q) error %v is not ErrBadSpan", in, err)
		}
	}
}

func TestTemperatureConstantsAndDelta(t *testing.T) {
	if unit.TempHeavyDistrict.Degrees() != 0 {
		t.Errorf("heavy district temp = %d", unit.TempHeavyDistrict.Degrees())
	}
	if unit.TempMediumDistrict.Degrees() != 15 {
		t.Errorf("medium district temp = %d", unit.TempMediumDistrict.Degrees())
	}
	if unit.TempLightDistrict.Degrees() != 30 {
		t.Errorf("light district temp = %d", unit.TempLightDistrict.Degrees())
	}
	if got := unit.TempMaxOperating.Delta(unit.TempStringing); got != 60 {
		t.Errorf("delta = %d, want 60", got)
	}
	if got := unit.TempHeavyDistrict.Delta(unit.TempStringing); got != -60 {
		t.Errorf("delta = %d, want -60", got)
	}
	if unit.TempStringing.String() != "60 F" {
		t.Errorf("String = %q", unit.TempStringing.String())
	}
	if unit.TempMaxOperating.Float() != 120 {
		t.Errorf("Float = %v", unit.TempMaxOperating.Float())
	}
}
