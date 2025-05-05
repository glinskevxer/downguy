package tests

import (
	"errors"
	"testing"

	"github.com/glinskevxer/downguy/internal/survey"
	"github.com/glinskevxer/downguy/internal/unit"
)

func sampleMeasurement() survey.Measurement {
	return survey.Measurement{
		Subject: "A3", Height: unit.Tenths(200),
		Method: survey.MethodStick, Crew: "CREW 41",
	}
}

func TestMethodTolerances(t *testing.T) {
	cases := map[survey.Method]int{
		survey.MethodLaser:     1,
		survey.MethodBucket:    1,
		survey.MethodStick:     2,
		survey.MethodPhoto:     5,
		survey.MethodRecord:    5,
		survey.MethodEstimated: 10,
	}
	for m, want := range cases {
		got, ok := survey.Tolerance(m)
		if !ok || got.Tenths() != want {
			t.Errorf("Tolerance(%s) = %d ok=%v, want %d", m, got.Tenths(), ok, want)
		}
	}
	if _, ok := survey.Tolerance(survey.Method("guess")); ok {
		t.Error("an unknown method has no tolerance")
	}
	if !survey.KnownMethod(survey.MethodLaser) || survey.KnownMethod(survey.Method("guess")) {
		t.Error("known method check")
	}
}

func TestMethodReliability(t *testing.T) {
	for _, m := range []survey.Method{survey.MethodStick, survey.MethodLaser, survey.MethodBucket} {
		if !m.Reliable() {
			t.Errorf("%s is a field measurement", m)
		}
	}
	for _, m := range []survey.Method{survey.MethodPhoto, survey.MethodEstimated, survey.MethodRecord, survey.Method("guess")} {
		if m.Reliable() {
			t.Errorf("%s is not good enough to decide on", m)
		}
	}
}

func TestParseMethod(t *testing.T) {
	for _, in := range []string{"from_record", "from record", "FROM-RECORD", " From Record "} {
		got, err := survey.ParseMethod(in)
		if err != nil || got != survey.MethodRecord {
			t.Errorf("ParseMethod(%q) = %q %v", in, got, err)
		}
	}
	if _, err := survey.ParseMethod("hunch"); !errors.Is(err, survey.ErrBadMeasurement) {
		t.Errorf("unknown method error = %v", err)
	}
}

func TestMeasurementValidate(t *testing.T) {
	if err := sampleMeasurement().Validate(); err != nil {
		t.Fatalf("a good measurement failed: %v", err)
	}
	cases := map[string]func(*survey.Measurement){
		"no subject": func(m *survey.Measurement) { m.Subject = " " },
		"bad method": func(m *survey.Measurement) { m.Method = survey.Method("hunch") },
		"no height":  func(m *survey.Measurement) { m.Height = 0 },
		"negative":   func(m *survey.Measurement) { m.Height = unit.Tenths(-10) },
		"absurd":     func(m *survey.Measurement) { m.Height = unit.Feet(200) },
		"no crew":    func(m *survey.Measurement) { m.Crew = "" },
	}
	for name, mutate := range cases {
		m := sampleMeasurement()
		mutate(&m)
		if err := m.Validate(); err == nil {
			t.Errorf("%s should have been rejected", name)
		}
	}
}

func TestMeasurementBand(t *testing.T) {
	m := sampleMeasurement()
	low, high := m.Band()
	if low.Tenths() != 198 || high.Tenths() != 202 {
		t.Errorf("stick band = %d to %d", low.Tenths(), high.Tenths())
	}
	m.Method = survey.MethodEstimated
	low, high = m.Band()
	if low.Tenths() != 190 || high.Tenths() != 210 {
		t.Errorf("estimated band = %d to %d", low.Tenths(), high.Tenths())
	}
	m.Method = survey.Method("hunch")
	low, high = m.Band()
	if low != m.Height || high != m.Height {
		t.Error("an unknown method carries no band at all")
	}
}

func TestClearsIsThreeWay(t *testing.T) {
	required := unit.Tenths(200)
	cases := []struct {
		tenths int
		method survey.Method
		want   survey.Verdict
	}{
		{210, survey.MethodStick, survey.VerdictClear},
		{202, survey.MethodStick, survey.VerdictClear},
		{201, survey.MethodStick, survey.VerdictMarginal},
		{200, survey.MethodStick, survey.VerdictMarginal},
		{199, survey.MethodStick, survey.VerdictMarginal},
		{197, survey.MethodStick, survey.VerdictShort},
		{190, survey.MethodStick, survey.VerdictShort},
		{201, survey.MethodLaser, survey.VerdictClear},
		{198, survey.MethodLaser, survey.VerdictShort},
		{195, survey.MethodEstimated, survey.VerdictMarginal},
	}
	for _, c := range cases {
		m := sampleMeasurement()
		m.Height = unit.Tenths(c.tenths)
		m.Method = c.method
		if got := m.Clears(required); got != c.want {
			t.Errorf("%d tenths by %s = %q, want %q", c.tenths, c.method, got, c.want)
		}
	}
}

func TestOnlyShortIsFailing(t *testing.T) {
	if !survey.VerdictShort.Failing() {
		t.Error("short is a failure")
	}
	if survey.VerdictClear.Failing() || survey.VerdictMarginal.Failing() {
		t.Error("clear and marginal are not failures")
	}
}

func TestMeasurementString(t *testing.T) {
	want := "A3 20.0 ft by stick (CREW 41)"
	if got := sampleMeasurement().String(); got != want {
		t.Errorf("String = %q, want %q", got, want)
	}
}
