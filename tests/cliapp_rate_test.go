package tests

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/glinskevxer/downguy/internal/cliapp"
)

func TestRatePrintsBothFormulas(t *testing.T) {
	t.Parallel()
	got := cli("rate", "2025")
	if got.Code != cliapp.CodeOK {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	for _, want := range []string{"cable", "telecom", "space factor", "annual"} {
		if !strings.Contains(got.Stdout, want) {
			t.Errorf("stdout does not mention %q:\n%s", want, got.Stdout)
		}
	}
	if strings.Count(got.Stdout, "2025") < 2 {
		t.Errorf("both formulas should print, got %q", got.Stdout)
	}
}

func TestRateForOneFormula(t *testing.T) {
	t.Parallel()
	got := cli("rate", "-formula", "telecom", "2025")
	if got.Code != cliapp.CodeOK {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	if strings.Contains(got.Stdout, "cable") {
		t.Errorf("only the telecom rate was asked for, got %q", got.Stdout)
	}
}

func TestRateExplainsItsWorking(t *testing.T) {
	t.Parallel()
	got := cli("rate", "-formula", "cable", "-explain", "2025")
	if got.Code != cliapp.CodeOK {
		t.Fatalf("code = %d", got.Code)
	}
	for _, want := range []string{"CABLE FORMULA", "presumed pole", "space factor", "annual rate"} {
		if !strings.Contains(got.Stdout, want) {
			t.Errorf("stdout does not mention %q:\n%s", want, got.Stdout)
		}
	}
}

func TestRateAsJSON(t *testing.T) {
	t.Parallel()
	got := cli("rate", "-json", "-attachers", "3", "2026")
	if got.Code != cliapp.CodeOK {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	var back []cliapp.RateJSON
	if err := json.Unmarshal([]byte(got.Stdout), &back); err != nil {
		t.Fatalf("%v\n%s", err, got.Stdout)
	}
	if len(back) != 2 {
		t.Fatalf("rates = %v", back)
	}
	for _, r := range back {
		if r.Year != 2026 || r.Attachers != 3 {
			t.Errorf("rate = %+v", r)
		}
		if r.Annual == "" || r.Monthly == "" || r.PoleCost == "" {
			t.Errorf("rate = %+v", r)
		}
		if r.Working != nil {
			t.Errorf("the working should only come when asked for, got %v", r.Working)
		}
	}
	if back[0].Formula != "cable" || back[1].Formula != "telecom" {
		t.Errorf("formulas = %q and %q", back[0].Formula, back[1].Formula)
	}
}

func TestRateJSONCarriesTheWorkingWhenAsked(t *testing.T) {
	t.Parallel()
	got := cli("rate", "-json", "-explain", "-formula", "telecom", "2025")
	if got.Code != cliapp.CodeOK {
		t.Fatal(got.Stderr)
	}
	var back []cliapp.RateJSON
	if err := json.Unmarshal([]byte(got.Stdout), &back); err != nil {
		t.Fatal(err)
	}
	if len(back) != 1 || len(back[0].Working) != 6 {
		t.Errorf("working = %v", back)
	}
}

func TestTheAttacherCountMovesTheTelecomRate(t *testing.T) {
	t.Parallel()
	few := cli("rate", "-json", "-formula", "telecom", "-attachers", "2", "2025")
	many := cli("rate", "-json", "-formula", "telecom", "-attachers", "8", "2025")
	if few.Code != cliapp.CodeOK || many.Code != cliapp.CodeOK {
		t.Fatalf("few %+v many %+v", few, many)
	}
	var a, b []cliapp.RateJSON
	if err := json.Unmarshal([]byte(few.Stdout), &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(many.Stdout), &b); err != nil {
		t.Fatal(err)
	}
	if a[0].Annual == b[0].Annual {
		t.Errorf("more attachers should mean a smaller share each: %s and %s", a[0].Annual, b[0].Annual)
	}
}

func TestRateRejectsBadUse(t *testing.T) {
	t.Parallel()
	cases := [][]string{
		{"rate"},
		{"rate", "2025", "2026"},
		{"rate", "twenty twenty five"},
		{"rate", "1999"},
		{"rate", "-formula", "wireless", "2025"},
		{"rate", "-attachers", "99", "2025"},
		{"rate", "-nonsense"},
	}
	for _, args := range cases {
		if got := cli(args...); got.Code != cliapp.CodeUsage {
			t.Errorf("%v gave code %d (%q)", args, got.Code, got.Stderr)
		}
	}
}
