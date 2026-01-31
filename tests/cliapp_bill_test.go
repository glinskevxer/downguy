package tests

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/glinskevxer/downguy/internal/cliapp"
)

const billingAttachments = `attachment_id,pole_id,owner,kind,height,authorized,in_force_from,in_force_to,voltage
A1,NW-441829,SUMMIT VALLEY ELECTRIC,primary,31.5,y,2001-06-01,,7620
A3,NW-441829,RIDGEPORT CABLEVISION,comm_cable,20.0,y,2019-04-01,,
A4,NW-441829,NORTHVALE TELEPHONE,comm_cable,18.6,y,2012-08-15,,
A7,NW-441829,MERIDIAN FIBER,fiber,17.4,y,2025-07-01,,
B3,NW-441830,RIDGEPORT CABLEVISION,comm_cable,20.0,y,2019-04-01,,
B9,NW-441830,UNIDENTIFIED,comm_cable,17.4,n,2019-04-01,,
`

func billingDB(t *testing.T) string {
	t.Helper()
	path := tempDB(t)
	for _, step := range [][2]string{{poleCSV, "poles"}, {billingAttachments, "attachments"}} {
		if got := cliIn(step[0], "import", "-db", path, step[1]); got.Code != cliapp.CodeOK {
			t.Fatalf("%s: %+v", step[1], got)
		}
	}
	return path
}

