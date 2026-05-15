package tests

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glinskevxer/downguy/internal/clearance"
	"github.com/glinskevxer/downguy/internal/loading"
	"github.com/glinskevxer/downguy/internal/makeready"
	"github.com/glinskevxer/downguy/internal/nesc"
	"github.com/glinskevxer/downguy/internal/report"
	"github.com/glinskevxer/downguy/internal/unit"
)

// The printed reports are the part of this system a person reads, and nothing
// else in the suite would notice a column moving, a heading changing or a
// number gaining a decimal place. These compare the rendering against a file
// on disk. Run the suite with -update-golden to rewrite them after a change
// that is meant.

var updateGolden = flag.Bool("update-golden", false, "rewrite the golden files from what the code now produces")

func goldenPath(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join("testdata", name+".golden")
}

func checkGolden(t *testing.T, name, got string) {
	t.Helper()
	path := goldenPath(t, name)
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", path)
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v\nrun the suite with -update-golden to write it", err)
	}
	if string(want) != got {
		t.Errorf("%s has drifted.\n--- want ---\n%s\n--- got ---\n%s", path, want, got)
	}
}

func goldenFindings() clearance.Findings {
	fs := clearance.Findings{
		{
			RuleID: "NESC-235C1", Severity: nesc.SeverityStop,
			Pole: "NW-441829", Subject: "A3", Against: "A2",
			Owner: "RIDGEPORT CABLEVISION", Blame: clearance.BlameApplicant,
			Measured: unit.Tenths(20), Required: unit.Tenths(33),
			Detail: "A3 under A2: 2.0 ft measured against 3.3 ft required, short by 1.3 ft",
		},
		{
			RuleID: "NESC-232A", Severity: nesc.SeverityCorrect,
			Pole: "NW-441829", Span: "SS-330", Subject: "A3", Against: "road",
			Owner: "RIDGEPORT CABLEVISION", Blame: clearance.BlamePreExisting,
			Measured: unit.Tenths(148), Required: unit.Tenths(155),
			Detail: "A3 at 100 feet over public street or road: 14.8 ft measured against 15.5 ft required, short by 0.7 ft",
		},
	}
	fs.Sort()
	return fs
}

func TestGoldenFindingsReport(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := report.Findings(&buf, goldenFindings()); err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "findings", buf.String())
}

func TestGoldenNoFindings(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := report.Findings(&buf, nil); err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "findings_empty", buf.String())
}

func goldenPlan() *makeready.Plan {
	return &makeready.Plan{
		Application: "APP-1042",
		Items: makeready.Order([]makeready.Item{
			{
				ID: "MR-0001", Action: makeready.ActionReplacePole, Pole: "NW-441829",
				Owner: "SUMMIT VALLEY ELECTRIC", Because: []string{"NESC-250B|NW-441829|||"},
				Detail: "45/3 SPF",
			},
			{
				ID: "MR-0002", Action: makeready.ActionTransfer, Pole: "NW-441829",
				Subject: "A3", Owner: "RIDGEPORT CABLEVISION",
				FromHeight: unit.Tenths(200), ToHeight: unit.Tenths(200),
				Because: []string{"NESC-250B|NW-441829|||"}, Detail: "transfer to the new 45/3 SPF",
			},
			{
				ID: "MR-0003", Action: makeready.ActionRaise, Pole: "NW-441830",
				Subject: "A4", Owner: "NORTHVALE TELEPHONE",
				FromHeight: unit.Tenths(186), ToHeight: unit.Tenths(196),
				Because: []string{"NESC-235C2|NW-441830||A4|A3"}, Detail: "lift A4 by 1.0 ft",
			},
		}),
	}
}

func TestGoldenWorkReport(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := report.Work(&buf, goldenPlan()); err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "work", buf.String())
}

func TestGoldenAnalysisReport(t *testing.T) {
	t.Parallel()
	a, err := loading.Analyse(samplePole(), sampleWires(t), nesc.Heavy, nesc.GradeC)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := report.Analysis(&buf, a); err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "analysis", buf.String())
}

func TestGoldenOwnerTotals(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	err := report.OwnerTotals(&buf, "make ready by party", map[string]string{
		"MERIDIAN FIBER":         "$318.50",
		"RIDGEPORT CABLEVISION":  "$1,204.00",
		"SUMMIT VALLEY ELECTRIC": "$0.00",
	})
	if err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "owner_totals", buf.String())
}

func TestGoldenCommandHelp(t *testing.T) {
	t.Parallel()
	got := cli("help")
	if got.Code != 0 {
		t.Fatalf("code = %d", got.Code)
	}
	checkGolden(t, "help", got.Stdout)
}

func TestGoldenSagRun(t *testing.T) {
	t.Parallel()
	got := cli("sag", "-span", "225", "2 ACSR SPARROW")
	if got.Code != 0 {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	checkGolden(t, "sag", got.Stdout)
}

func TestGoldenRateSchedule(t *testing.T) {
	t.Parallel()
	got := cli("rate", "-explain", "2025")
	if got.Code != 0 {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	checkGolden(t, "rate", got.Stdout)
}

func TestEveryGoldenFileIsUsed(t *testing.T) {
	t.Parallel()
	entries, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile("golden_test.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, entry := range entries {
		name := strings.TrimSuffix(entry.Name(), ".golden")
		if name == entry.Name() {
			continue
		}
		if !strings.Contains(text, `"`+name+`"`) {
			t.Errorf("testdata/%s is not compared against anything", entry.Name())
		}
	}
}
