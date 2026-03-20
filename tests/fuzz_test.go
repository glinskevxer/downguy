package tests

import (
	"strings"
	"testing"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/unit"
)

// The parsers here read files other systems wrote, so they see everything: a
// height column with a footnote in it, a date the export tool localised, an
// identifier with a stray tab. None of them may panic, and each one has an
// invariant that has to hold for whatever it does accept.

func FuzzParseHeight(f *testing.F) {
	for _, seed := range []string{
		"", " ", "0", "23.6", "23.6 ft", "23", `23' 7"`, "23'", "40in",
		"-4.5", "0.04", "abc", `23' 14"`, "23.4567", "12x", ".", "-", "'",
		"999999999999", "1e9", "\t20.0\n", "23 ft 7 in",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, in string) {
		got, err := unit.ParseHeight(in)
		if err != nil {
			return
		}
		// Anything that parsed has to print as a height and read back as the
		// same number of tenths. A parser that accepts a string it cannot
		// round trip is a parser that has quietly lost information.
		back, err := unit.ParseHeight(strings.TrimSuffix(got.String(), " ft"))
		if err != nil {
			t.Fatalf("ParseHeight(%q) = %s, which will not parse again: %v", in, got, err)
		}
		if back != got {
			t.Fatalf("ParseHeight(%q) = %d tenths, which reads back as %d", in, got.Tenths(), back.Tenths())
		}
	})
}

func FuzzParseSpan(f *testing.F) {
	for _, seed := range []string{"", "225", "225 ft", "225'", " 96 ", "224.5", "12.", "long", "-3"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, in string) {
		got, err := unit.ParseSpan(in)
		if err != nil {
			return
		}
		if got.String() != itoaTest(got.Feet())+" ft" {
			t.Fatalf("ParseSpan(%q) = %d, which prints as %q", in, got.Feet(), got.String())
		}
	})
}

func FuzzParseMoney(f *testing.F) {
	for _, seed := range []string{"", "0", "12.34", "$12.34", "1,250", "-$5.50", ".75", "12.345", "free", "$"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, in string) {
		got, err := unit.ParseMoney(in)
		if err != nil {
			return
		}
		back, err := unit.ParseMoney(got.String())
		if err != nil {
			t.Fatalf("ParseMoney(%q) = %s, which will not parse again: %v", in, got, err)
		}
		if back != got {
			t.Fatalf("ParseMoney(%q) = %d cents, which reads back as %d", in, got.AsCents(), back.AsCents())
		}
	})
}

func FuzzParsePercent(f *testing.F) {
	for _, seed := range []string{"", "7.4%", "7.4", "100%", "0.05%", "-2.5%", "7.456%", "%"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, in string) {
		got, err := unit.ParsePercent(in)
		if err != nil {
			return
		}
		back, err := unit.ParsePercent(got.String())
		if err != nil {
			t.Fatalf("ParsePercent(%q) = %s, which will not parse again: %v", in, got, err)
		}
		if back != got {
			t.Fatalf("ParsePercent(%q) = %d, which reads back as %d", in, int32(got), int32(back))
		}
	})
}

func FuzzParseDate(f *testing.F) {
	for _, seed := range []string{
		"", "2025-01-20", "2024-02-29", "2025-02-29", "2025-1-2",
		"20250120", "2025/01/20", "abcd-ef-gh", "0000-00-00", "9999-12-31",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, in string) {
		got, err := calendar.ParseDate(in)
		if err != nil {
			return
		}
		if got.String() != in {
			t.Fatalf("ParseDate(%q) prints as %q", in, got.String())
		}
		// A parsed date has to survive a trip through the time package and
		// back, which is what every date arithmetic here depends on.
		if !calendar.FromTime(got.Time()).Equal(got) {
			t.Fatalf("ParseDate(%q) does not survive a time round trip", in)
		}
	})
}

func FuzzParsePoleID(f *testing.F) {
	for _, seed := range []string{
		"", "NW-441829", "nw-441829", " se-1129043", "441829",
		"N-441829", "NWX-441829", "NW-44182", "NW-11290431", "NW-4418X9", "12-441829",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, in string) {
		got, err := plant.ParsePoleID(in)
		if err != nil {
			return
		}
		if len(got.District()) != 2 {
			t.Fatalf("ParsePoleID(%q) = %q, whose district is %q", in, got, got.District())
		}
		// A tag that parsed is already normalised, so parsing it again is a
		// no op. Anything else means two spellings of one pole reach the
		// database as two poles.
		again, err := plant.ParsePoleID(string(got))
		if err != nil {
			t.Fatalf("ParsePoleID(%q) = %q, which will not parse again: %v", in, got, err)
		}
		if again != got {
			t.Fatalf("ParsePoleID(%q) = %q, which normalises again to %q", in, got, again)
		}
	})
}

func FuzzParseKindAndCover(f *testing.F) {
	for _, seed := range []string{"", "comm_cable", "comm cable", "COMM-CABLE", "road", "water no sail", "swamp"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, in string) {
		if kind, err := plant.ParseKind(in); err == nil {
			if !plant.KnownKind(kind) {
				t.Fatalf("ParseKind(%q) = %q, which is not a known kind", in, kind)
			}
			if kind.IsSupply() == kind.IsCommunication() {
				t.Fatalf("%q is both supply and communication, or neither", kind)
			}
		}
		if cover, err := plant.ParseCover(in); err == nil {
			if !plant.KnownCover(cover) {
				t.Fatalf("ParseCover(%q) = %q, which is not a known cover", in, cover)
			}
			if cover.Name() == "" {
				t.Fatalf("%q has no long form", cover)
			}
		}
	})
}

// itoaTest is a local whole number formatter so the fuzz invariants do not
// depend on the formatting under test.
func itoaTest(n int) string {
	if n == 0 {
		return "0"
	}
	sign := ""
	if n < 0 {
		sign, n = "-", -n
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return sign + digits
}
