package tests

import (
	"strings"
	"testing"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/rental"
	"github.com/glinskevxer/downguy/internal/unit"
)

func TestFormulaParsing(t *testing.T) {
	for _, in := range []string{"cable", "CABLE", " Cable "} {
		got, err := rental.ParseFormula(in)
		if err != nil || got != rental.FormulaCable {
			t.Errorf("ParseFormula(%q) = %q %v", in, got, err)
		}
	}
	if _, err := rental.ParseFormula("wireless"); err == nil {
		t.Error("there is no wireless formula here")
	}
	if !rental.KnownFormula(rental.FormulaTelecom) || rental.KnownFormula(rental.Formula("wireless")) {
		t.Error("known formula check")
	}
	f := rental.Formulas()
	if len(f) != 2 || f[0] != rental.FormulaCable {
		t.Errorf("formulas = %v", f)
	}
}

func TestStandardRateForBothFormulas(t *testing.T) {
	cable, err := rental.Standard(calendar.RateYear(2025), rental.FormulaCable, 5)
	if err != nil {
		t.Fatal(err)
	}
	telecom, err := rental.Standard(calendar.RateYear(2025), rental.FormulaTelecom, 5)
	if err != nil {
		t.Fatal(err)
	}
	if cable.Annual <= 0 || telecom.Annual <= 0 {
		t.Fatalf("rates = %s and %s", cable.Annual, telecom.Annual)
	}
	if telecom.Annual <= cable.Annual {
		t.Errorf("with five attachers the telecom rate is the higher one: %s against %s", telecom.Annual, cable.Annual)
	}
	// A joint use rate lands in single figure dollars per pole per year.
	for _, r := range []*rental.Rate{cable, telecom} {
		if r.Annual.AsCents() < 100 || r.Annual.AsCents() > 5000 {
			t.Errorf("%s looks wrong: %s", r.Inputs.Formula, r.Annual)
		}
	}
	if cable.String() == "" {
		t.Error("String should say something")
	}
}

func TestRateRisesWithTheFiledCost(t *testing.T) {
	var last unit.Money
	for _, y := range rental.FiledYears() {
		r, err := rental.Standard(calendar.RateYear(y), rental.FormulaCable, 5)
		if err != nil {
			t.Fatal(err)
		}
		if r.Annual <= last {
			t.Errorf("%d rate %s is not above the previous %s", y, r.Annual, last)
		}
		last = r.Annual
	}
}

func TestComputeRejectsBadInput(t *testing.T) {
	s := rental.StandardSpace()
	if _, err := rental.Compute(calendar.RateYear(2025), rental.Formula("wireless"), s, rental.StandardOccupied, 5); err == nil {
		t.Error("an unknown formula should be rejected")
	}
	if _, err := rental.Compute(calendar.RateYear(1999), rental.FormulaCable, s, rental.StandardOccupied, 5); err == nil {
		t.Error("a year with no filing should be rejected")
	}
	if _, err := rental.Compute(calendar.RateYear(2025), rental.FormulaCable, s, 0, 5); err == nil {
		t.Error("occupying nothing should be rejected")
	}
	if _, err := rental.Compute(calendar.RateYear(2025), rental.FormulaTelecom, s, rental.StandardOccupied, 0); err == nil {
		t.Error("no attachers should be rejected")
	}
}

func TestMonthlyIsATwelfth(t *testing.T) {
	r, err := rental.Standard(calendar.RateYear(2025), rental.FormulaCable, 5)
	if err != nil {
		t.Fatal(err)
	}
	if r.Monthly().AsCents() != r.Annual.AsCents()/12 {
		t.Errorf("monthly = %s of %s", r.Monthly(), r.Annual)
	}
}

func TestProratedByTimeOnThePole(t *testing.T) {
	r, err := rental.Standard(calendar.RateYear(2025), rental.FormulaCable, 5)
	if err != nil {
		t.Fatal(err)
	}
	full := calendar.Range{From: calendar.MustNew(2025, 1, 1), To: calendar.MustNew(2025, 12, 31)}
	if got := r.Prorated(full); got != r.Annual {
		t.Errorf("a full year = %s, want %s", got, r.Annual)
	}
	open := calendar.Range{From: calendar.MustNew(2020, 1, 1)}
	if got := r.Prorated(open); got != r.Annual {
		t.Errorf("an open range covering the year = %s", got)
	}
	half := calendar.Range{From: calendar.MustNew(2025, 7, 1), To: calendar.MustNew(2025, 12, 31)}
	got := r.Prorated(half)
	if got >= r.Annual || got.IsZero() {
		t.Errorf("half a year = %s of %s", got, r.Annual)
	}
	// 184 days of 365 rounds to just over half.
	if got.AsCents()*2 < r.Annual.AsCents() {
		t.Errorf("184 days is more than half the year, got %s of %s", got, r.Annual)
	}
	none := calendar.Range{From: calendar.MustNew(2026, 1, 1), To: calendar.MustNew(2026, 12, 31)}
	if !r.Prorated(none).IsZero() {
		t.Errorf("a range outside the year = %s", r.Prorated(none))
	}
}

func TestExplainShowsTheWorking(t *testing.T) {
	r, err := rental.Standard(calendar.RateYear(2025), rental.FormulaTelecom, 5)
	if err != nil {
		t.Fatal(err)
	}
	lines := r.Explain()
	if len(lines) != 6 {
		t.Fatalf("explanation = %v", lines)
	}
	joined := strings.Join(lines, "\n")
	for _, want := range []string{"telecom", "space factor", "annual rate", "bare pole"} {
		if !strings.Contains(joined, want) {
			t.Errorf("the explanation does not mention %q: %s", want, joined)
		}
	}
}

func TestPublishASchedule(t *testing.T) {
	s, err := rental.Publish(calendar.RateYear(2026), 5)
	if err != nil {
		t.Fatal(err)
	}
	if s.Year != 2026 || s.Attachers != 5 {
		t.Errorf("schedule = %+v", s)
	}
	if len(s.Rates) != 2 {
		t.Fatalf("rates = %v", s.Rates)
	}
	cable, ok := s.For(rental.FormulaCable)
	if !ok || cable.Annual.IsZero() {
		t.Errorf("cable = %+v ok=%v", cable, ok)
	}
	if _, ok := s.For(rental.Formula("wireless")); ok {
		t.Error("no wireless rate")
	}
	if _, err := rental.Publish(calendar.RateYear(1999), 5); err == nil {
		t.Error("a year with no filing cannot be published")
	}
}
