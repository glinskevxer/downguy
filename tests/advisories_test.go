package tests

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// acceptanceRegister is the shape of security-advisories.json.
type acceptanceRegister struct {
	Policy      string       `json:"policy"`
	Acceptances []acceptance `json:"acceptances"`
}

type acceptance struct {
	Scope      string   `json:"scope"`
	Reason     string   `json:"reason"`
	Assessed   string   `json:"assessed"`
	AssessedBy string   `json:"assessed_by"`
	ReviewBy   string   `json:"review_by"`
	Checked    []string `json:"checked"`
}

func loadRegister(t *testing.T) acceptanceRegister {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(repoRoot(t), "security-advisories.json"))
	if err != nil {
		t.Fatal(err)
	}
	var register acceptanceRegister
	if err := json.Unmarshal(body, &register); err != nil {
		t.Fatalf("the register is not valid json: %v", err)
	}
	return register
}

func TestTheAcceptanceRegisterIsComplete(t *testing.T) {
	t.Parallel()
	register := loadRegister(t)
	if register.Policy == "" {
		t.Error("the register states no policy")
	}
	if len(register.Acceptances) == 0 {
		t.Fatal("the register accepts nothing, so it should not exist")
	}
	for _, entry := range register.Acceptances {
		if entry.Scope == "" || entry.Reason == "" || entry.AssessedBy == "" {
			t.Errorf("acceptance = %+v", entry)
		}
		if len(entry.Checked) == 0 {
			t.Errorf("the %s acceptance records nothing that was actually checked", entry.Scope)
		}
		for _, field := range []struct {
			name  string
			value string
		}{{"assessed", entry.Assessed}, {"review_by", entry.ReviewBy}} {
			if _, err := time.Parse("2006-01-02", field.value); err != nil {
				t.Errorf("the %s acceptance has %s = %q", entry.Scope, field.name, field.value)
			}
		}
		assessed, err := time.Parse("2006-01-02", entry.Assessed)
		if err != nil {
			continue
		}
		review, err := time.Parse("2006-01-02", entry.ReviewBy)
		if err != nil {
			continue
		}
		if !review.After(assessed) {
			t.Errorf("the %s acceptance is reviewed on or before the day it was written", entry.Scope)
		}
		if review.Sub(assessed) > 400*24*time.Hour {
			t.Errorf("the %s acceptance runs %v, which is longer than anybody remembers a decision",
				entry.Scope, review.Sub(assessed))
		}
	}
}

func TestTheAcceptanceHasNotExpired(t *testing.T) {
	t.Parallel()
	for _, entry := range loadRegister(t).Acceptances {
		review, err := time.Parse("2006-01-02", entry.ReviewBy)
		if err != nil {
			continue
		}
		if review.Before(time.Now()) {
			t.Errorf("the %s acceptance was due for review on %s, so the advisory gate is red by design",
				entry.Scope, entry.ReviewBy)
		}
	}
}

func TestTheAdvisoryGateRefusesAnExpiredAcceptance(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("no bash on this machine")
	}
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("no python3 on this machine")
	}
	root := repoRoot(t)
	body, err := os.ReadFile(filepath.Join(root, "security-advisories.json"))
	if err != nil {
		t.Fatal(err)
	}
	register := filepath.Join(t.TempDir(), "register.json")
	if err := os.WriteFile(register, body, 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", filepath.Join(root, "scripts", "advisories.sh"))
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "REGISTER="+register, "TODAY=2099-01-01")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("an acceptance a lifetime past its review date should fail the gate:\n%s", out)
	}
	if !strings.Contains(string(out), "due for review") {
		// Without the scanner installed the gate fails for that reason first,
		// which is still a refusal rather than a pass.
		if !strings.Contains(string(out), "govulncheck is not installed") {
			t.Errorf("output = %s", out)
		}
	}
}

func TestTheWorkflowRunsTheAdvisoryGateRatherThanTheRawScanner(t *testing.T) {
	t.Parallel()
	w := loadWorkflow(t)
	var script strings.Builder
	for _, step := range w.Jobs["advisories"].Steps {
		script.WriteString(step.Run)
		script.WriteString("\n")
	}
	text := script.String()
	if !strings.Contains(text, "make advisories") {
		t.Error("the advisories job should go through the gate, not straight to the scanner")
	}
	if !strings.Contains(text, "govulncheck@v") {
		t.Error("the scanner should be installed at a pinned version")
	}
	makefile, err := os.ReadFile(filepath.Join(repoRoot(t), "Makefile"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(makefile), "advisories.sh") {
		t.Error("the makefile has no advisories target")
	}
}
