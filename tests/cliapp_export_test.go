package tests

import (
	"strings"
	"testing"

	"github.com/glinskevxer/downguy/internal/cliapp"
)

func TestExportRoundTripsThroughTheImporter(t *testing.T) {
	t.Parallel()
	path := loadedDB(t)
	got := cli("export", "-db", path, "attachments")
	if got.Code != cliapp.CodeOK {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	lines := strings.Split(strings.TrimSpace(got.Stdout), "\n")
	if len(lines) != 4 {
		t.Fatalf("one header and three rows:\n%s", got.Stdout)
	}
	if !strings.HasPrefix(lines[0], "attachment_id,pole_id,owner") {
		t.Errorf("header = %q", lines[0])
	}

	// Feed it straight back into a fresh database.
	fresh := tempDB(t)
	if step := cliIn(poleCSV, "import", "-db", fresh, "poles"); step.Code != cliapp.CodeOK {
		t.Fatal(step.Stderr)
	}
	back := cliIn(got.Stdout, "import", "-db", fresh, "attachments")
	if back.Code != cliapp.CodeOK {
		t.Fatalf("the export did not import: %+v", back)
	}
	if !strings.Contains(back.Stdout, "filed 3 attachments") {
		t.Errorf("stdout = %q", back.Stdout)
	}
	again := cli("export", "-db", fresh, "attachments")
	if again.Stdout != got.Stdout {
		t.Errorf("the round trip is not stable:\n%s\nagainst\n%s", again.Stdout, got.Stdout)
	}
}

func TestExportFilters(t *testing.T) {
	t.Parallel()
	path := loadedDB(t)
	byPole := cli("export", "-db", path, "-pole", "NW-441829", "attachments")
	if byPole.Code != cliapp.CodeOK {
		t.Fatalf("code = %d, stderr = %q", byPole.Code, byPole.Stderr)
	}
	if strings.Count(byPole.Stdout, "\n") != 4 {
		t.Errorf("stdout = %q", byPole.Stdout)
	}

	byOwner := cli("export", "-db", path, "-owner", "RIDGEPORT CABLEVISION", "attachments")
	if byOwner.Code != cliapp.CodeOK {
		t.Fatalf("code = %d", byOwner.Code)
	}
	if strings.Count(byOwner.Stdout, "RIDGEPORT CABLEVISION") != 1 {
		t.Errorf("stdout = %q", byOwner.Stdout)
	}

	unauthorized := cli("export", "-db", path, "-unauthorized", "attachments")
	if unauthorized.Code != cliapp.CodeOK {
		t.Fatalf("code = %d, stderr = %q", unauthorized.Code, unauthorized.Stderr)
	}
	if strings.Count(strings.TrimSpace(unauthorized.Stdout), "\n") != 1 {
		t.Errorf("one header and one unauthorized row:\n%s", unauthorized.Stdout)
	}
	if !strings.Contains(unauthorized.Stdout, "UNIDENTIFIED") {
		t.Errorf("stdout = %q", unauthorized.Stdout)
	}
	if strings.Contains(unauthorized.Stdout, "RIDGEPORT") {
		t.Errorf("an authorized attachment should not be in the unauthorized export:\n%s", unauthorized.Stdout)
	}
}

func TestExportRejectsBadUse(t *testing.T) {
	t.Parallel()
	path := loadedDB(t)
	cases := [][]string{
		{"export", "-db", path},
		{"export", "-db", path, "widgets"},
		{"export", "-db", path, "-pole", "nope", "attachments"},
		{"export", "-db", path, "-pole", "NW-441829", "-owner", "X", "attachments"},
		{"export", "-nonsense"},
	}
	for _, args := range cases {
		if got := cli(args...); got.Code != cliapp.CodeUsage {
			t.Errorf("%v gave code %d (%q)", args, got.Code, got.Stderr)
		}
	}
}

func TestExportOfAnEmptyDatabase(t *testing.T) {
	t.Parallel()
	path := tempDB(t)
	got := cli("export", "-db", path, "attachments")
	if got.Code != cliapp.CodeFindings {
		t.Errorf("code = %d", got.Code)
	}
	if got.Stdout != "" {
		t.Errorf("stdout = %q", got.Stdout)
	}
}
