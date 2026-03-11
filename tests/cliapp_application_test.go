package tests

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/glinskevxer/downguy/internal/cliapp"
)

func openApplication(t *testing.T, path, id, applicant string) {
	t.Helper()
	got := cli("application", "-db", path,
		"-applicant", applicant, "-owner", "SUMMIT VALLEY ELECTRIC",
		"-pole", "NW-441829", "-pole", "NW-441830",
		"-on", "2025-03-03", "-by", "j.aldana",
		"open", id)
	if got.Code != cliapp.CodeOK {
		t.Fatalf("open %s: code %d, stderr %q", id, got.Code, got.Stderr)
	}
}

func TestApplicationOpenAndShow(t *testing.T) {
	t.Parallel()
	path := tempDB(t)
	openApplication(t, path, "APP-1042", "MERIDIAN FIBER")

	got := cli("application", "-db", path, "show", "APP-1042")
	if got.Code != cliapp.CodeOK {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	for _, want := range []string{"APP-1042", "MERIDIAN FIBER", "draft", "NW-441829, NW-441830", "j.aldana"} {
		if !strings.Contains(got.Stdout, want) {
			t.Errorf("stdout does not mention %q:\n%s", want, got.Stdout)
		}
	}
}

func TestApplicationMoveThroughTheMachine(t *testing.T) {
	t.Parallel()
	path := tempDB(t)
	openApplication(t, path, "APP-1042", "MERIDIAN FIBER")

	steps := [][2]string{
		{"submitted", "2025-03-05"},
		{"accepted", "2025-03-12"},
		{"surveying", "2025-03-20"},
	}
	for _, step := range steps {
		got := cli("application", "-db", path, "-to", step[0], "-on", step[1],
			"-by", "d.orme", "move", "APP-1042")
		if got.Code != cliapp.CodeOK {
			t.Fatalf("moving to %s: code %d, stderr %q", step[0], got.Code, got.Stderr)
		}
	}
	shown := cli("application", "-db", path, "-json", "show", "APP-1042")
	if shown.Code != cliapp.CodeOK {
		t.Fatal(shown.Stderr)
	}
	var back cliapp.ApplicationJSON
	if err := json.Unmarshal([]byte(shown.Stdout), &back); err != nil {
		t.Fatalf("%v\n%s", err, shown.Stdout)
	}
	if back.State != "surveying" || back.HeldBy != "utility" || !back.Live {
		t.Errorf("application = %+v", back)
	}
	if len(back.History) != 4 {
		t.Fatalf("history = %v", back.History)
	}
	if back.History[3].To != "surveying" || back.History[3].By != "d.orme" {
		t.Errorf("last event = %+v", back.History[3])
	}
	if len(back.Poles) != 2 {
		t.Errorf("poles = %v", back.Poles)
	}
}

func TestApplicationRefusesAnIllegalMove(t *testing.T) {
	t.Parallel()
	path := tempDB(t)
	openApplication(t, path, "APP-1042", "MERIDIAN FIBER")
	got := cli("application", "-db", path, "-to", "closed", "-on", "2025-03-05",
		"-by", "d.orme", "move", "APP-1042")
	if got.Code != cliapp.CodeUsage {
		t.Fatalf("code = %d", got.Code)
	}
	if !strings.Contains(got.Stderr, "cannot go from draft to closed") {
		t.Errorf("stderr = %q", got.Stderr)
	}
	// And the refused move left nothing behind.
	shown := cli("application", "-db", path, "-json", "show", "APP-1042")
	var back cliapp.ApplicationJSON
	if err := json.Unmarshal([]byte(shown.Stdout), &back); err != nil {
		t.Fatal(err)
	}
	if back.State != "draft" || len(back.History) != 1 {
		t.Errorf("application = %+v", back)
	}
}

func TestApplicationRefusesABackwardsDate(t *testing.T) {
	t.Parallel()
	path := tempDB(t)
	openApplication(t, path, "APP-1042", "MERIDIAN FIBER")
	if got := cli("application", "-db", path, "-to", "submitted", "-on", "2025-03-05",
		"-by", "d.orme", "move", "APP-1042"); got.Code != cliapp.CodeOK {
		t.Fatal(got.Stderr)
	}
	got := cli("application", "-db", path, "-to", "accepted", "-on", "2025-03-01",
		"-by", "d.orme", "move", "APP-1042")
	if got.Code != cliapp.CodeUsage {
		t.Errorf("code = %d", got.Code)
	}
}

func TestApplicationList(t *testing.T) {
	t.Parallel()
	path := tempDB(t)
	openApplication(t, path, "APP-1042", "MERIDIAN FIBER")
	openApplication(t, path, "APP-1043", "CASCADE BROADBAND")
	if got := cli("application", "-db", path, "-to", "withdrawn", "-on", "2025-04-01",
		"-by", "j.aldana", "move", "APP-1043"); got.Code != cliapp.CodeOK {
		t.Fatal(got.Stderr)
	}

	live := cli("application", "-db", path, "list")
	if live.Code != cliapp.CodeOK {
		t.Fatalf("code = %d, stderr = %q", live.Code, live.Stderr)
	}
	if !strings.Contains(live.Stdout, "APP-1042") || strings.Contains(live.Stdout, "APP-1043") {
		t.Errorf("only the live one should be listed:\n%s", live.Stdout)
	}

	byState := cli("application", "-db", path, "-state", "withdrawn", "-json", "list")
	var back []cliapp.ApplicationJSON
	if err := json.Unmarshal([]byte(byState.Stdout), &back); err != nil {
		t.Fatal(err)
	}
	if len(back) != 1 || back[0].ID != "APP-1043" || back[0].Live {
		t.Errorf("by state = %v", back)
	}

	byApplicant := cli("application", "-db", path, "-applicant", "MERIDIAN FIBER", "list")
	if !strings.Contains(byApplicant.Stdout, "APP-1042") {
		t.Errorf("by applicant:\n%s", byApplicant.Stdout)
	}
}

func TestApplicationListOnAnEmptyDatabase(t *testing.T) {
	t.Parallel()
	path := tempDB(t)
	got := cli("application", "-db", path, "list")
	if got.Code != cliapp.CodeFindings {
		t.Errorf("code = %d", got.Code)
	}
	if !strings.Contains(got.Stdout, "no applications") {
		t.Errorf("stdout = %q", got.Stdout)
	}
	asJSON := cli("application", "-db", path, "-json", "list")
	if asJSON.Code != cliapp.CodeFindings {
		t.Errorf("code = %d", asJSON.Code)
	}
	if strings.TrimSpace(asJSON.Stdout) != "[]" {
		t.Errorf("stdout = %q", asJSON.Stdout)
	}
}

func TestApplicationRejectsBadUse(t *testing.T) {
	t.Parallel()
	path := tempDB(t)
	openApplication(t, path, "APP-1042", "MERIDIAN FIBER")
	cases := [][]string{
		{"application", "-db", path},
		{"application", "-db", path, "frobnicate", "APP-1042"},
		{"application", "-db", path, "show"},
		{"application", "-db", path, "show", "APP-1042", "APP-1043"},
		{"application", "-db", path, "list", "APP-1042"},
		{"application", "-db", path, "-state", "pending", "list"},
		{"application", "-db", path, "-on", "2025-03-03", "-by", "x", "open", "APP-2"},
		{"application", "-db", path, "-applicant", "A", "-owner", "B", "-pole", "nope",
			"-on", "2025-03-03", "-by", "x", "open", "APP-2"},
		{"application", "-db", path, "-applicant", "A", "-owner", "B", "-pole", "NW-441829",
			"-on", "2025-03-03", "open", "APP-2"},
		{"application", "-db", path, "-applicant", "A", "-owner", "B", "-pole", "NW-441829",
			"-by", "x", "open", "APP-2"},
		{"application", "-db", path, "-to", "pending", "-on", "2025-03-05", "-by", "x", "move", "APP-1042"},
		{"application", "-db", path, "-to", "submitted", "-by", "x", "move", "APP-1042"},
		{"application", "-db", path, "-to", "submitted", "-on", "2025-03-05", "move", "APP-1042"},
		{"application", "-nonsense"},
	}
	for _, args := range cases {
		if got := cli(args...); got.Code != cliapp.CodeUsage {
			t.Errorf("%v gave code %d (%q)", args, got.Code, got.Stderr)
		}
	}
	if got := cli("application", "-db", path, "show", "APP-9999"); got.Code != cliapp.CodeFindings {
		t.Errorf("a missing application gave code %d", got.Code)
	}
	if got := cli("application", "-db", path, "-to", "submitted", "-on", "2025-03-05",
		"-by", "x", "move", "APP-9999"); got.Code != cliapp.CodeFindings {
		t.Errorf("moving a missing application gave code %d", got.Code)
	}
}

func TestARepeatedOptionRefusesAnEmptyValue(t *testing.T) {
	t.Parallel()
	path := tempDB(t)
	got := cli("application", "-db", path,
		"-applicant", "A", "-owner", "B", "-pole", "  ",
		"-on", "2025-03-03", "-by", "x", "open", "APP-2")
	if got.Code != cliapp.CodeUsage {
		t.Errorf("code = %d", got.Code)
	}
	if !strings.Contains(got.Stderr, "an empty value") {
		t.Errorf("stderr = %q", got.Stderr)
	}
}