func TestBillRunsAYear(t *testing.T) {
	t.Parallel()
	path := billingDB(t)
	got := cli("bill", "-db", path, "-issued", "2026-01-15", "2025")
	if got.Code != cliapp.CodeOK {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	for _, want := range []string{"RB-001", "RIDGEPORT CABLEVISION", "MERIDIAN FIBER", "billed"} {
		if !strings.Contains(got.Stdout, want) {
			t.Errorf("stdout does not mention %q:\n%s", want, got.Stdout)
		}
	}
	if !strings.Contains(got.Stdout, "2025 inventory") {
		t.Errorf("the inventory header should print, got %q", got.Stdout)
	}
}

func TestBillAsJSONWithATelecomAttacher(t *testing.T) {
	t.Parallel()
	path := billingDB(t)
	got := cli("bill", "-db", path, "-json", "-issued", "2026-01-15",
		"-telecom", "northvale telephone", "2025")
	if got.Code != cliapp.CodeOK {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	var back []cliapp.BillJSON
	if err := json.Unmarshal([]byte(got.Stdout), &back); err != nil {
		t.Fatalf("%v\n%s", err, got.Stdout)
	}
	if len(back) != 4 {
		t.Fatalf("bills = %v", back)
	}
	byOwner := map[string]cliapp.BillJSON{}
	for _, b := range back {
		byOwner[b.Owner] = b
		if b.Year != 2025 || b.Total == "" || b.Due != "2026-03-01" {
			t.Errorf("bill = %+v", b)
		}
	}
	if byOwner["NORTHVALE TELEPHONE"].Formula != "telecom" {
		t.Errorf("formula = %q", byOwner["NORTHVALE TELEPHONE"].Formula)
	}
	if byOwner["RIDGEPORT CABLEVISION"].Formula != "cable" {
		t.Errorf("formula = %q", byOwner["RIDGEPORT CABLEVISION"].Formula)
	}
	if byOwner["MERIDIAN FIBER"].Prorated != 1 {
		t.Errorf("the fiber went up in July, so it prorates: %+v", byOwner["MERIDIAN FIBER"])
	}
	if byOwner["UNIDENTIFIED"].Flagged != 1 {
		t.Errorf("the unauthorized attachment should be flagged: %+v", byOwner["UNIDENTIFIED"])
	}
}

func TestBillAsCSV(t *testing.T) {
	t.Parallel()
	path := billingDB(t)
	got := cli("bill", "-db", path, "-csv", "-issued", "2026-01-15", "2025")
	if got.Code != cliapp.CodeOK {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	lines := strings.Split(strings.TrimSpace(got.Stdout), "\n")
	if len(lines) != 6 {
		t.Fatalf("one header and one row per attachment:\n%s", got.Stdout)
	}
	if !strings.HasPrefix(lines[0], "number,year,owner") {
		t.Errorf("header = %q", lines[0])
	}
}

func TestBillCommitWritesTheRun(t *testing.T) {
	t.Parallel()
	path := billingDB(t)
	if got := cli("bill", "-db", path, "-issued", "2026-01-15", "2025"); got.Code != cliapp.CodeOK {
		t.Fatal(got.Stderr)
	}
	committed := cli("bill", "-db", path, "-commit", "-issued", "2026-01-15", "2025")
	if committed.Code != cliapp.CodeOK {
		t.Fatalf("code = %d, stderr = %q", committed.Code, committed.Stderr)
	}
	// Running the same committed billing twice must not double it.
	again := cli("bill", "-db", path, "-commit", "-issued", "2026-01-15", "2025")
	if again.Code != cliapp.CodeOK {
		t.Fatalf("code = %d, stderr = %q", again.Code, again.Stderr)
	}
}

func TestBillAnEarlierYearLeavesOutWhatWasNotUpYet(t *testing.T) {
	t.Parallel()
	path := billingDB(t)
	got := cli("bill", "-db", path, "-json", "-issued", "2025-01-15", "2024")
	if got.Code != cliapp.CodeOK {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	var back []cliapp.BillJSON
	if err := json.Unmarshal([]byte(got.Stdout), &back); err != nil {
		t.Fatal(err)
	}
	if len(back) != 3 {
		t.Fatalf("the fiber went up in 2025, so 2024 bills three attachers: %v", back)
	}
	for _, b := range back {
		if b.Owner == "MERIDIAN FIBER" {
			t.Error("the fiber was not up in 2024")
		}
	}
}

func TestBillOnAYearWithNoFiling(t *testing.T) {
	t.Parallel()
	path := billingDB(t)
	got := cli("bill", "-db", path, "-issued", "2026-01-15", "2019")
	if got.Code != cliapp.CodeUsage {
		t.Errorf("2019 has no filed rate: %+v", got)
	}
}

func TestBillRejectsBadUse(t *testing.T) {
	t.Parallel()
	path := billingDB(t)
	cases := [][]string{
		{"bill", "-db", path},
		{"bill", "-db", path, "-issued", "2026-01-15", "2025", "2026"},
		{"bill", "-db", path, "-issued", "2026-01-15", "twenty"},
		{"bill", "-db", path, "2025"},
		{"bill", "-db", path, "-issued", "not-a-date", "2025"},
		{"bill", "-db", path, "-issued", "2026-01-15", "-prefix", "", "2025"},
		{"bill", "-nonsense"},
	}
	for _, args := range cases {
		if got := cli(args...); got.Code != cliapp.CodeUsage {
			t.Errorf("%v gave code %d (%q)", args, got.Code, got.Stderr)
		}
	}
}

func TestAuditFindsAnUnauthorizedAttachment(t *testing.T) {
	t.Parallel()
	path := billingDB(t)
	got := cli("audit", "-db", path, "-found", "2026-05-01", "2025")
	if got.Code != cliapp.CodeFindings {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	for _, want := range []string{"B9", "UNIDENTIFIED", "penalty", "claimed"} {
		if !strings.Contains(got.Stdout, want) {
			t.Errorf("stdout does not mention %q:\n%s", want, got.Stdout)
		}
	}
}

func TestAuditAsJSONAndSelfReported(t *testing.T) {
	t.Parallel()
	path := billingDB(t)
	audit := cli("audit", "-db", path, "-json", "-found", "2026-05-01", "2025")
	if audit.Code != cliapp.CodeFindings {
		t.Fatalf("code = %d", audit.Code)
	}
	var found []cliapp.ClaimJSON
	if err := json.Unmarshal([]byte(audit.Stdout), &found); err != nil {
		t.Fatalf("%v\n%s", err, audit.Stdout)
	}
	if len(found) != 1 || found[0].Sanction != "penalty" {
		t.Fatalf("claims = %v", found)
	}
	if found[0].Penalty == "$0.00" {
		t.Errorf("an audit claim carries a penalty, got %+v", found[0])
	}

	self := cli("audit", "-db", path, "-json", "-self-reported",
		"-found", "2026-05-01", "-noticed", "2026-04-15", "2025")
	if self.Code != cliapp.CodeFindings {
		t.Fatalf("code = %d", self.Code)
	}
	var reported []cliapp.ClaimJSON
	if err := json.Unmarshal([]byte(self.Stdout), &reported); err != nil {
		t.Fatal(err)
	}
	if reported[0].Sanction != "none" {
		t.Errorf("inside the grace period nothing is owed, got %+v", reported[0])
	}
}

func TestAuditOnACleanSystem(t *testing.T) {
	t.Parallel()
	path := tempDB(t)
	clean := `attachment_id,pole_id,owner,kind,height,authorized,in_force_from
A3,NW-441829,RIDGEPORT CABLEVISION,comm_cable,20.0,y,2019-04-01
`
	for _, step := range [][2]string{{poleCSV, "poles"}, {clean, "attachments"}} {
		if got := cliIn(step[0], "import", "-db", path, step[1]); got.Code != cliapp.CodeOK {
			t.Fatalf("%s: %+v", step[1], got)
		}
	}
	got := cli("audit", "-db", path, "-found", "2026-05-01", "2025")
	if got.Code != cliapp.CodeOK {
		t.Fatalf("code = %d", got.Code)
	}
	if !strings.Contains(got.Stdout, "no unauthorized attachments") {
		t.Errorf("stdout = %q", got.Stdout)
	}
}

func TestAuditRejectsBadUse(t *testing.T) {
	t.Parallel()
	path := billingDB(t)
	cases := [][]string{
		{"audit", "-db", path},
		{"audit", "-db", path, "-found", "2026-05-01", "2025", "2026"},
		{"audit", "-db", path, "2025"},
		{"audit", "-db", path, "-found", "not-a-date", "2025"},
		{"audit", "-db", path, "-found", "2026-05-01", "-noticed", "nope", "2025"},
		{"audit", "-db", path, "-found", "2026-05-01", "twenty"},
		{"audit", "-nonsense"},
	}
	for _, args := range cases {
		if got := cli(args...); got.Code != cliapp.CodeUsage {
			t.Errorf("%v gave code %d (%q)", args, got.Code, got.Stderr)
		}
	}
}
