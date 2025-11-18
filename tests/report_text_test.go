package tests

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glinskevxer/downguy/internal/clearance"
	"github.com/glinskevxer/downguy/internal/loading"
	"github.com/glinskevxer/downguy/internal/makeready"
	"github.com/glinskevxer/downguy/internal/nesc"
	"github.com/glinskevxer/downguy/internal/report"
)

func rendered(t *testing.T, render func(*bytes.Buffer) error) string {
	t.Helper()
	var buf bytes.Buffer
	if err := render(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestRuleAndHeading(t *testing.T) {
	got := rendered(t, func(b *bytes.Buffer) error { return report.Rule(b) })
	if len(strings.TrimRight(got, "\n")) != report.Width {
		t.Errorf("rule is %d wide", len(strings.TrimRight(got, "\n")))
	}
	got = rendered(t, func(b *bytes.Buffer) error { return report.Heading(b, "make ready") })
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 2 || lines[0] != "MAKE READY" {
		t.Errorf("heading = %q", got)
	}
}

func TestTableAlignsToTheWidestCell(t *testing.T) {
	table := report.NewTable("id", "amount").RightAlign(1)
	table.Row("a-very-long-identifier", "1.00")
	table.Row("x", "1234.00")
	if table.Len() != 2 {
		t.Errorf("len = %d", table.Len())
	}
	got := rendered(t, func(b *bytes.Buffer) error { return table.Render(b) })
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("table = %q", got)
	}
	if !strings.HasPrefix(lines[0], "id  ") {
		t.Errorf("header = %q", lines[0])
	}
	if !strings.HasSuffix(lines[2], "   1.00") {
		t.Errorf("the amount column should be right aligned, got %q", lines[2])
	}
	if !strings.Contains(lines[1], "---") {
		t.Errorf("separator = %q", lines[1])
	}
}

func TestTablePadsAShortRow(t *testing.T) {
	table := report.NewTable("a", "b", "c")
	table.Row("one")
	table.Row("one", "two", "three", "four")
	got := rendered(t, func(b *bytes.Buffer) error { return table.Render(b) })
	if strings.Contains(got, "four") {
		t.Errorf("an over wide row should be trimmed, got %q", got)
	}
	if !strings.Contains(got, "one") {
		t.Errorf("table = %q", got)
	}
}

func TestRenderFindings(t *testing.T) {
	empty := rendered(t, func(b *bytes.Buffer) error { return report.Findings(b, nil) })
	if strings.TrimSpace(empty) != "no findings" {
		t.Errorf("empty = %q", empty)
	}
	fs := clearance.Findings{
		finding("NESC-235C1", nesc.SeverityStop, clearance.BlameApplicant, "MERIDIAN FIBER", 28, 33),
		finding("NESC-232A", nesc.SeverityCorrect, clearance.BlamePreExisting, "RIDGEPORT CABLEVISION", 200, 155),
	}
	fs[1].Span = "SS-330"
	got := rendered(t, func(b *bytes.Buffer) error { return report.Findings(b, fs) })
	if !strings.Contains(got, "NESC-235C1") || !strings.Contains(got, "MERIDIAN FIBER") {
		t.Errorf("report = %q", got)
	}
	if !strings.Contains(got, "SS-330") {
		t.Errorf("a span finding should print its span, got %q", got)
	}
	if !strings.Contains(got, "0.5 ft") {
		t.Errorf("the deficit should print, got %q", got)
	}
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 4 {
		t.Errorf("report = %q", got)
	}
}

func TestRenderWork(t *testing.T) {
	empty := rendered(t, func(b *bytes.Buffer) error { return report.Work(b, nil) })
	if strings.TrimSpace(empty) != "no make ready" {
		t.Errorf("empty = %q", empty)
	}
	plan := planWith(
		raise("MR-0001", "NW-441829", "RIDGEPORT CABLEVISION"),
		makeready.Item{ID: "MR-0002", Action: makeready.ActionAddGuy, Pole: "NW-441829",
			Owner: "SUMMIT VALLEY ELECTRIC", Because: []string{"x"}, Detail: "guy the pull"},
	)
	got := rendered(t, func(b *bytes.Buffer) error { return report.Work(b, plan) })
	if !strings.Contains(got, "MR-0001") || !strings.Contains(got, "add_guy") {
		t.Errorf("report = %q", got)
	}
	if !strings.Contains(got, "20.0 ft") {
		t.Errorf("the heights should print, got %q", got)
	}
	for _, line := range strings.Split(got, "\n") {
		if strings.Contains(line, "add_guy") && strings.Contains(line, " ft") {
			t.Errorf("an item that moves nothing should print blanks, got %q", line)
		}
	}
}

func TestRenderWorkListsTheUnresolved(t *testing.T) {
	plan := planner().Plan(clearance.Findings{
		planFinding("NESC-235C1", "A3", "A2", 85, 89),
		planFinding("NESC-250B", "", "", 0, 0),
	})
	got := rendered(t, func(b *bytes.Buffer) error { return report.Work(b, plan) })
	if !strings.Contains(got, "have no remedy") {
		t.Errorf("report = %q", got)
	}
	if !strings.Contains(got, "NESC-250B") {
		t.Errorf("the unresolved finding should be named, got %q", got)
	}
}

func TestRenderAnalysis(t *testing.T) {
	empty := rendered(t, func(b *bytes.Buffer) error { return report.Analysis(b, nil) })
	if strings.TrimSpace(empty) != "no analysis" {
		t.Errorf("empty = %q", empty)
	}
	a, err := loading.Analyse(samplePole(), sampleWires(t), nesc.Heavy, nesc.GradeC)
	if err != nil {
		t.Fatal(err)
	}
	got := rendered(t, func(b *bytes.Buffer) error { return report.Analysis(b, a) })
	if !strings.Contains(got, "NW-441829") || !strings.Contains(got, "pole wind") {
		t.Errorf("report = %q", got)
	}
	if !strings.Contains(got, "capacity") || !strings.Contains(got, "total") {
		t.Errorf("the totals should print, got %q", got)
	}
}

func TestRenderOwnerTotals(t *testing.T) {
	got := rendered(t, func(b *bytes.Buffer) error {
		return report.OwnerTotals(b, "make ready by party", map[string]string{
			"RIDGEPORT CABLEVISION": "$1,204.00",
			"MERIDIAN FIBER":        "$318.50",
		})
	})
	if !strings.Contains(got, "MAKE READY BY PARTY") {
		t.Errorf("report = %q", got)
	}
	meridian := strings.Index(got, "MERIDIAN FIBER")
	ridgeport := strings.Index(got, "RIDGEPORT CABLEVISION")
	if meridian < 0 || ridgeport < 0 || meridian > ridgeport {
		t.Errorf("parties should be sorted, got %q", got)
	}
	empty := rendered(t, func(b *bytes.Buffer) error {
		return report.OwnerTotals(b, "nothing", nil)
	})
	if !strings.Contains(empty, "NOTHING") {
		t.Errorf("report = %q", empty)
	}
}
