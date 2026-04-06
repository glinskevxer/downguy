package tests

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/glinskevxer/downguy/internal/cliapp"
)

const agedPoleCSV = `pole_id,length_ft,class,species,owner,condition,installed_in
NW-441829,40,4,SPF,SUMMIT VALLEY ELECTRIC,sound,1968
NW-441830,45,3,douglas fir,SUMMIT VALLEY ELECTRIC,serviceable,1998
NW-441831,40,4,SPF,SUMMIT VALLEY ELECTRIC,sound,2020
SE-1129043,40,4,SPF,CASCADE POWER,sound,1985
`

func inspectDB(t *testing.T) string {
	t.Helper()
	path := tempDB(t)
	if got := cliIn(agedPoleCSV, "import", "-db", path, "poles"); got.Code != cliapp.CodeOK {
		t.Fatal(got.Stderr)
	}
	return path
}

func TestInspectRanksTheOptimisticFirst(t *testing.T) {
	t.Parallel()
	path := inspectDB(t)
	got := cli("inspect", "-db", path, "-json", "-year", "2026")
	if got.Code != cliapp.CodeFindings {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	var back []cliapp.InspectRowJSON
	if err := json.Unmarshal([]byte(got.Stdout), &back); err != nil {
		t.Fatalf("%v\n%s", err, got.Stdout)
	}
	if len(back) != 4 {
		t.Fatalf("rows = %v", back)
	}
	if !back[0].Optimistic {
		t.Errorf("the optimistically graded poles come first, got %+v", back[0])
	}
	if back[0].Pole != "NW-441829" {
		t.Errorf("the 1968 pole graded sound should lead, got %s", back[0].Pole)
	}
	if back[0].Stage != "overdue" || back[0].Age != 58 {
		t.Errorf("row = %+v", back[0])
	}
	if back[len(back)-1].Pole != "NW-441831" {
		t.Errorf("the 2020 pole should sort last, got %s", back[len(back)-1].Pole)
	}
	for _, r := range back {
		if !r.Due {
			t.Errorf("nothing has an inspection on record, so everything is due: %+v", r)
		}
		if r.Interval <= 0 || r.Expected == "" {
			t.Errorf("row = %+v", r)
		}
	}
}

func TestInspectOptimisticOnly(t *testing.T) {
	t.Parallel()
	path := inspectDB(t)
	got := cli("inspect", "-db", path, "-json", "-year", "2026", "-optimistic-only")
	if got.Code != cliapp.CodeFindings {
		t.Fatalf("code = %d", got.Code)
	}
	var back []cliapp.InspectRowJSON
	if err := json.Unmarshal([]byte(got.Stdout), &back); err != nil {
		t.Fatal(err)
	}
	if len(back) == 0 {
		t.Fatal("the 1968 pole graded sound should be on this list")
	}
	for _, r := range back {
		if !r.Optimistic {
			t.Errorf("row = %+v", r)
		}
	}
}

func TestInspectFiltersAndLimits(t *testing.T) {
	t.Parallel()
	path := inspectDB(t)
	got := cli("inspect", "-db", path, "-json", "-year", "2026", "-district", "SE")
	var district []cliapp.InspectRowJSON
	if err := json.Unmarshal([]byte(got.Stdout), &district); err != nil {
		t.Fatal(err)
	}
	if len(district) != 1 || district[0].Pole != "SE-1129043" {
		t.Errorf("district = %v", district)
	}

	limited := cli("inspect", "-db", path, "-json", "-year", "2026", "-limit", "2")
	var page []cliapp.InspectRowJSON
	if err := json.Unmarshal([]byte(limited.Stdout), &page); err != nil {
		t.Fatal(err)
	}
	if len(page) != 2 {
		t.Errorf("limit = %v", page)
	}
}

func TestInspectPrintsATable(t *testing.T) {
	t.Parallel()
	path := inspectDB(t)
	got := cli("inspect", "-db", path, "-year", "2026")
	if got.Code != cliapp.CodeFindings {
		t.Fatalf("code = %d", got.Code)
	}
	for _, want := range []string{"NW-441829", "overdue", "graded better than its age", "SPF"} {
		if !strings.Contains(got.Stdout, want) {
			t.Errorf("stdout does not mention %q:\n%s", want, got.Stdout)
		}
	}
}

func TestInspectOnAnEmptyDatabase(t *testing.T) {
	t.Parallel()
	path := tempDB(t)
	got := cli("inspect", "-db", path, "-year", "2026")
	if got.Code != cliapp.CodeOK {
		t.Errorf("code = %d", got.Code)
	}
	if !strings.Contains(got.Stdout, "nothing to inspect") {
		t.Errorf("stdout = %q", got.Stdout)
	}
}

func TestInspectRejectsBadUse(t *testing.T) {
	t.Parallel()
	path := inspectDB(t)
	cases := [][]string{
		{"inspect", "-db", path},
		{"inspect", "-db", path, "-year", "1800"},
		{"inspect", "-db", path, "-year", "2026", "-limit", "-1"},
		{"inspect", "-db", path, "-year", "2026", "extra"},
		{"inspect", "-nonsense"},
	}
	for _, args := range cases {
		if got := cli(args...); got.Code != cliapp.CodeUsage {
			t.Errorf("%v gave code %d (%q)", args, got.Code, got.Stderr)
		}
	}
}
