package tests

import (
	"errors"
	"testing"

	"github.com/glinskevxer/downguy/internal/unit"
)

func TestFeetBuildsWholeTenths(t *testing.T) {
	if got := unit.Feet(23).Tenths(); got != 230 {
		t.Fatalf("Feet(23) tenths = %d, want 230", got)
	}
	if got := unit.Feet(0).Tenths(); got != 0 {
		t.Fatalf("Feet(0) tenths = %d, want 0", got)
	}
}

func TestInchesRoundsToNearestTenth(t *testing.T) {
	cases := []struct {
		in   int
		want int
	}{
		{0, 0},
		{1, 1},
		{2, 2},
		{6, 5},
		{7, 6},
		{11, 9},
		{12, 10},
		{40, 33},
		{-6, -5},
		{-7, -6},
	}
	for _, c := range cases {
		if got := unit.Inches(c.in).Tenths(); got != c.want {
			t.Errorf("Inches(%d) = %d tenths, want %d", c.in, got, c.want)
		}
	}
}

func TestFeetInchesCombines(t *testing.T) {
	if got := unit.FeetInches(18, 6).Tenths(); got != 185 {
		t.Fatalf("FeetInches(18,6) = %d, want 185", got)
	}
	if got := unit.FeetInches(0, 40).Tenths(); got != 33 {
		t.Fatalf("FeetInches(0,40) = %d, want 33", got)
	}
}

func TestHeightStringPrintsOneDecimal(t *testing.T) {
	cases := []struct {
		h    unit.Height
		want string
	}{
		{unit.Feet(23), "23.0 ft"},
		{unit.Tenths(236), "23.6 ft"},
		{unit.Tenths(-45), "-4.5 ft"},
		{unit.Tenths(0), "0.0 ft"},
	}
	for _, c := range cases {
		if got := c.h.String(); got != c.want {
			t.Errorf("%d.String() = %q, want %q", int(c.h), got, c.want)
		}
	}
}

func TestFeetInchesFormattingCarries(t *testing.T) {
	cases := []struct {
		h    unit.Height
		want string
	}{
		{unit.Tenths(236), `23' 7"`},
		{unit.Tenths(230), `23' 0"`},
		{unit.Tenths(239), `23' 11"`},
		{unit.Tenths(2396), `239' 7"`},
		{unit.Tenths(-236), `-23' 7"`},
	}
	for _, c := range cases {
		if got := c.h.FeetInches(); got != c.want {
			t.Errorf("%d.FeetInches() = %q, want %q", int(c.h), got, c.want)
		}
	}
}

func TestParseHeightReadsEveryFieldForm(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"23.6", 236},
		{"23.6 ft", 236},
		{"23", 230},
		{`23' 7"`, 236},
		{"23'", 230},
		{"40in", 33},
		{"  18.25  ", 183},
		{"-4.5", -45},
		{"0.04", 0},
	}
	for _, c := range cases {
		got, err := unit.ParseHeight(c.in)
		if err != nil {
			t.Errorf("ParseHeight(%q) errored: %v", c.in, err)
			continue
		}
		if got.Tenths() != c.want {
			t.Errorf("ParseHeight(%q) = %d tenths, want %d", c.in, got.Tenths(), c.want)
		}
	}
}

func TestParseHeightRejectsJunk(t *testing.T) {
	for _, in := range []string{"", "  ", "abc", `23' 14"`, "23.4567", "12x"} {
		if _, err := unit.ParseHeight(in); err == nil {
			t.Errorf("ParseHeight(%q) should have failed", in)
		} else if !errors.Is(err, unit.ErrBadHeight) {
			t.Errorf("ParseHeight(%q) error %v is not ErrBadHeight", in, err)
		}
	}
}

func TestHeightArithmetic(t *testing.T) {
	a := unit.Tenths(185)
	b := unit.Tenths(40)
	if got := a.Add(b).Tenths(); got != 225 {
		t.Errorf("add = %d, want 225", got)
	}
	if got := a.Sub(b).Tenths(); got != 145 {
		t.Errorf("sub = %d, want 145", got)
	}
	if got := b.Sub(a).Abs().Tenths(); got != 145 {
		t.Errorf("abs = %d, want 145", got)
	}
	if got := a.WholeFeet(); got != 18 {
		t.Errorf("whole feet = %d, want 18", got)
	}
	if got := a.Float(); got != 18.5 {
		t.Errorf("float = %v, want 18.5", got)
	}
}
