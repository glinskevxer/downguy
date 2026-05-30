package tests

import (
	"testing"

	"github.com/glinskevxer/downguy/internal/clearance"
	"github.com/glinskevxer/downguy/internal/nesc"
	"github.com/glinskevxer/downguy/internal/unit"
)

func onSubject(rule string, sev nesc.Severity, pole, subject string, measured, required int) clearance.Finding {
	return clearance.Finding{
		RuleID: rule, Severity: sev, Pole: pole, Subject: subject,
		Owner: "RIDGEPORT CABLEVISION", Blame: clearance.BlameUnattributed,
		Measured: unit.Tenths(measured), Required: unit.Tenths(required),
	}
}

func TestGoverningRules(t *testing.T) {
	t.Parallel()
	rules := clearance.GoverningRules()
	if len(rules) != 3 {
		t.Fatalf("governing rules = %v", rules)
	}
	for i := 1; i < len(rules); i++ {
		if rules[i] < rules[i-1] {
			t.Fatalf("governing rules are not sorted: %v", rules)
		}
	}
	for _, id := range rules {
		if !clearance.Governing(id) {
			t.Errorf("%s should govern", id)
		}
		if _, err := nesc.LookupRule(id); err != nil {
			t.Errorf("%s governs and is not catalogued: %v", id, err)
		}
	}
	if !clearance.Governing("nesc-235c1") {
		t.Error("the check should not care about case")
	}
	if clearance.Governing("NESC-232A") {
		t.Error("a ground clearance finding does not force a move on its own")
	}
}

func TestSuppressCollapsesFindingsOnAFacilityAlreadyMoving(t *testing.T) {
	t.Parallel()
	fs := clearance.Findings{
		onSubject("NESC-235C1", nesc.SeverityStop, "NW-441829", "A3", 20, 33),
		onSubject("NESC-235C2", nesc.SeverityCorrect, "NW-441829", "A3", 8, 10),
		onSubject("NESC-238A", nesc.SeverityCorrect, "NW-441829", "A3", 18, 25),
	}
	kept, hidden := clearance.Suppress(fs)
	if len(kept) != 1 || kept[0].RuleID != "NESC-235C1" {
		t.Fatalf("kept = %v", kept)
	}
	if len(hidden) != 2 {
		t.Fatalf("hidden = %v", hidden)
	}
	for _, h := range hidden {
		if h.Because.RuleID != "NESC-235C1" {
			t.Errorf("hidden behind %s", h.Because.RuleID)
		}
		if h.Reason == "" {
			t.Errorf("suppression = %+v", h)
		}
	}
	rules := clearance.SuppressedRules(hidden)
	if len(rules) != 2 || rules[0] != "NESC-235C2" {
		t.Errorf("suppressed rules = %v", rules)
	}
}

func TestSuppressLeavesAnotherFacilityAlone(t *testing.T) {
	t.Parallel()
	fs := clearance.Findings{
		onSubject("NESC-235C1", nesc.SeverityStop, "NW-441829", "A3", 20, 33),
		onSubject("NESC-235C2", nesc.SeverityCorrect, "NW-441829", "A4", 8, 10),
	}
	kept, hidden := clearance.Suppress(fs)
	if len(kept) != 2 {
		t.Errorf("a finding on another facility is its own problem, got %v", kept)
	}
	if len(hidden) != 0 {
		t.Errorf("hidden = %v", hidden)
	}
}

func TestSuppressLeavesAnotherPoleAlone(t *testing.T) {
	t.Parallel()
	fs := clearance.Findings{
		onSubject("NESC-235C1", nesc.SeverityStop, "NW-441829", "A3", 20, 33),
		onSubject("NESC-235C2", nesc.SeverityCorrect, "NW-441830", "A3", 8, 10),
	}
	kept, hidden := clearance.Suppress(fs)
	if len(kept) != 2 || len(hidden) != 0 {
		t.Errorf("kept = %v hidden = %v", kept, hidden)
	}
}

func TestSuppressNeverHidesTheMoreSevere(t *testing.T) {
	t.Parallel()
	// A climbing space finding governs at severity stop; a second stop finding
	// on the same facility is not softer than it, so both stay.
	fs := clearance.Findings{
		onSubject("NESC-236A", nesc.SeverityStop, "NW-441829", "A6", 0, 25),
		onSubject("NESC-235C1", nesc.SeverityStop, "NW-441829", "A6", 20, 33),
	}
	kept, hidden := clearance.Suppress(fs)
	if len(kept) != 2 || len(hidden) != 0 {
		t.Errorf("two governing findings both stand: kept %v hidden %v", kept, hidden)
	}

	// And a stop finding is never hidden behind a correct one.
	mixed := clearance.Findings{
		onSubject("NESC-250B", nesc.SeverityStop, "NW-441829", "A3", 0, 0),
	}
	mixed[0].Severity = nesc.SeverityCorrect
	mixed = append(mixed, onSubject("NESC-235C2", nesc.SeverityStop, "NW-441829", "A3", 8, 10))
	kept, hidden = clearance.Suppress(mixed)
	if len(kept) != 2 || len(hidden) != 0 {
		t.Errorf("kept = %v hidden = %v", kept, hidden)
	}
}

func TestSuppressKeepsTheWorstGoverningFinding(t *testing.T) {
	t.Parallel()
	fs := clearance.Findings{
		onSubject("NESC-250B", nesc.SeverityCorrect, "NW-441829", "A3", 0, 0),
		onSubject("NESC-236A", nesc.SeverityStop, "NW-441829", "A3", 0, 25),
		onSubject("NESC-238A", nesc.SeverityCorrect, "NW-441829", "A3", 18, 25),
	}
	kept, hidden := clearance.Suppress(fs)
	if len(kept) != 2 {
		t.Fatalf("both governing findings stand, got %v", kept)
	}
	if len(hidden) != 1 || hidden[0].Because.RuleID != "NESC-236A" {
		t.Errorf("the softer one should hide behind the worst governing finding, got %v", hidden)
	}
}

func TestSuppressOfNothing(t *testing.T) {
	t.Parallel()
	kept, hidden := clearance.Suppress(nil)
	if kept != nil || hidden != nil {
		t.Errorf("kept = %v hidden = %v", kept, hidden)
	}
	if got := clearance.SuppressedRules(nil); got != nil {
		t.Errorf("suppressed rules = %v", got)
	}
}

func TestSuppressLeavesAnUngovernedSetAlone(t *testing.T) {
	t.Parallel()
	fs := clearance.Findings{
		onSubject("NESC-232A", nesc.SeverityCorrect, "NW-441829", "A3", 148, 155),
		onSubject("NESC-235C2", nesc.SeverityCorrect, "NW-441829", "A3", 8, 10),
	}
	kept, hidden := clearance.Suppress(fs)
	if len(kept) != 2 || len(hidden) != 0 {
		t.Errorf("nothing here forces a move, so nothing hides: kept %v hidden %v", kept, hidden)
	}
}
