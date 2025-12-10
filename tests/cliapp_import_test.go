package tests

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glinskevxer/downguy/internal/cliapp"
)

func tempDB(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "downguy.db")
	if got := cli("init", "-db", path); got.Code != cliapp.CodeOK {
		t.Fatalf("init failed: %+v", got)
	}
	return path
}

func TestImportPolesFromStdin(t *testing.T) {
	path := tempDB(t)
	got := cliIn(poleCSV, "import", "-db", path, "poles")
	if got.Code != cliapp.CodeOK {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	if !strings.Contains(got.Stdout, "filed 3 poles") {
		t.Errorf("stdout = %q", got.Stdout)
	}
}

func TestImportFromAFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "downguy.db")
	if got := cli("init", "-db", path); got.Code != cliapp.CodeOK {
		t.Fatal(got.Stderr)
	}
	file := filepath.Join(dir, "poles.csv")
	if err := writeFile(file, poleCSV); err != nil {
		t.Fatal(err)
	}
	got := cli("import", "-db", path, "poles", file)
	if got.Code != cliapp.CodeOK {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	if !strings.Contains(got.Stdout, "filed 3 poles") {
		t.Errorf("stdout = %q", got.Stdout)
	}
	missing := cli("import", "-db", path, "poles", filepath.Join(dir, "nothing.csv"))
	if missing.Code != cliapp.CodeUsage {
		t.Errorf("code = %d", missing.Code)
	}
}

func TestImportTheWholeChain(t *testing.T) {
	path := tempDB(t)
	if got := cliIn(poleCSV, "import", "-db", path, "poles"); got.Code != cliapp.CodeOK {
		t.Fatal(got.Stderr)
	}
	if got := cliIn(attachmentCSV, "import", "-db", path, "attachments"); got.Code != cliapp.CodeOK {
		t.Fatalf("attachments: %+v", got)
	}
	got := cliIn(spanCSV, "import", "-db", path, "spans")
	if got.Code != cliapp.CodeFindings {
		t.Fatalf("the span fixture has one bad row, so this should report findings: %+v", got)
	}
	if !strings.Contains(got.Stdout, "filed 1 spans") {
		t.Errorf("stdout = %q", got.Stdout)
	}
	surveys := cliIn(surveyCSV, "import", "-db", path, "surveys")
	if surveys.Code != cliapp.CodeOK {
		t.Fatalf("surveys: %+v", surveys)
	}
	if !strings.Contains(surveys.Stdout, "filed 2 surveys") {
		t.Errorf("stdout = %q", surveys.Stdout)
	}
}

func TestImportReportsBadRows(t *testing.T) {
	path := tempDB(t)
	body := poleCSV + "nope,40,4,SPF,SUMMIT VALLEY ELECTRIC,sound,1998,,\n"
	got := cliIn(body, "import", "-db", path, "poles")
	if got.Code != cliapp.CodeFindings {
		t.Fatalf("code = %d", got.Code)
	}
	if !strings.Contains(got.Stdout, "1 rows rejected") {
		t.Errorf("stdout = %q", got.Stdout)
	}
	if !strings.Contains(got.Stderr, "pole_id") {
		t.Errorf("stderr = %q", got.Stderr)
	}
}

func TestImportTruncatesALongProblemList(t *testing.T) {
	path := tempDB(t)
	var b strings.Builder
	b.WriteString("pole_id,length_ft,class,species,owner\n")
	for i := 0; i < 8; i++ {
		b.WriteString("nope,40,4,SPF,SUMMIT VALLEY ELECTRIC\n")
	}
	// Options come before the kind, which is where the flag package stops
	// looking for them.
	got := cliIn(b.String(), "import", "-db", path, "-max-problems", "3", "poles")
	if got.Code != cliapp.CodeFindings {
		t.Fatalf("code = %d", got.Code)
	}
	if strings.Count(got.Stderr, "pole_id") != 3 {
		t.Errorf("stderr = %q", got.Stderr)
	}
	if !strings.Contains(got.Stderr, "and 5 more") {
		t.Errorf("stderr = %q", got.Stderr)
	}
}

func TestImportDryRunWritesNothing(t *testing.T) {
	path := tempDB(t)
	got := cliIn(poleCSV, "import", "-db", path, "-dry-run", "poles")
	if got.Code != cliapp.CodeOK {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	if !strings.Contains(got.Stdout, "would file 3 poles") {
		t.Errorf("stdout = %q", got.Stdout)
	}
	// A second dry run against a database that was never written still reads
	// the same, which is the point.
	again := cliIn(attachmentCSV, "import", "-db", path, "-dry-run", "attachments")
	if again.Code != cliapp.CodeOK {
		t.Errorf("attachments with no poles loaded should still parse: %+v", again)
	}
}

func TestImportRejectsBadUse(t *testing.T) {
	path := tempDB(t)
	cases := [][]string{
		{"import", "-db", path},
		{"import", "-db", path, "poles", "a", "b"},
		{"import", "-db", path, "widgets"},
		{"import", "-nonsense"},
	}
	for _, args := range cases {
		got := cli(args...)
		if got.Code != cliapp.CodeUsage {
			t.Errorf("%v gave code %d", args, got.Code)
		}
	}
}

func TestImportRejectsAFileMissingItsColumns(t *testing.T) {
	path := tempDB(t)
	got := cliIn("pole_id,owner\nNW-441829,X\n", "import", "-db", path, "poles")
	if got.Code != cliapp.CodeUsage {
		t.Errorf("code = %d", got.Code)
	}
	if !strings.Contains(got.Stderr, "class") {
		t.Errorf("stderr = %q", got.Stderr)
	}
}
