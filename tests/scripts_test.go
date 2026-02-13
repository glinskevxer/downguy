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
