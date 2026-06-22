package tests

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/glinskevxer/downguy/internal/cliapp"
)

func importedDB(t *testing.T) string {
	t.Helper()
	path := tempDB(t)
	got := cliIn(poleCSV, "import", "-db", path, "-by", "j.aldana", "-on", "2025-05-06", "poles")
	if got.Code != cliapp.CodeOK {
		t.Fatalf("import: %+v", got)
	}
	got = cliIn(attachmentCSV, "import", "-db", path, "-by", "d.orme", "-on", "2025-05-08", "attachments")
	if got.Code != cliapp.CodeOK {
		t.Fatalf("import: %+v", got)
	}
	return path
}

func TestImportRecordsWhoRanIt(t *testing.T) {
	t.Parallel()
	path := importedDB(t)
	got := cli("history", "-db", path, "-json", "-subject", "pole", "-id", "poles")
	if got.Code != cliapp.CodeOK {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	var back []cliapp.ChangeJSON
	if err := json.Unmarshal([]byte(got.Stdout), &back); err != nil {
		t.Fatalf("%v\n%s", err, got.Stdout)
	}
	if len(back) != 1 {
		t.Fatalf("changes = %v", back)
	}
	c := back[0]
	if c.Actor != "j.aldana" || c.Action != "imported" || c.At != "2025-05-06" {
		t.Errorf("change = %+v", c)
	}
	if !strings.Contains(c.Detail, "3 rows filed") {
		t.Errorf("detail = %q", c.Detail)
	}
	if c.Seq <= 0 {
		t.Errorf("seq = %d", c.Seq)
	}
}

func TestAnImportWithNoNameRecordsNothing(t *testing.T) {
	t.Parallel()
	path := tempDB(t)
	if got := cliIn(poleCSV, "import", "-db", path, "poles"); got.Code != cliapp.CodeOK {
		t.Fatal(got.Stderr)
	}
	got := cli("history", "-db", path, "-since", "2020-01-01")
	if got.Code != cliapp.CodeFindings {
		t.Errorf("code = %d", got.Code)
	}
	if !strings.Contains(got.Stdout, "nothing recorded") {
		t.Errorf("stdout = %q", got.Stdout)
	}
}

func TestHistoryByActorAndSince(t *testing.T) {
	t.Parallel()
	path := importedDB(t)
	byActor := cli("history", "-db", path, "-json", "-actor", "d.orme")
	if byActor.Code != cliapp.CodeOK {
		t.Fatalf("code = %d", byActor.Code)
	}
	var mine []cliapp.ChangeJSON
	if err := json.Unmarshal([]byte(byActor.Stdout), &mine); err != nil {
		t.Fatal(err)
	}
	if len(mine) != 1 || mine[0].Subject != "attachment" {
		t.Errorf("by actor = %v", mine)
	}

	since := cli("history", "-db", path, "-json", "-since", "2025-05-07")
	var later []cliapp.ChangeJSON
	if err := json.Unmarshal([]byte(since.Stdout), &later); err != nil {
		t.Fatal(err)
	}
	if len(later) != 1 || later[0].Actor != "d.orme" {
		t.Errorf("since = %v", later)
	}

	all := cli("history", "-db", path, "-since", "2020-01-01")
	if all.Code != cliapp.CodeOK {
		t.Fatalf("code = %d", all.Code)
	}
	for _, want := range []string{"j.aldana", "d.orme", "imported", "rows filed"} {
		if !strings.Contains(all.Stdout, want) {
			t.Errorf("stdout does not mention %q:\n%s", want, all.Stdout)
		}
	}
}

func TestHistoryRejectsBadUse(t *testing.T) {
	t.Parallel()
	path := importedDB(t)
	cases := [][]string{
		{"history", "-db", path},
		{"history", "-db", path, "-subject", "pole"},
		{"history", "-db", path, "-id", "poles"},
		{"history", "-db", path, "-subject", "weather", "-id", "poles"},
		{"history", "-db", path, "-actor", "j.aldana", "-since", "2020-01-01"},
		{"history", "-db", path, "-since", "not-a-date"},
		{"history", "-db", path, "-since", "2020-01-01", "extra"},
		{"history", "-nonsense"},
	}
	for _, args := range cases {
		if got := cli(args...); got.Code != cliapp.CodeUsage {
			t.Errorf("%v gave code %d (%q)", args, got.Code, got.Stderr)
		}
	}
}

func TestImportRefusesANameWithNoDate(t *testing.T) {
	t.Parallel()
	path := tempDB(t)
	got := cliIn(poleCSV, "import", "-db", path, "-by", "j.aldana", "poles")
	if got.Code != cliapp.CodeUsage {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	if !strings.Contains(got.Stderr, "-by needs -on") {
		t.Errorf("stderr = %q", got.Stderr)
	}
}
