package tests

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/glinskevxer/downguy/internal/apiserver"
	"github.com/glinskevxer/downguy/internal/cliapp"
	"github.com/glinskevxer/downguy/internal/config"
)

func rateAPI(t *testing.T) http.Handler {
	t.Helper()
	s, err := apiserver.New(openStore(t), config.Default())
	if err != nil {
		t.Fatal(err)
	}
	return s.RegisterRates().Handler()
}

func TestRateEndpoint(t *testing.T) {
	t.Parallel()
	rec, body := get(t, rateAPI(t), "/rates/2025")
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", rec.Code, body)
	}
	var back apiserver.RateYearJSON
	if err := json.Unmarshal(body, &back); err != nil {
		t.Fatal(err)
	}
	if back.Year != 2025 || back.Attachers != config.Default().Attachers {
		t.Errorf("year = %+v", back)
	}
	if len(back.Rates) != 2 {
		t.Fatalf("rates = %v", back.Rates)
	}
	if back.Rates[0].Formula != "cable" || back.Rates[1].Formula != "telecom" {
		t.Errorf("formulas = %v", back.Rates)
	}
	for _, r := range back.Rates {
		if r.Annual == "" || r.Monthly == "" || r.SpaceShare == "" || r.PoleCost == "" {
			t.Errorf("rate = %+v", r)
		}
	}
}

func TestRateEndpointTakesAnAttacherCount(t *testing.T) {
	t.Parallel()
	h := rateAPI(t)
	rec, body := get(t, h, "/rates/2025?attachers=3")
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d", rec.Code)
	}
	var few apiserver.RateYearJSON
	if err := json.Unmarshal(body, &few); err != nil {
		t.Fatal(err)
	}
	if few.Attachers != 3 {
		t.Errorf("attachers = %d", few.Attachers)
	}
	_, body = get(t, h, "/rates/2025?attachers=8")
	var many apiserver.RateYearJSON
	if err := json.Unmarshal(body, &many); err != nil {
		t.Fatal(err)
	}
	if few.Rates[1].Annual == many.Rates[1].Annual {
		t.Error("the telecom rate should move with the attacher count")
	}
	for _, bad := range []string{"?attachers=0", "?attachers=99", "?attachers=lots"} {
		rec, _ := get(t, h, "/rates/2025"+bad)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s gave code %d", bad, rec.Code)
		}
	}
}

func TestRateEndpointRejectsBadYears(t *testing.T) {
	t.Parallel()
	h := rateAPI(t)
	rec, _ := get(t, h, "/rates/1999")
	if rec.Code != http.StatusNotFound {
		t.Errorf("code = %d", rec.Code)
	}
	for _, path := range []string{"/rates/nineteen", "/rates/", "/rates/2025/extra"} {
		rec, _ := get(t, h, path)
		if rec.Code != http.StatusBadRequest && rec.Code != http.StatusNotFound {
			t.Errorf("%s gave code %d", path, rec.Code)
		}
	}
}

func TestRulesEndpoint(t *testing.T) {
	t.Parallel()
	h := rateAPI(t)
	rec, body := get(t, h, "/rules")
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d", rec.Code)
	}
	var back []apiserver.RuleJSON
	if err := json.Unmarshal(body, &back); err != nil {
		t.Fatal(err)
	}
	if len(back) < 10 {
		t.Errorf("rules = %d", len(back))
	}
	for _, r := range back {
		if r.ID == "" || r.Section == "" || r.Severity == "" || r.Applies == "" {
			t.Errorf("rule = %+v", r)
		}
	}
	rec, body = get(t, h, "/rules?section=235")
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d", rec.Code)
	}
	var section []apiserver.RuleJSON
	if err := json.Unmarshal(body, &section); err != nil {
		t.Fatal(err)
	}
	if len(section) != 3 {
		t.Errorf("section 235 = %v", section)
	}
	rec, _ = get(t, h, "/rules?section=999")
	if rec.Code != http.StatusNotFound {
		t.Errorf("code = %d", rec.Code)
	}
}

func TestServeCheckPrintsItsRoutes(t *testing.T) {
	t.Parallel()
	path := tempDB(t)
	got := cli("serve", "-db", path, "-check")
	if got.Code != cliapp.CodeOK {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	for _, want := range []string{"would listen on", "/healthz", "/poles", "/rates/", "/rules"} {
		if !strings.Contains(got.Stdout, want) {
			t.Errorf("stdout does not mention %q:\n%s", want, got.Stdout)
		}
	}
}

func TestServeTakesAnAddressOverride(t *testing.T) {
	t.Parallel()
	path := tempDB(t)
	got := cli("serve", "-db", path, "-check", "-address", "0.0.0.0:9999")
	if got.Code != cliapp.CodeOK {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	if !strings.Contains(got.Stdout, "0.0.0.0:9999") {
		t.Errorf("stdout = %q", got.Stdout)
	}
}

func TestServeRejectsBadUse(t *testing.T) {
	t.Parallel()
	path := tempDB(t)
	cases := [][]string{
		{"serve", "-db", path, "-check", "extra"},
		{"serve", "-db", path, "-check", "-address", " "},
		{"serve", "-nonsense"},
	}
	for _, args := range cases {
		if got := cli(args...); got.Code != cliapp.CodeUsage {
			t.Errorf("%v gave code %d (%q)", args, got.Code, got.Stderr)
		}
	}
}
