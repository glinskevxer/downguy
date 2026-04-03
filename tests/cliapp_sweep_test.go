package tests

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/glinskevxer/downguy/internal/cliapp"
)

const sweepSurveys = `survey_id,pole_id,taken,crew,condition,subject,height,method
SV-7001,NW-441829,2025-05-06,CREW 41,sound,A1,31.5,laser
SV-7001,NW-441829,2025-05-06,CREW 41,sound,A3,29.5,laser
SV-7001,NW-441829,2025-05-06,CREW 41,sound,A9,17.4,stick
`

func sweepDB(t *testing.T) string {
	t.Helper()
	path := loadedDB(t)
	if got := cliIn(sweepSurveys, "import", "-db", path, "surveys"); got.Code != cliapp.CodeOK {
		t.Fatal(got.Stderr)
	}
	return path
}

func TestSweepFindsAndGroups(t *testing.T) {
	t.Parallel()
	path := sweepDB(t)
	got := cli("sweep", "-db", path)
	if got.Code != cliapp.CodeFindings {
		t.Fatalf("code = %d, stderr = %q, stdout = %q", got.Code, got.Stderr, got.Stdout)
	}
	for _, want := range []string{"poles checked", "BY RULE", "BY ATTACHER", "FINDINGS", "SKIPPED", "NESC-235C1"} {
		if !strings.Contains(got.Stdout, want) {
			t.Errorf("stdout does not mention %q:\n%s", want, got.Stdout)
		}
	}
	if !strings.Contains(got.Stdout, "no survey on file") {
		t.Errorf("the unsurveyed poles should be listed as skipped:\n%s", got.Stdout)
	}
}

func TestSweepAsJSON(t *testing.T) {
	t.Parallel()
	path := sweepDB(t)
	got := cli("sweep", "-db", path, "-json")
	if got.Code != cliapp.CodeFindings {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	var back cliapp.SweepJSON
	if err := json.Unmarshal([]byte(got.Stdout), &back); err != nil {
		t.Fatalf("%v\n%s", err, got.Stdout)
	}
	if back.Poles != 1 {
		t.Errorf("one pole has a survey, got %d", back.Poles)
	}
	if len(back.Skipped) != 2 {
		t.Errorf("skipped = %v", back.Skipped)
	}
	if back.Skipped[0].Pole > back.Skipped[1].Pole {
		t.Errorf("skips should be sorted, got %v", back.Skipped)
	}
	if len(back.Findings) == 0 {
		t.Fatal("no findings")
	}
	total := 0
	for _, n := range back.ByRule {
		total += n
	}
	if total != len(back.Findings) {
		t.Errorf("the rule counts add to %d against %d findings", total, len(back.Findings))
	}
}

func TestSweepAsCSV(t *testing.T) {
	t.Parallel()
	path := sweepDB(t)
	got := cli("sweep", "-db", path, "-csv")
	if got.Code != cliapp.CodeFindings {
		t.Fatalf("code = %d", got.Code)
	}
	lines := strings.Split(strings.TrimSpace(got.Stdout), "\n")
	if len(lines) < 2 {
		t.Fatalf("csv = %q", got.Stdout)
	}
	if !strings.HasPrefix(lines[0], "rule,severity,pole") {
		t.Errorf("header = %q", lines[0])
	}
}

func TestSweepSkipsAStaleSurvey(t *testing.T) {
	t.Parallel()
	path := sweepDB(t)
	got := cli("sweep", "-db", path, "-json", "-as-of", "2026-06-01")
	if got.Code != cliapp.CodeOK {
		t.Fatalf("with every survey stale there is nothing to report: %+v", got)
	}
	var back cliapp.SweepJSON
	if err := json.Unmarshal([]byte(got.Stdout), &back); err != nil {
		t.Fatal(err)
	}
	if back.Poles != 0 {
		t.Errorf("poles checked = %d", back.Poles)
	}
	stale := 0
	for _, s := range back.Skipped {
		if strings.Contains(s.Reason, "shelf life") {
			stale++
		}
	}
	if stale != 1 {
		t.Errorf("skipped = %v", back.Skipped)
	}
	allowed := cli("sweep", "-db", path, "-json", "-as-of", "2026-06-01", "-allow-stale")
	if allowed.Code != cliapp.CodeFindings {
		t.Errorf("with the override the stale pole is checked: %+v", allowed)
	}
}

func TestSweepOneDistrict(t *testing.T) {
	t.Parallel()
	path := sweepDB(t)
	got := cli("sweep", "-db", path, "-json", "-only-district", "SE")
	if got.Code != cliapp.CodeOK {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	var back cliapp.SweepJSON
	if err := json.Unmarshal([]byte(got.Stdout), &back); err != nil {
		t.Fatal(err)
	}
	if back.Poles != 0 || len(back.Skipped) != 1 {
		t.Errorf("the south east has one unsurveyed pole, got %+v", back)
	}
}

func TestSweepOfAnEmptyDatabase(t *testing.T) {
	t.Parallel()
	path := tempDB(t)
	got := cli("sweep", "-db", path)
	if got.Code != cliapp.CodeOK {
		t.Errorf("code = %d", got.Code)
	}
	if !strings.Contains(got.Stdout, "0 poles checked") {
		t.Errorf("stdout = %q", got.Stdout)
	}
}

func TestSweepRejectsBadUse(t *testing.T) {
	t.Parallel()
	path := sweepDB(t)
	cases := [][]string{
		{"sweep", "-db", path, "extra"},
		{"sweep", "-db", path, "-district", "tropical"},
		{"sweep", "-db", path, "-grade", "A"},
		{"sweep", "-db", path, "-as-of", "not-a-date"},
		{"sweep", "-nonsense"},
	}
	for _, args := range cases {
		if got := cli(args...); got.Code != cliapp.CodeUsage {
			t.Errorf("%v gave code %d (%q)", args, got.Code, got.Stderr)
		}
	}
}
