package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// repoRoot returns the checkout root from the tests directory.
func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Dir(wd)
}

func TestEveryShippedScriptIsExecutableAndParses(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(repoRoot(t), "scripts")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("there are no scripts")
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sh") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode()&0o111 == 0 {
			t.Errorf("%s is not executable", entry.Name())
		}
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(string(body), "#!/usr/bin/env bash\n") {
			t.Errorf("%s does not start with a portable shebang", entry.Name())
		}
		if !strings.Contains(string(body), "set -euo pipefail") {
			t.Errorf("%s does not set the strict shell options", entry.Name())
		}
		if _, err := exec.LookPath("bash"); err != nil {
			continue
		}
		cmd := exec.Command("bash", "-n", path)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Errorf("%s does not parse: %v\n%s", entry.Name(), err, out)
		}
	}
}

func TestTheMakefileCoversWhatTheScriptsExpect(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile(filepath.Join(repoRoot(t), "Makefile"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, target := range []string{
		"build:", "fmt-check:", "vet:", "test:", "race:", "shuffle:",
		"cover:", "cover-check:", "tidy-check:", "smoke:", "clean:",
	} {
		if !strings.Contains(text, "\n"+target) {
			t.Errorf("the makefile has no %s target", target)
		}
	}
	for _, script := range []string{"cover-floor.sh", "tidy-check.sh", "smoke.sh"} {
		if !strings.Contains(text, script) {
			t.Errorf("the makefile never runs %s", script)
		}
	}
	if !strings.Contains(text, "internal/version.Release=") {
		t.Error("the build target does not stamp the release")
	}
}

func TestCoverFloorScriptReadsARealProfile(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("no bash on this machine")
	}
	root := repoRoot(t)
	script := filepath.Join(root, "scripts", "cover-floor.sh")

	missing := exec.Command("bash", script, filepath.Join(t.TempDir(), "nothing.out"), "80")
	missing.Dir = root
	if err := missing.Run(); err == nil {
		t.Error("a missing profile should fail the build")
	}

	empty := filepath.Join(t.TempDir(), "empty.out")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	blank := exec.Command("bash", script, empty, "80")
	blank.Dir = root
	if err := blank.Run(); err == nil {
		t.Error("an empty profile should fail the build")
	}

	// A profile from one small run covers a sliver of one package, so it
	// clears a floor of one percent and fails a floor of ninety nine. Building
	// it with the real toolchain is what makes this a test of the script
	// rather than of a hand written fixture the script may never see.
	profile := filepath.Join(t.TempDir(), "coverage.out")
	build := exec.Command("go", "test", "./tests/", "-run", "TestFeetBuildsWholeTenths",
		"-coverpkg=./internal/unit", "-coverprofile="+profile)
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building a profile: %v\n%s", err, out)
	}

	over := exec.Command("bash", script, profile, "1")
	over.Dir = root
	if out, err := over.CombinedOutput(); err != nil {
		t.Errorf("a real profile should clear a one percent floor: %v\n%s", err, out)
	}
	under := exec.Command("bash", script, profile, "99")
	under.Dir = root
	if out, err := under.CombinedOutput(); err == nil {
		t.Errorf("one test should not clear a ninety nine percent floor:\n%s", out)
	}
}

func TestTheModuleBuildsOnTheOldestToolchainCIUses(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile(filepath.Join(repoRoot(t), "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	var directive string
	for _, line := range strings.Split(string(body), "\n") {
		if strings.HasPrefix(line, "go ") {
			directive = strings.TrimSpace(strings.TrimPrefix(line, "go "))
			break
		}
	}
	if directive == "" {
		t.Fatal("go.mod has no go directive")
	}
	// The workflow builds on an older toolchain as well as the pinned one, and
	// a go directive above that toolchain makes the older job refuse the
	// module outright rather than fail a test.
	w := loadWorkflow(t)
	oldest := ""
	for _, v := range w.Jobs["test"].Strategy.Matrix["go"] {
		if oldest == "" || v < oldest {
			oldest = v
		}
	}
	if oldest == "" {
		t.Fatal("the test job has no toolchain matrix")
	}
	if !strings.HasPrefix(oldest, directive[:4]) && directive > oldest {
		t.Errorf("go.mod asks for go %s and the workflow builds on %s", directive, oldest)
	}
	if strings.HasPrefix(directive, "1.23") {
		t.Errorf("go.mod asks for %s, which the %s job cannot build", directive, oldest)
	}
}

func TestTheRaceScriptSaysWhichItDid(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile(filepath.Join(repoRoot(t), "scripts", "race.sh"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{"RACE_REQUIRED", "race: skipped", "CGO_ENABLED=1", "go test -race"} {
		if !strings.Contains(text, want) {
			t.Errorf("race.sh does not mention %q", want)
		}
	}
	w := loadWorkflow(t)
	found := false
	for _, step := range w.Jobs["ordering"].Steps {
		if strings.Contains(step.Run, "make race") {
			found = true
		}
	}
	if !found {
		t.Error("the ordering job does not run the race target")
	}
	workflowBody, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(workflowBody), "RACE_REQUIRED") {
		t.Error("the workflow does not insist the detector actually runs")
	}
}
