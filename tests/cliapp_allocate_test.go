package tests

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glinskevxer/downguy/internal/cliapp"
)

const findingsJSON = `[
  {"rule": "NESC-235C2", "severity": "correct", "pole": "NW-441829", "subject": "A3",
   "against": "A7", "owner": "RIDGEPORT CABLEVISION", "blame": "applicant",
   "measured_ft": "0.4", "required_ft": "1.0", "deficit_ft": "0.6", "detail": "A3 over A7"},
  {"rule": "NESC-261A", "severity": "correct", "pole": "NW-441829", "subject": "NW-441829",
   "owner": "SUMMIT VALLEY ELECTRIC", "blame": "pre_existing",
   "measured_ft": "0.0", "required_ft": "0.0", "deficit_ft": "0.0", "detail": "unguyed"}
]`

const allocPlanJSON = `{
  "application": "APP-1042",
  "items": [
    {"id": "MR-0001", "action": "raise", "pole": "NW-441829", "subject": "A3",
     "owner": "RIDGEPORT CABLEVISION", "from_ft": "20.0", "to_ft": "20.6",
     "because": ["NESC-235C2|NW-441829||A3|A7"], "detail": "open 0.6 ft"},
    {"id": "MR-0002", "action": "add_guy", "pole": "NW-441829",
     "owner": "SUMMIT VALLEY ELECTRIC", "because": ["NESC-261A|NW-441829||NW-441829|"],
     "detail": "guy the pull"}
  ]
}`

func findingsFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "findings.json")
	if err := writeFile(path, findingsJSON); err != nil {
		t.Fatal(err)
	}
	return path
}

func allocateArgs(findings string, extra ...string) []string {
	args := []string{
		"allocate", "-on", "2025-06-01",
		"-owner", "SUMMIT VALLEY ELECTRIC=OWN-1",
		"-applicant", "MERIDIAN FIBER=ATT-9",
		"-attacher", "RIDGEPORT CABLEVISION=ATT-3",
		"-findings", findings,
	}
	return append(args, extra...)
}

