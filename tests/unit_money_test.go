package tests

import (
	"errors"
	"testing"

	"github.com/glinskevxer/downguy/internal/unit"
)

func TestMoneyConstructionAndPrint(t *testing.T) {
	if unit.Dollars(1250).AsCents() != 125000 {
		t.Fatalf("Dollars(1250) = %d", unit.Dollars(1250).AsCents())
	}
	cases := []struct {
		m    unit.Money
		want string
	}{
		{unit.Cents(0), "$0.00"},
		{unit.Cents(5), "$0.05"},
		{unit.Cents(1234), "$12.34"},
		{unit.Cents(-1234), "-$12.34"},
		{unit.Dollars(1250), "$1250.00"},
	}
	for _, c := range cases {
		if got := c.m.String(); got != c.want {
			t.Errorf("%d.String() = %q, want %q", c.m.AsCents(), got, c.want)
		}
	}
}

func TestMoneyArithmetic(t *testing.T) {
	a := unit.Dollars(400)
	b := unit.Cents(2575)
	if got := a.Add(b).AsCents(); got != 42575 {
		t.Errorf("add = %d", got)
	}
	if got := a.Sub(b).AsCents(); got != 37425 {
		t.Errorf("sub = %d", got)
	}
	if got := b.Times(4).AsCents(); got != 10300 {
		t.Errorf("times = %d", got)
	}
	if got := b.Neg().AsCents(); got != -2575 {
		t.Errorf("neg = %d", got)
	}
	if !unit.Cents(0).IsZero() {
		t.Error("zero should be zero")
	}
	if unit.Cents(1).IsZero() {
		t.Error("one cent is not zero")
	}
	if got := unit.SumMoney([]unit.Money{a, b, unit.Cents(-100)}).AsCents(); got != 42475 {
		t.Errorf("sum = %d", got)
	}
	if got := b.Float(); got != 25.75 {
		t.Errorf("float = %v", got)
	}
}

func TestParseMoneyForms(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"12.34", 1234},
		{"$12.34", 1234},
		{"$ 12.34", 1234},
		{"1,250", 125000},
		{"$1,250.00", 125000},
		{"12.3", 1230},
		{"12", 1200},
		{"-$5.50", -550},
		{".75", 75},
		{"0", 0},
	}
	for _, c := range cases {
		got, err := unit.ParseMoney(c.in)
		if err != nil {
			t.Errorf("ParseMoney(%q): %v", c.in, err)
			continue
		}
		if got.AsCents() != c.want {
			t.Errorf("ParseMoney(%q) = %d, want %d", c.in, got.AsCents(), c.want)
		}
	}
}

func TestParseMoneyRejectsJunk(t *testing.T) {
	for _, in := range []string{"", "  ", "free", "12.345", "$", "1.2.3", "12x"} {
		if _, err := unit.ParseMoney(in); err == nil {
			t.Errorf("ParseMoney(%q) should have failed", in)
		} else if !errors.Is(err, unit.ErrBadMoney) {
			t.Errorf("ParseMoney(%q) gave %v", in, err)
		}
	}
}

func TestBasisPointsParseAndPrint(t *testing.T) {
	cases := []struct {
		in   string
		want int32
		str  string
	}{
		{"7.4%", 740, "7.40%"},
		{"7.4", 740, "7.40%"},
		{"100%", 10000, "100.00%"},
		{"0.05%", 5, "0.05%"},
		{"-2.5%", -250, "-2.50%"},
	}
	for _, c := range cases {
		got, err := unit.ParsePercent(c.in)
		if err != nil {
			t.Errorf("ParsePercent(%q): %v", c.in, err)
			continue
		}
		if int32(got) != c.want {
			t.Errorf("ParsePercent(%q) = %d, want %d", c.in, int32(got), c.want)
		}
		if got.String() != c.str {
			t.Errorf("ParsePercent(%q).String() = %q, want %q", c.in, got.String(), c.str)
		}
	}
	if _, err := unit.ParsePercent("7.456%"); err == nil {
		t.Error("three decimals of percent should be rejected")
	}
	if _, err := unit.ParsePercent(""); err == nil {
		t.Error("empty percent should be rejected")
	}
	if unit.Percent(12) != 1200 {
		t.Errorf("Percent(12) = %d", unit.Percent(12))
	}
	if unit.FullRate.Float() != 1 {
		t.Errorf("FullRate float = %v", unit.FullRate.Float())
	}
}

func TestBasisPointsOfRoundsHalfAway(t *testing.T) {
	cases := []struct {
		rate  int32
		money int64
		want  int64
	}{
		{10000, 12345, 12345},
		{5000, 12345, 6173},
		{5000, -12345, -6173},
		{740, 100000, 7400},
		{1, 100, 0},
		{50, 100, 1},
	}
	for _, c := range cases {
		got := unit.BasisPoints(c.rate).Of(unit.Cents(c.money))
		if got.AsCents() != c.want {
			t.Errorf("%d bp of %d = %d, want %d", c.rate, c.money, got.AsCents(), c.want)
		}
	}
}
