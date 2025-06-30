package tests

import (
	"testing"

	"github.com/glinskevxer/downguy/internal/clearance"
	"github.com/glinskevxer/downguy/internal/nesc"
	"github.com/glinskevxer/downguy/internal/unit"
)

func finding(rule string, sev nesc.Severity, blame clearance.Blame, owner string, measured, required int) clearance.Finding {
	return clearance.Finding{
		RuleID: rule, Severity: sev, Pole: "NW-441829", Subject: "A3",
		Owner: owner, Blame: blame,
		Measured: unit.Tenths(measured), Required: unit.Tenths(required),
		Detail: "detail",
	}
}

func TestBlameSet(t *testing.T) {
	for _, b := range []clearance.Blame{clearance.BlamePreExisting, clearance.BlameApplicant, clearance.BlameAggravated, clearance.BlameUnattributed} {
		if !clearance.KnownBlame(b) {
			t.Errorf("%s should be a known attribution", b)
		}
	}
	if clearance.KnownBlame(clearance.Blame("the weather")) {
		t.Error("the weather is not an attribution")
	}
}

func TestFindingDeficitAndRule(t *testing.T) {
	f := finding("NESC-235C1", nesc.SeverityStop, clearance.BlameApplicant, "MERIDIAN FIBER", 28, 33)
	if f.Deficit().Tenths() != 5 {
		t.Errorf("deficit = %d", f.Deficit().Tenths())
	}
	clear := finding("NESC-235C1", nesc.SeverityStop, clearance.BlameApplicant, "MERIDIAN FIBER", 40, 33)
	if clear.Deficit().Tenths() != 0 {
		t.Errorf("a clearing measurement has no deficit, got %d", clear.Deficit().Tenths())
	}
	r, err := f.Rule()
	if err != nil || r.Section != "235" {
		t.Errorf("rule = %+v %v", r, err)
	}
	bad := f
	bad.RuleID = "NESC-999"
	if _, err := bad.Rule(); err == nil {
		t.Error("an uncatalogued rule should be rejected")
	}
}

func TestFindingKeyIsStable(t *testing.T) {
	a := finding("NESC-235C1", nesc.SeverityStop, clearance.BlameApplicant, "MERIDIAN FIBER", 28, 33)
	b := a
	b.Measured = unit.Tenths(20)
	b.Detail = "something else"
	if a.Key() != b.Key() {
		t.Error("the same condition should key the same however deep it is")
	}
	c := a
	c.Subject = "A4"
	if a.Key() == c.Key() {
		t.Error("a different subject is a different condition")
	}
	if a.String() == "" {
		t.Error("String should say something")
	}
	span := a
	span.Span = "SP-1"
	if span.String() == a.String() {
		t.Error("a span finding should name the span")
	}
}

func TestFindingsSortWorstFirst(t *testing.T) {
	fs := clearance.Findings{
		finding("NESC-232A", nesc.SeverityCorrect, clearance.BlamePreExisting, "A", 28, 33),
		finding("NESC-235C1", nesc.SeverityStop, clearance.BlamePreExisting, "B", 30, 33),
		finding("NESC-236A", nesc.SeverityStop, clearance.BlameApplicant, "C", 30, 33),
		finding("NESC-239E", nesc.SeverityRecord, clearance.BlameApplicant, "D", 20, 33),
	}
	fs.Sort()
	if fs[0].RuleID != "NESC-236A" {
		t.Errorf("first = %s, want the applicant's stop finding", fs[0].RuleID)
	}
	if fs[1].RuleID != "NESC-235C1" {
		t.Errorf("second = %s", fs[1].RuleID)
	}
	if fs[2].RuleID != "NESC-232A" {
		t.Errorf("third = %s, want the correct finding", fs[2].RuleID)
	}
	if fs[3].RuleID != "NESC-239E" {
		t.Errorf("last = %s, want the record only finding", fs[3].RuleID)
	}
}

func TestFindingsSortBreaksTiesOnDeficit(t *testing.T) {
	fs := clearance.Findings{
		finding("NESC-232A", nesc.SeverityCorrect, clearance.BlameApplicant, "A", 30, 33),
		finding("NESC-234C", nesc.SeverityCorrect, clearance.BlameApplicant, "B", 20, 33),
	}
	fs.Sort()
	if fs[0].RuleID != "NESC-234C" {
		t.Errorf("the deeper deficit should come first, got %s", fs[0].RuleID)
	}
}

func TestFindingsFilters(t *testing.T) {
	fs := clearance.Findings{
		finding("NESC-232A", nesc.SeverityCorrect, clearance.BlamePreExisting, "RIDGEPORT CABLEVISION", 28, 33),
		finding("NESC-235C1", nesc.SeverityStop, clearance.BlameApplicant, "MERIDIAN FIBER", 30, 33),
		finding("NESC-236A", nesc.SeverityStop, clearance.BlameAggravated, "meridian fiber", 30, 33),
	}
	if got := len(fs.OfSeverity(nesc.SeverityStop)); got != 2 {
		t.Errorf("stop findings = %d", got)
	}
	if got := len(fs.OfSeverity(nesc.SeverityAdvisory)); got != 0 {
		t.Errorf("advisory findings = %d", got)
	}
	if got := len(fs.OfBlame(clearance.BlameApplicant)); got != 1 {
		t.Errorf("applicant findings = %d", got)
	}
	if got := len(fs.OfOwner("  Meridian Fiber ")); got != 2 {
		t.Errorf("owner findings = %d", got)
	}
	if got := len(fs.OfOwner("nobody")); got != 0 {
		t.Errorf("unknown owner findings = %d", got)
	}
	if !fs.Blocking() {
		t.Error("a stop finding blocks")
	}
	if fs.OfSeverity(nesc.SeverityCorrect).Blocking() {
		t.Error("a correct finding does not block")
	}
}

func TestFindingsOwnersAndRules(t *testing.T) {
	fs := clearance.Findings{
		finding("NESC-235C1", nesc.SeverityStop, clearance.BlameApplicant, "MERIDIAN FIBER", 30, 33),
		finding("NESC-232A", nesc.SeverityCorrect, clearance.BlamePreExisting, "RIDGEPORT CABLEVISION", 28, 33),
		finding("NESC-235C1", nesc.SeverityStop, clearance.BlameApplicant, "meridian fiber", 30, 33),
		finding("NESC-236A", nesc.SeverityStop, clearance.BlameApplicant, "  ", 30, 33),
	}
	owners := fs.Owners()
	if len(owners) != 2 || owners[0] != "MERIDIAN FIBER" || owners[1] != "RIDGEPORT CABLEVISION" {
		t.Errorf("owners = %v", owners)
	}
	rules := fs.Rules()
	if len(rules) != 3 || rules[0] != "NESC-232A" {
		t.Errorf("rules = %v", rules)
	}
	if got := clearance.Findings(nil).Owners(); got != nil {
		t.Errorf("no findings, no owners, got %v", got)
	}
}
