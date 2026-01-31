package tests

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/glinskevxer/downguy/internal/cliapp"
)

func loadedDB(t *testing.T) string {
	t.Helper()
	path := tempDB(t)
	if got := cliIn(poleCSV, "import", "-db", path, "poles"); got.Code != cliapp.CodeOK {
		t.Fatal(got.Stderr)
	}
	if got := cliIn(attachmentCSV, "import", "-db", path, "attachments"); got.Code != cliapp.CodeOK {
		t.Fatal(got.Stderr)
	}
	spans := "span_id,from_pole,to_pole,length_ft,cover\nSP-1,NW-441829,NW-441830,200,road\n"
	if got := cliIn(spans, "import", "-db", path, "spans"); got.Code != cliapp.CodeOK {
		t.Fatal(got.Stderr)
	}
	return path
}

func TestPolePrintsAnAssembly(t *testing.T) {
	t.Parallel()
	path := loadedDB(t)
	got := cli("pole", "-db", path, "NW-441829")
	if got.Code != cliapp.CodeOK {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	for _, want := range []string{"NW-441829 40/4 SPF", "SUMMIT VALLEY ELECTRIC", "safety zone", "A3", "RIDGEPORT CABLEVISION"} {
		if !strings.Contains(got.Stdout, want) {
			t.Errorf("stdout does not mention %q:\n%s", want, got.Stdout)
		}
	}
	if strings.Contains(got.Stdout, "SP-1") {
		t.Error("spans should only print when asked for")
	}
}

func TestPoleWithSpans(t *testing.T) {
	t.Parallel()
	path := loadedDB(t)
	got := cli("pole", "-db", path, "-spans", "NW-441829")
	if got.Code != cliapp.CodeOK {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	if !strings.Contains(got.Stdout, "SP-1") || !strings.Contains(got.Stdout, "NW-441830") {
		t.Errorf("stdout = %q", got.Stdout)
	}
}

func TestPoleAsJSON(t *testing.T) {
	t.Parallel()
	path := loadedDB(t)
	got := cli("pole", "-db", path, "-json", "-spans", "NW-441829")
	if got.Code != cliapp.CodeOK {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	var back cliapp.PoleJSON
	if err := json.Unmarshal([]byte(got.Stdout), &back); err != nil {
		t.Fatalf("%v\n%s", err, got.Stdout)
	}
	if back.Pole != "NW-441829" || back.Class != "4" || back.Length != 40 {
		t.Errorf("pole = %+v", back)
	}
	if len(back.Attachments) != 3 {
		t.Errorf("attachments = %v", back.Attachments)
	}
	if back.Attachments[0].ID != "A1" {
		t.Errorf("attachments should come back highest first, got %v", back.Attachments)
	}
	if back.SafetyZone == "" {
		t.Error("the safety zone should be on the record")
	}
	if len(back.Spans) != 1 || back.Spans[0].Other != "NW-441830" {
		t.Errorf("spans = %v", back.Spans)
	}
}

func TestPoleOnABarePole(t *testing.T) {
	t.Parallel()
	path := loadedDB(t)
	got := cli("pole", "-db", path, "SE-1129043")
	if got.Code != cliapp.CodeOK {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	if !strings.Contains(got.Stdout, "nothing on the pole") {
		t.Errorf("stdout = %q", got.Stdout)
	}
	if strings.Contains(got.Stdout, "safety zone") {
		t.Error("a bare pole has no safety zone to report")
	}
}

func TestPoleOnAMissingPole(t *testing.T) {
	t.Parallel()
	path := loadedDB(t)
	got := cli("pole", "-db", path, "NW-999999")
	if got.Code != cliapp.CodeFindings {
		t.Errorf("code = %d", got.Code)
	}
	if !strings.Contains(got.Stderr, "NW-999999") {
		t.Errorf("stderr = %q", got.Stderr)
	}
}

func TestPoleRejectsBadUse(t *testing.T) {
	t.Parallel()
	path := loadedDB(t)
	cases := [][]string{
		{"pole", "-db", path},
		{"pole", "-db", path, "NW-441829", "NW-441830"},
		{"pole", "-db", path, "nope"},
		{"pole", "-nonsense"},
	}
	for _, args := range cases {
		if got := cli(args...); got.Code != cliapp.CodeUsage {
			t.Errorf("%v gave code %d", args, got.Code)
		}
	}
}
