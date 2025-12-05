package tests

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glinskevxer/downguy/internal/cliapp"
	"github.com/glinskevxer/downguy/internal/store"
)

func TestInitCreatesAndThenSaysNothingToDo(t *testing.T) {
	path := filepath.Join(t.TempDir(), "downguy.db")
	got := cli("init", "-db", path)
	if got.Code != cliapp.CodeOK {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	if !strings.Contains(got.Stdout, "migrated from schema version 0") {
		t.Errorf("stdout = %q", got.Stdout)
	}
	again := cli("init", "-db", path)
	if again.Code != cliapp.CodeOK {
		t.Fatalf("code = %d", again.Code)
	}
	if !strings.Contains(again.Stdout, "already at schema version") {
		t.Errorf("stdout = %q", again.Stdout)
	}
	if !strings.Contains(again.Stdout, itoa(store.LatestVersion())) {
		t.Errorf("stdout should name the version, got %q", again.Stdout)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}

func TestInitRejectsBadUse(t *testing.T) {
	path := filepath.Join(t.TempDir(), "downguy.db")
	got := cli("init", "-db", path, "extra")
	if got.Code != cliapp.CodeUsage {
		t.Errorf("code = %d", got.Code)
	}
	if !strings.Contains(got.Stderr, "takes no arguments") {
		t.Errorf("stderr = %q", got.Stderr)
	}
	got = cli("init", "-nonsense")
	if got.Code != cliapp.CodeUsage {
		t.Errorf("code = %d", got.Code)
	}
}

func TestInitRejectsABadConfiguration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	if err := writeFile(path, "district: tropical\n"); err != nil {
		t.Fatal(err)
	}
	got := cli("init", "-config", path)
	if got.Code != cliapp.CodeUsage {
		t.Errorf("code = %d", got.Code)
	}
	if !strings.Contains(got.Stderr, "tropical") {
		t.Errorf("stderr = %q", got.Stderr)
	}
}

func TestConfigPrintsTheEffectiveSettings(t *testing.T) {
	got := cli("config")
	if got.Code != cliapp.CodeOK {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	for _, want := range []string{"database:", "district: heavy", "grade: C", "attachers: 5"} {
		if !strings.Contains(got.Stdout, want) {
			t.Errorf("stdout does not mention %q:\n%s", want, got.Stdout)
		}
	}
}

func TestConfigTakesTheFlagOverTheFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "downguy.yaml")
	if err := writeFile(path, "database: from-the-file.db\ndistrict: light\n"); err != nil {
		t.Fatal(err)
	}
	got := cli("config", "-config", path)
	if !strings.Contains(got.Stdout, "from-the-file.db") || !strings.Contains(got.Stdout, "district: light") {
		t.Errorf("stdout = %q", got.Stdout)
	}
	got = cli("config", "-config", path, "-db", "from-the-flag.db")
	if !strings.Contains(got.Stdout, "from-the-flag.db") {
		t.Errorf("the flag should win, got %q", got.Stdout)
	}
	if !strings.Contains(got.Stdout, "district: light") {
		t.Errorf("the rest of the file should still apply, got %q", got.Stdout)
	}
}

func TestConfigRejectsBadUse(t *testing.T) {
	got := cli("config", "extra")
	if got.Code != cliapp.CodeUsage {
		t.Errorf("code = %d", got.Code)
	}
	got = cli("config", "-nonsense")
	if got.Code != cliapp.CodeUsage {
		t.Errorf("code = %d", got.Code)
	}
}

func TestFlagsUsageListsTheOptions(t *testing.T) {
	c, ok := cliapp.Lookup("init")
	if !ok {
		t.Fatal("init is not registered")
	}
	for _, want := range []string{"-config", "-db", "options:"} {
		if !strings.Contains(c.Usage, want) {
			t.Errorf("the usage does not mention %q:\n%s", want, c.Usage)
		}
	}
}
