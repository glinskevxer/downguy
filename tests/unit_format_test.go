package tests

import (
	"testing"

	"github.com/glinskevxer/downguy/internal/unit"
)

func TestWhole(t *testing.T) {
	t.Parallel()
	cases := map[int]string{0: "0", 7: "7", 225: "225", -12: "-12"}
	for in, want := range cases {
		if got := unit.Whole(in); got != want {
			t.Errorf("Whole(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestDecimalPrintsOnePlaceWithNoUnit(t *testing.T) {
	t.Parallel()
	cases := map[int]string{
		0: "0.0", 5: "0.5", 200: "20.0", 186: "18.6",
		-8: "-0.8", -236: "-23.6",
	}
	for tenths, want := range cases {
		if got := unit.Decimal(unit.Tenths(tenths)); got != want {
			t.Errorf("Decimal(%d) = %q, want %q", tenths, got, want)
		}
	}
	// What Decimal writes has to read back as the same height, or an export
	// cannot be fed to the importer.
	for _, tenths := range []int{0, 1, 9, 200, 1234, -7, -456} {
		text := unit.Decimal(unit.Tenths(tenths))
		back, err := unit.ParseHeight(text)
		if err != nil {
			t.Fatalf("Decimal(%d) = %q, which will not parse: %v", tenths, text, err)
		}
		if back.Tenths() != tenths {
			t.Errorf("%q read back as %d tenths, want %d", text, back.Tenths(), tenths)
		}
	}
}

func TestPlainPrintsMoneyASpreadsheetCanRead(t *testing.T) {
	t.Parallel()
	cases := map[int64]string{
		0: "0.00", 5: "0.05", 1234: "12.34", 125000: "1250.00", -550: "-5.50",
	}
	for cents, want := range cases {
		if got := unit.Plain(unit.Cents(cents)); got != want {
			t.Errorf("Plain(%d) = %q, want %q", cents, got, want)
		}
	}
	for _, cents := range []int64{0, 1, 99, 100, 1234567, -42} {
		text := unit.Plain(unit.Cents(cents))
		back, err := unit.ParseMoney(text)
		if err != nil {
			t.Fatalf("Plain(%d) = %q, which will not parse: %v", cents, text, err)
		}
		if back.AsCents() != cents {
			t.Errorf("%q read back as %d cents, want %d", text, back.AsCents(), cents)
		}
	}
}

func TestRoundToWholeGoesAwayFromZero(t *testing.T) {
	t.Parallel()
	cases := map[float64]float64{
		0: 0, 0.4: 0, 0.5: 1, 1.49: 1, 1.5: 2,
		-0.4: 0, -0.5: -1, -1.5: -2, 12345.6: 12346,
	}
	for in, want := range cases {
		if got := unit.RoundToWhole(in); got != want {
			t.Errorf("RoundToWhole(%v) = %v, want %v", in, got, want)
		}
	}
}

func TestTheFormattersAgreeWithTheTypesOwnStrings(t *testing.T) {
	t.Parallel()
	// String carries the unit and Decimal does not, but the number in them has
	// to be the same or a table and a csv of the same field disagree.
	for _, tenths := range []int{0, 5, 186, -236} {
		h := unit.Tenths(tenths)
		if unit.Decimal(h)+" ft" != h.String() {
			t.Errorf("Decimal gives %q and String gives %q", unit.Decimal(h), h.String())
		}
	}
	for _, cents := range []int64{0, 5, 1234, 125000} {
		m := unit.Cents(cents)
		if "$"+unit.Plain(m) != m.String() {
			t.Errorf("Plain gives %q and String gives %q", unit.Plain(m), m.String())
		}
	}
}
