package tests

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/glinskevxer/downguy/internal/cliapp"
)

func walkApplication(t *testing.T, path, id string, steps [][2]string) {
	t.Helper()
	for _, step := range steps {
		got := cli("application", "-db", path, "-to", step[0], "-on", step[1],
			"-by", "d.orme", "move", id)
		if got.Code != cliapp.CodeOK {
			t.Fatalf("moving %s to %s: code %d, stderr %q", id, step[0], got.Code, got.Stderr)
		}
	}
}

func TestComplianceOnAPromptApplication(t *testing.T) {
	t.Parallel()
	path := tempDB(t)
	openApplication(t, path, "APP-1042", "MERIDIAN FIBER")
	walkApplication(t, path, "APP-1042", [][2]string{
		{"submitted", "2025-03-05"},
		{"accepted", "2025-03-12"},
		{"surveying", "2025-03-20"},
	})
	got := cli("compliance", "-db", path, "-as-of", "2025-04-01", "APP-1042")
	if got.Code != cliapp.CodeOK {
		t.Fatalf("code = %d, stderr = %q, stdout = %q", got.Code, got.Stderr, got.Stdout)
	}
	for _, want := range []string{"APP-1042", "on time", "days with the utility", "surveying"} {
		if !strings.Contains(got.Stdout, want) {
			t.Errorf("stdout does not mention %q:\n%s", want, got.Stdout)
		}
	}
	if !strings.Contains(got.Stdout, "open") {
		t.Errorf("the state it is sitting in should read as open:\n%s", got.Stdout)
	}
}

func TestComplianceCatchesALateClock(t *testing.T) {
	t.Parallel()
	path := tempDB(t)
	openApplication(t, path, "APP-1042", "MERIDIAN FIBER")
	walkApplication(t, path, "APP-1042", [][2]string{
		{"submitted", "2025-03-05"},
		{"accepted", "2025-03-12"},
		{"surveying", "2025-03-20"},
		{"engineered", "2025-07-01"},
	})
	got := cli("compliance", "-db", path, "-json", "-as-of", "2025-07-15", "APP-1042")
	if got.Code != cliapp.CodeFindings {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	var back []cliapp.ComplianceJSON
	if err := json.Unmarshal([]byte(got.Stdout), &back); err != nil {
		t.Fatalf("%v\n%s", err, got.Stdout)
	}
	if len(back) != 1 {
		t.Fatalf("assessments = %v", back)
	}
	row := back[0]
	if row.OnTime {
		t.Error("a survey running from March to July misses the forty five day clock")
	}
	if len(row.Late) != 1 || row.Late[0].State != "surveying" {
		t.Errorf("late = %v", row.Late)
	}
	if row.Late[0].Holder != "utility" || row.Late[0].Overrun <= 0 {
		t.Errorf("late = %+v", row.Late[0])
	}
	if row.Late[0].Cite == "" || row.Late[0].Due == "" {
		t.Errorf("a timed interval should carry its citation and due date, got %+v", row.Late[0])
	}
	if len(row.Intervals) == 0 {
		t.Error("naming one application should print every interval")
	}
}

func TestComplianceSweepsTheLiveApplications(t *testing.T) {
	t.Parallel()
	path := tempDB(t)
	openApplication(t, path, "APP-1042", "MERIDIAN FIBER")
	openApplication(t, path, "APP-1043", "CASCADE BROADBAND")
	walkApplication(t, path, "APP-1042", [][2]string{{"submitted", "2025-03-05"}})
	walkApplication(t, path, "APP-1043", [][2]string{{"submitted", "2025-03-05"}})

	early := cli("compliance", "-db", path, "-as-of", "2025-03-10")
	if early.Code != cliapp.CodeOK {
		t.Fatalf("code = %d, stderr = %q", early.Code, early.Stderr)
	}
	if !strings.Contains(early.Stdout, "inside its clocks") {
		t.Errorf("stdout = %q", early.Stdout)
	}

	late := cli("compliance", "-db", path, "-as-of", "2025-05-01")
	if late.Code != cliapp.CodeFindings {
		t.Fatalf("code = %d", late.Code)
	}
	if !strings.Contains(late.Stdout, "APP-1042") || !strings.Contains(late.Stdout, "APP-1043") {
		t.Errorf("both should be late:\n%s", late.Stdout)
	}
	if strings.Contains(late.Stdout, "draft") {
		t.Errorf("a sweep prints only the late intervals:\n%s", late.Stdout)
	}

	all := cli("compliance", "-db", path, "-all", "-as-of", "2025-05-01")
	if !strings.Contains(all.Stdout, "draft") {
		t.Errorf("with -all every interval prints:\n%s", all.Stdout)
	}
}

func TestComplianceOnAnEmptySweep(t *testing.T) {
	t.Parallel()
	path := tempDB(t)
	got := cli("compliance", "-db", path, "-json", "-as-of", "2025-05-01")
	if got.Code != cliapp.CodeOK {
		t.Errorf("code = %d", got.Code)
	}
	if strings.TrimSpace(got.Stdout) != "[]" {
		t.Errorf("stdout = %q", got.Stdout)
	}
}

func TestComplianceRejectsBadUse(t *testing.T) {
	t.Parallel()
	path := tempDB(t)
	openApplication(t, path, "APP-1042", "MERIDIAN FIBER")
	cases := [][]string{
		{"compliance", "-db", path},
		{"compliance", "-db", path, "-as-of", "not-a-date"},
		{"compliance", "-db", path, "-as-of", "2025-05-01", "APP-1042", "APP-1043"},
		{"compliance", "-db", path, "-as-of", "2019-05-01", "APP-1042"},
		{"compliance", "-nonsense"},
	}
	for _, args := range cases {
		if got := cli(args...); got.Code != cliapp.CodeUsage {
			t.Errorf("%v gave code %d (%q)", args, got.Code, got.Stderr)
		}
	}
	if got := cli("compliance", "-db", path, "-as-of", "2025-05-01", "APP-9999"); got.Code != cliapp.CodeFindings {
		t.Errorf("a missing application gave code %d", got.Code)
	}
}