func TestAllocateLandsChargesOnParties(t *testing.T) {
	t.Parallel()
	got := cliIn(allocPlanJSON, allocateArgs(findingsFile(t))...)
	if got.Code != cliapp.CodeOK {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	for _, want := range []string{"APP-1042", "federal", "MERIDIAN FIBER", "INVOICED BY PARTY"} {
		if !strings.Contains(got.Stdout, want) {
			t.Errorf("stdout does not mention %q:\n%s", want, got.Stdout)
		}
	}
}

func TestAllocateAsJSON(t *testing.T) {
	t.Parallel()
	got := cliIn(allocPlanJSON, allocateArgs(findingsFile(t), "-json")...)
	if got.Code != cliapp.CodeOK {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	var back cliapp.SplitJSON
	if err := json.Unmarshal([]byte(got.Stdout), &back); err != nil {
		t.Fatalf("%v\n%s", err, got.Stdout)
	}
	if back.Application != "APP-1042" || back.Policy != "federal" {
		t.Errorf("split = %+v", back)
	}
	if len(back.Charges) != 2 {
		t.Fatalf("charges = %v", back.Charges)
	}
	byItem := map[string]cliapp.ChargeJSON{}
	for _, c := range back.Charges {
		byItem[c.Item] = c
	}
	if byItem["MR-0001"].Party != "MERIDIAN FIBER" || byItem["MR-0001"].Payer != "applicant" {
		t.Errorf("the applicant pays for what it caused, got %+v", byItem["MR-0001"])
	}
	if byItem["MR-0002"].Payer != "existing_attacher" {
		t.Errorf("a pre existing condition is not the applicant's, got %+v", byItem["MR-0002"])
	}
	if len(back.Invoices) != 2 {
		t.Errorf("invoices = %v", back.Invoices)
	}
	for _, i := range back.Invoices {
		if i.Number == "" || i.Party == "" || i.Total == "" || i.Due == "" {
			t.Errorf("invoice = %+v", i)
		}
	}
}

func TestTheStatePolicySharesThePreExistingWork(t *testing.T) {
	t.Parallel()
	got := cliIn(allocPlanJSON, allocateArgs(findingsFile(t), "-json", "-policy", "state commission")...)
	if got.Code != cliapp.CodeOK {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	var back cliapp.SplitJSON
	if err := json.Unmarshal([]byte(got.Stdout), &back); err != nil {
		t.Fatal(err)
	}
	if back.Policy != "state commission" {
		t.Errorf("policy = %q", back.Policy)
	}
	shared := 0
	for _, c := range back.Charges {
		if c.Item == "MR-0002" {
			shared++
		}
	}
	if shared != 2 {
		t.Errorf("the guy should be split two ways under this policy, got %d charges", shared)
	}
}

func TestAllocateReportsWhatItCannotBill(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "findings.json")
	if err := writeFile(path, `[]`); err != nil {
		t.Fatal(err)
	}
	got := cliIn(allocPlanJSON, allocateArgs(path)...)
	if got.Code != cliapp.CodeFindings {
		t.Fatalf("code = %d, stdout = %q", got.Code, got.Stdout)
	}
	if !strings.Contains(got.Stderr, "nothing to bill") {
		t.Errorf("stderr = %q", got.Stderr)
	}
}

func TestAllocateRejectsBadUse(t *testing.T) {
	t.Parallel()
	findings := findingsFile(t)
	dir := t.TempDir()
	broken := filepath.Join(dir, "broken.json")
	if err := writeFile(broken, "not json"); err != nil {
		t.Fatal(err)
	}
	empty := filepath.Join(dir, "empty.json")
	if err := writeFile(empty, ""); err != nil {
		t.Fatal(err)
	}
	badHeight := filepath.Join(dir, "height.json")
	if err := writeFile(badHeight, `[{"rule":"NESC-232A","measured_ft":"tall","required_ft":"1.0"}]`); err != nil {
		t.Fatal(err)
	}

	cases := [][]string{
		allocateArgs(findings, "extra"),
		allocateArgs(findings, "-policy", "municipal"),
		allocateArgs(broken),
		allocateArgs(empty),
		allocateArgs(badHeight),
		allocateArgs(filepath.Join(dir, "missing.json")),
		{"allocate", "-on", "2025-06-01", "-findings", findings, "-applicant", "MERIDIAN FIBER=ATT-9"},
		{"allocate", "-on", "2025-06-01", "-findings", findings, "-owner", "SUMMIT VALLEY ELECTRIC=OWN-1"},
		{"allocate", "-on", "2025-06-01", "-findings", findings,
			"-owner", "no-equals", "-applicant", "MERIDIAN FIBER=ATT-9"},
		{"allocate", "-on", "2025-06-01", "-findings", findings,
			"-owner", "SUMMIT VALLEY ELECTRIC=OWN-1", "-applicant", "MERIDIAN FIBER=ATT-9",
			"-attacher", "no-equals"},
		{"allocate", "-owner", "A=1", "-applicant", "B=2", "-findings", findings},
		{"allocate", "-nonsense"},
	}
	for _, args := range cases {
		if got := cliIn(allocPlanJSON, args...); got.Code != cliapp.CodeUsage {
			t.Errorf("%v gave code %d (%q)", args, got.Code, got.Stderr)
		}
	}
}

func TestAllocateReadsAWholeMakereadyDocument(t *testing.T) {
	t.Parallel()
	// The makeready command writes a document with an "unresolved" list on it.
	// Handing that straight in should work rather than needing the array
	// pulled out by hand first.
	path := filepath.Join(t.TempDir(), "plan.json")
	body := `{"application":"APP-1042","items":[],"unresolved":` + findingsJSON + `}`
	if err := writeFile(path, body); err != nil {
		t.Fatal(err)
	}
	got := cliIn(allocPlanJSON, allocateArgs(path, "-json")...)
	if got.Code != cliapp.CodeOK {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	var back cliapp.SplitJSON
	if err := json.Unmarshal([]byte(got.Stdout), &back); err != nil {
		t.Fatal(err)
	}
	if len(back.Charges) != 2 {
		t.Errorf("charges = %v", back.Charges)
	}
}

func TestPoliciesLists(t *testing.T) {
	t.Parallel()
	got := cli("policies")
	if got.Code != cliapp.CodeOK {
		t.Fatalf("code = %d", got.Code)
	}
	for _, want := range []string{"federal", "state commission", "existing_attacher", "shared", "25.00%"} {
		if !strings.Contains(got.Stdout, want) {
			t.Errorf("stdout does not mention %q:\n%s", want, got.Stdout)
		}
	}
	asJSON := cli("policies", "-json")
	if asJSON.Code != cliapp.CodeOK {
		t.Fatal(asJSON.Stderr)
	}
	var back []struct {
		Name             string `json:"name"`
		PreExistingPayer string `json:"pre_existing_payer"`
		ApplicantShare   string `json:"applicant_share"`
	}
	if err := json.Unmarshal([]byte(asJSON.Stdout), &back); err != nil {
		t.Fatal(err)
	}
	if len(back) != 2 {
		t.Fatalf("policies = %v", back)
	}
	if back[0].ApplicantShare != "" {
		t.Errorf("the federal policy shares nothing, got %q", back[0].ApplicantShare)
	}
	if back[1].ApplicantShare == "" {
		t.Error("the state policy carries a share")
	}
	if bad := cli("policies", "extra"); bad.Code != cliapp.CodeUsage {
		t.Errorf("code = %d", bad.Code)
	}
}
