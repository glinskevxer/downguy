package tests

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/glinskevxer/downguy/internal/cliapp"
	"github.com/glinskevxer/downguy/internal/report"
)

const clearSurveyCSV = `survey_id,pole_id,taken,crew,condition,subject,height,method
SV-9001,NW-441829,2025-05-06,CREW 41,sound,A1,31.5,laser
SV-9001,NW-441829,2025-05-06,CREW 41,sound,A3,20.0,laser
SV-9001,NW-441829,2025-05-06,CREW 41,sound,A9,17.4,stick
`

func surveyedDB(t *testing.T) string {
	t.Helper()
	path := loadedDB(t)
	if got := cliIn(clearSurveyCSV, "import", "-db", path, "surveys"); got.Code != cliapp.CodeOK {
		t.Fatal(got.Stderr)
	}
	return path
}

func TestClearRunsAgainstTheSurvey(t *testing.T) {
	t.Parallel()
	path := surveyedDB(t)
	got := cli("clear", "-db", path, "NW-441829")
	if got.Code != cliapp.CodeOK && got.Code != cliapp.CodeFindings {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	if !strings.Contains(got.Stdout, "NW-441829:") {
		t.Errorf("the survey summary should print, got %q", got.Stdout)
	}
}

func TestClearFindsASafetyZoneViolation(t *testing.T) {
	t.Parallel()
	path := loadedDB(t)
	// Lift the cable to two feet under the primary.
	body := `survey_id,pole_id,taken,crew,condition,subject,height,method
SV-9002,NW-441829,2025-05-06,CREW 41,sound,A1,31.5,laser
SV-9002,NW-441829,2025-05-06,CREW 41,sound,A3,29.5,laser
`
	if got := cliIn(body, "import", "-db", path, "surveys"); got.Code != cliapp.CodeOK {
		t.Fatal(got.Stderr)
	}
	got := cli("clear", "-db", path, "NW-441829")
	if got.Code != cliapp.CodeFindings {
		t.Fatalf("code = %d, stdout = %q", got.Code, got.Stdout)
	}
	if !strings.Contains(got.Stdout, "NESC-235C1") {
		t.Errorf("stdout = %q", got.Stdout)
	}
}

func TestClearAsJSON(t *testing.T) {
	t.Parallel()
	path := loadedDB(t)
	body := `survey_id,pole_id,taken,crew,condition,subject,height,method
SV-9003,NW-441829,2025-05-06,CREW 41,sound,A1,31.5,laser
SV-9003,NW-441829,2025-05-06,CREW 41,sound,A3,29.5,laser
`
	if got := cliIn(body, "import", "-db", path, "surveys"); got.Code != cliapp.CodeOK {
		t.Fatal(got.Stderr)
	}
	got := cli("clear", "-db", path, "-json", "NW-441829")
	if got.Code != cliapp.CodeFindings {
		t.Fatalf("code = %d", got.Code)
	}
	var back []report.FindingJSON
	if err := json.Unmarshal([]byte(got.Stdout), &back); err != nil {
		t.Fatalf("%v\n%s", err, got.Stdout)
	}
	if len(back) == 0 {
		t.Fatal("no findings came back")
	}
	if back[0].Rule != "NESC-235C1" || back[0].Pole != "NW-441829" {
		t.Errorf("finding = %+v", back[0])
	}
}

func TestClearRefusesAStaleSurvey(t *testing.T) {
	t.Parallel()
	path := surveyedDB(t)
	got := cli("clear", "-db", path, "-as-of", "2026-06-01", "NW-441829")
	if got.Code != cliapp.CodeFindings {
		t.Fatalf("code = %d", got.Code)
	}
	if !strings.Contains(got.Stderr, "shelf life") {
		t.Errorf("stderr = %q", got.Stderr)
	}
	allowed := cli("clear", "-db", path, "-as-of", "2026-06-01", "-allow-stale", "NW-441829")
	if allowed.Code == cliapp.CodeUsage {
		t.Errorf("with the override it should run: %+v", allowed)
	}
	fresh := cli("clear", "-db", path, "-as-of", "2025-06-01", "NW-441829")
	if strings.Contains(fresh.Stderr, "shelf life") {
		t.Errorf("a month old survey is not stale: %q", fresh.Stderr)
	}
}

func TestClearNeedsASurvey(t *testing.T) {
	t.Parallel()
	path := loadedDB(t)
	got := cli("clear", "-db", path, "NW-441829")
	if got.Code != cliapp.CodeFindings {
		t.Errorf("code = %d", got.Code)
	}
	if !strings.Contains(got.Stderr, "no survey") {
		t.Errorf("stderr = %q", got.Stderr)
	}
}

func TestClearRejectsBadUse(t *testing.T) {
	t.Parallel()
	path := surveyedDB(t)
	cases := [][]string{
		{"clear", "-db", path},
		{"clear", "-db", path, "NW-441829", "NW-441830"},
		{"clear", "-db", path, "nope"},
		{"clear", "-db", path, "-grade", "A", "NW-441829"},
		{"clear", "-db", path, "-district", "tropical", "NW-441829"},
		{"clear", "-db", path, "-as-of", "not-a-date", "NW-441829"},
		{"clear", "-nonsense"},
	}
	for _, args := range cases {
		if got := cli(args...); got.Code != cliapp.CodeUsage {
			t.Errorf("%v gave code %d (%q)", args, got.Code, got.Stderr)
		}
	}
	if got := cli("clear", "-db", path, "NW-999999"); got.Code != cliapp.CodeFindings {
		t.Errorf("a missing pole gave code %d", got.Code)
	}
}

func TestRulesLists(t *testing.T) {
	t.Parallel()
	got := cli("rules")
	if got.Code != cliapp.CodeOK {
		t.Fatalf("code = %d", got.Code)
	}
	for _, want := range []string{"NESC-235C1", "NESC-232A", "stop", "correct"} {
		if !strings.Contains(got.Stdout, want) {
			t.Errorf("stdout does not mention %q:\n%s", want, got.Stdout)
		}
	}
}

func TestRulesBySection(t *testing.T) {
	t.Parallel()
	got := cli("rules", "-section", "235")
	if got.Code != cliapp.CodeOK {
		t.Fatalf("code = %d", got.Code)
	}
	if !strings.Contains(got.Stdout, "NESC-235C1") {
		t.Errorf("stdout = %q", got.Stdout)
	}
	if strings.Contains(got.Stdout, "NESC-232A") {
		t.Errorf("section 235 should not list section 232, got %q", got.Stdout)
	}
	if missing := cli("rules", "-section", "999"); missing.Code != cliapp.CodeUsage {
		t.Errorf("code = %d", missing.Code)
	}
}

func TestRulesAsJSON(t *testing.T) {
	t.Parallel()
	got := cli("rules", "-json")
	if got.Code != cliapp.CodeOK {
		t.Fatalf("code = %d", got.Code)
	}
	var back []struct {
		ID       string `json:"id"`
		Section  string `json:"section"`
		Severity string `json:"severity"`
		Applies  string `json:"applies_to"`
	}
	if err := json.Unmarshal([]byte(got.Stdout), &back); err != nil {
		t.Fatalf("%v\n%s", err, got.Stdout)
	}
	if len(back) < 10 {
		t.Errorf("rules = %d", len(back))
	}
	for _, r := range back {
		if r.ID == "" || r.Section == "" || r.Severity == "" || r.Applies == "" {
			t.Errorf("rule = %+v", r)
		}
	}
}

func TestRulesRejectsBadUse(t *testing.T) {
	t.Parallel()
	if got := cli("rules", "extra"); got.Code != cliapp.CodeUsage {
		t.Errorf("code = %d", got.Code)
	}
}
