package tests

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/glinskevxer/downguy/internal/cliapp"
)

const planJSON = `{
  "application": "APP-1042",
  "items": [
    {"id": "MR-0001", "action": "raise", "pole": "NW-441829", "subject": "A3",
     "owner": "RIDGEPORT CABLEVISION", "from_ft": "20.0", "to_ft": "20.6",
     "because": ["NESC-235C2|NW-441829||A3|A7"], "detail": "open 0.6 ft"},
    {"id": "MR-0002", "action": "add_guy", "pole": "NW-441829",
     "owner": "SUMMIT VALLEY ELECTRIC", "because": ["NESC-261A|NW-441829|||"],
     "detail": "guy the pull"}
  ]
}`

func TestEstimatePricesAPlan(t *testing.T) {
	got := cliIn(planJSON, "estimate", "-on", "2025-06-01")
	if got.Code != cliapp.CodeOK {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	for _, want := range []string{"APP-1042", "LBR-COMM-HR", "MAT-ANCHOR", "direct cost"} {
		if !strings.Contains(got.Stdout, want) {
			t.Errorf("stdout does not mention %q:\n%s", want, got.Stdout)
		}
	}
	if strings.Contains(got.Stdout, "good until") {
		t.Error("a plain estimate is not a quotation")
	}
}

func TestEstimateWithAQuote(t *testing.T) {
	got := cliIn(planJSON, "estimate", "-on", "2025-06-01", "-quote")
	if got.Code != cliapp.CodeOK {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	for _, want := range []string{"contingency", "administrative overhead", "good until  2025-08-30"} {
		if !strings.Contains(got.Stdout, want) {
			t.Errorf("stdout does not mention %q:\n%s", want, got.Stdout)
		}
	}
}

func TestEstimateAsJSON(t *testing.T) {
	got := cliIn(planJSON, "estimate", "-on", "2025-06-01", "-quote", "-json")
	if got.Code != cliapp.CodeOK {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	var back cliapp.EstimateJSON
	if err := json.Unmarshal([]byte(got.Stdout), &back); err != nil {
		t.Fatalf("%v\n%s", err, got.Stdout)
	}
	if back.Application != "APP-1042" || back.PricedOn != "2025-06-01" {
		t.Errorf("estimate = %+v", back)
	}
	if len(back.Lines) < 5 {
		t.Errorf("lines = %v", back.Lines)
	}
	if len(back.Adjustments) != 3 {
		t.Errorf("adjustments = %v", back.Adjustments)
	}
	if back.Total == back.Direct {
		t.Error("a quotation should total more than the direct cost")
	}
	if back.GoodUntil == "" {
		t.Error("a quotation should carry an expiry")
	}
	for _, l := range back.Lines {
		if l.Item == "" || l.Code == "" || l.Amount == "" {
			t.Errorf("line = %+v", l)
		}
	}
}

func TestEstimateRepricesAgainstADifferentBook(t *testing.T) {
	old := cliIn(planJSON, "estimate", "-on", "2025-06-01", "-json")
	fresh := cliIn(planJSON, "estimate", "-on", "2026-06-01", "-json")
	if old.Code != cliapp.CodeOK || fresh.Code != cliapp.CodeOK {
		t.Fatalf("old %+v fresh %+v", old, fresh)
	}
	var a, b cliapp.EstimateJSON
	if err := json.Unmarshal([]byte(old.Stdout), &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(fresh.Stdout), &b); err != nil {
		t.Fatal(err)
	}
	if a.Total == b.Total {
		t.Errorf("the 2026 book is dearer: %s against %s", b.Total, a.Total)
	}
}

func TestEstimateReportsWhatItCannotPrice(t *testing.T) {
	body := `{"application":"APP-1","items":[{"id":"MR-1","action":"resag","pole":"NW-441829","subject":"A3","because":["x"]}]}`
	got := cliIn(body, "estimate", "-on", "2025-06-01")
	if got.Code != cliapp.CodeOK {
		t.Fatalf("resagging is priced, so this should be clean: %+v", got)
	}
}

func TestEstimateRejectsBadUse(t *testing.T) {
	cases := []struct {
		stdin string
		args  []string
	}{
		{planJSON, []string{"estimate"}},
		{planJSON, []string{"estimate", "-on", "not-a-date"}},
		{planJSON, []string{"estimate", "-on", "2025-06-01", "extra"}},
		{planJSON, []string{"estimate", "-on", "2024-01-01"}},
		{"not json", []string{"estimate", "-on", "2025-06-01"}},
		{`{"items":[{"id":"MR-1","action":"pray","pole":"NW-441829","because":["x"]}]}`,
			[]string{"estimate", "-on", "2025-06-01"}},
		{`{"items":[{"id":"MR-1","action":"raise","pole":"NW-441829","subject":"A3","from_ft":"high","to_ft":"20.0","because":["x"]}]}`,
			[]string{"estimate", "-on", "2025-06-01"}},
		{`{"items":[{"id":"MR-1","action":"raise","pole":"NW-441829","subject":"A3","from_ft":"20.0","to_ft":"19.0","because":["x"]}]}`,
			[]string{"estimate", "-on", "2025-06-01"}},
		{planJSON, []string{"estimate", "-nonsense"}},
	}
	for _, c := range cases {
		if got := cliIn(c.stdin, c.args...); got.Code != cliapp.CodeUsage {
			t.Errorf("%v gave code %d (%q)", c.args, got.Code, got.Stderr)
		}
	}
}

func TestCostbookPrints(t *testing.T) {
	got := cli("costbook", "-on", "2025-06-01")
	if got.Code != cliapp.CodeOK {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	for _, want := range []string{"MAT-POLE-45-3", "LBR-LINE-HR", "hour", "each"} {
		if !strings.Contains(got.Stdout, want) {
			t.Errorf("stdout does not mention %q:\n%s", want, got.Stdout)
		}
	}
}

func TestCostbookAsJSONAndAcrossThePriceRise(t *testing.T) {
	old := cli("costbook", "-on", "2025-06-01", "-json")
	fresh := cli("costbook", "-on", "2026-06-01", "-json")
	if old.Code != cliapp.CodeOK || fresh.Code != cliapp.CodeOK {
		t.Fatalf("old %+v fresh %+v", old, fresh)
	}
	var a, b []struct {
		Code  string `json:"code"`
		Price string `json:"price"`
	}
	if err := json.Unmarshal([]byte(old.Stdout), &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(fresh.Stdout), &b); err != nil {
		t.Fatal(err)
	}
	if len(a) != len(b) || len(a) == 0 {
		t.Fatalf("books = %d and %d", len(a), len(b))
	}
	same := 0
	for i := range a {
		if a[i].Code != b[i].Code {
			t.Fatalf("the books do not line up: %s against %s", a[i].Code, b[i].Code)
		}
		if a[i].Price == b[i].Price {
			same++
		}
	}
	if same != 0 {
		t.Errorf("%d prices did not move across the rise", same)
	}
}

func TestCostbookRejectsBadUse(t *testing.T) {
	cases := [][]string{
		{"costbook"},
		{"costbook", "-on", "2024-01-01"},
		{"costbook", "-on", "2025-06-01", "extra"},
		{"costbook", "-nonsense"},
	}
	for _, args := range cases {
		if got := cli(args...); got.Code != cliapp.CodeUsage {
			t.Errorf("%v gave code %d (%q)", args, got.Code, got.Stderr)
		}
	}
}
