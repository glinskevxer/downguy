package tests

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/glinskevxer/downguy/internal/clearance"
	"github.com/glinskevxer/downguy/internal/loading"
	"github.com/glinskevxer/downguy/internal/makeready"
	"github.com/glinskevxer/downguy/internal/nesc"
	"github.com/glinskevxer/downguy/internal/report"
)

func TestFindingsJSONShape(t *testing.T) {
	fs := clearance.Findings{
		finding("NESC-235C1", nesc.SeverityStop, clearance.BlameApplicant, "MERIDIAN FIBER", 28, 33),
	}
	fs[0].Span = "SS-330"
	got := report.FindingsJSON(fs)
	if len(got) != 1 {
		t.Fatalf("findings = %v", got)
	}
	f := got[0]
	if f.Rule != "NESC-235C1" || f.Severity != "stop" || f.Blame != "applicant" {
		t.Errorf("finding = %+v", f)
	}
	if f.Measured != "2.8" || f.Required != "3.3" || f.Deficit != "0.5" {
		t.Errorf("finding = %+v", f)
	}
	if f.Span != "SS-330" {
		t.Errorf("span = %q", f.Span)
	}
	if got := report.FindingsJSON(nil); len(got) != 0 {
		t.Errorf("no findings = %v", got)
	}
}

func TestFindingsJSONOmitsEmptyFields(t *testing.T) {
	fs := clearance.Findings{finding("NESC-232A", nesc.SeverityCorrect, clearance.BlamePreExisting, "X", 148, 155)}
	fs[0].Detail = ""
	fs[0].Against = ""
	var buf bytes.Buffer
	if err := report.WriteJSON(&buf, report.FindingsJSON(fs)); err != nil {
		t.Fatal(err)
	}
	text := buf.String()
	if strings.Contains(text, `"span"`) || strings.Contains(text, `"against"`) {
		t.Errorf("empty fields should be omitted:\n%s", text)
	}
	if !strings.Contains(text, `"deficit_ft": "0.7"`) {
		t.Errorf("json = %s", text)
	}
}

func TestPlanJSONShape(t *testing.T) {
	plan := planWith(
		raise("MR-0001", "NW-441829", "RIDGEPORT CABLEVISION"),
		makeready.Item{ID: "MR-0002", Action: makeready.ActionAddGuy, Pole: "NW-441829",
			Owner: "SUMMIT VALLEY ELECTRIC", Because: []string{"NESC-261A|NW-441829|||"}},
	)
	got := report.PlanToJSON(plan)
	if got.Application != "APP-1042" || len(got.Items) != 2 {
		t.Fatalf("plan = %+v", got)
	}
	byID := map[string]report.WorkItemJSON{}
	for _, item := range got.Items {
		byID[item.ID] = item
	}
	if byID["MR-0001"].From != "20.0" || byID["MR-0001"].To != "21.2" {
		t.Errorf("item = %+v", byID["MR-0001"])
	}
	if byID["MR-0002"].From != "" {
		t.Errorf("an item that moves nothing should carry no heights, got %+v", byID["MR-0002"])
	}
	if len(byID["MR-0002"].Because) != 1 {
		t.Errorf("the citation should survive, got %+v", byID["MR-0002"])
	}
	if got.Unresolved != nil {
		t.Errorf("nothing is unresolved, got %v", got.Unresolved)
	}
	empty := report.PlanToJSON(nil)
	if empty.Items == nil || len(empty.Items) != 0 {
		t.Errorf("a nil plan should render an empty item list, got %+v", empty)
	}
}

func TestPlanJSONCarriesTheUnresolved(t *testing.T) {
	plan := planner().Plan(clearance.Findings{planFinding("NESC-250B", "", "", 0, 0)})
	got := report.PlanToJSON(plan)
	if len(got.Unresolved) != 1 {
		t.Fatalf("unresolved = %v", got.Unresolved)
	}
	if got.Unresolved[0].Rule != "NESC-250B" {
		t.Errorf("unresolved = %+v", got.Unresolved[0])
	}
}

func TestAnalysisJSONShape(t *testing.T) {
	a, err := loading.Analyse(samplePole(), sampleWires(t), nesc.Heavy, nesc.GradeC)
	if err != nil {
		t.Fatal(err)
	}
	got := report.AnalysisToJSON(a)
	if got.Pole != "NW-441829" || got.District != "heavy" || got.Grade != "C" {
		t.Errorf("analysis = %+v", got)
	}
	if len(got.Components) != len(a.Components) {
		t.Errorf("components = %v", got.Components)
	}
	if got.Passes != a.Passes() || got.Utilisation != a.UtilisationPercent() {
		t.Errorf("analysis = %+v", got)
	}
	if got.Factored != float64(int64(got.Factored)) {
		t.Errorf("moments should be whole foot pounds, got %v", got.Factored)
	}
	empty := report.AnalysisToJSON(nil)
	if empty.Components == nil || len(empty.Components) != 0 {
		t.Errorf("a nil analysis should render an empty component list, got %+v", empty)
	}
}

func TestWriteJSONIsIndentedAndParses(t *testing.T) {
	a, err := loading.Analyse(samplePole(), sampleWires(t), nesc.Heavy, nesc.GradeC)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := report.WriteJSON(&buf, report.AnalysisToJSON(a)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "\n  \"pole\"") {
		t.Errorf("json should be indented:\n%s", buf.String())
	}
	if !strings.HasSuffix(buf.String(), "\n") {
		t.Error("json should end with a newline")
	}
	var back report.AnalysisJSON
	if err := json.Unmarshal(buf.Bytes(), &back); err != nil {
		t.Fatal(err)
	}
	if back.Pole != "NW-441829" || back.Utilisation != a.UtilisationPercent() {
		t.Errorf("round trip = %+v", back)
	}
}

func TestWriteJSONRefusesWhatItCannotEncode(t *testing.T) {
	var buf bytes.Buffer
	if err := report.WriteJSON(&buf, make(chan int)); err == nil {
		t.Error("a channel should not encode")
	}
}

func TestNegativeHeightsCarryTheirSign(t *testing.T) {
	fs := clearance.Findings{finding("NESC-232A", nesc.SeverityCorrect, clearance.BlamePreExisting, "X", -12, 155)}
	got := report.FindingsJSON(fs)
	if got[0].Measured != "-1.2" {
		t.Errorf("measured = %q", got[0].Measured)
	}
}
