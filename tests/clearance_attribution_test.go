package tests

import (
	"testing"

	"github.com/glinskevxer/downguy/internal/clearance"
	"github.com/glinskevxer/downguy/internal/nesc"
	"github.com/glinskevxer/downguy/internal/unit"
)

func f(rule, subject string, measured, required int) clearance.Finding {
	return clearance.Finding{
		RuleID: rule, Severity: nesc.SeverityCorrect,
		Pole: "NW-441829", Subject: subject,
		Owner: "RIDGEPORT CABLEVISION", Blame: clearance.BlamePreExisting,
		Measured: unit.Tenths(measured), Required: unit.Tenths(required),
	}
}

func TestAttributeNamesTheApplicantForANewCondition(t *testing.T) {
	before := clearance.Findings{}
	after := clearance.Findings{f("NESC-232A", "A7", 140, 155)}
	got := clearance.Attribute(before, after)
	if len(got) != 1 || got[0].Blame != clearance.BlameApplicant {
		t.Errorf("got = %v", got)
	}
}

func TestAttributeLeavesAPreExistingConditionAlone(t *testing.T) {
	before := clearance.Findings{f("NESC-232A", "A3", 140, 155)}
	after := clearance.Findings{f("NESC-232A", "A3", 140, 155)}
	got := clearance.Attribute(before, after)
	if len(got) != 1 || got[0].Blame != clearance.BlamePreExisting {
		t.Errorf("got = %v", got)
	}
}

func TestAttributeCallsADeeperDeficitAggravated(t *testing.T) {
	before := clearance.Findings{f("NESC-232A", "A3", 150, 155)}
	after := clearance.Findings{f("NESC-232A", "A3", 140, 155)}
	got := clearance.Attribute(before, after)
	if len(got) != 1 || got[0].Blame != clearance.BlameAggravated {
		t.Errorf("got = %v", got)
	}
}

func TestAShallowerDeficitIsStillPreExisting(t *testing.T) {
	before := clearance.Findings{f("NESC-232A", "A3", 140, 155)}
	after := clearance.Findings{f("NESC-232A", "A3", 150, 155)}
	got := clearance.Attribute(before, after)
	if len(got) != 1 || got[0].Blame != clearance.BlamePreExisting {
		t.Errorf("an improvement is not the applicant's problem, got %v", got)
	}
}

func TestAttributeDoesNotDisturbTheInput(t *testing.T) {
	after := clearance.Findings{f("NESC-232A", "A7", 140, 155)}
	clearance.Attribute(nil, after)
	if after[0].Blame != clearance.BlamePreExisting {
		t.Error("Attribute should work on a copy")
	}
}

func TestResolvedFindsWhatWentAway(t *testing.T) {
	before := clearance.Findings{
		f("NESC-232A", "A3", 140, 155),
		f("NESC-235C2", "A4", 8, 10),
	}
	after := clearance.Findings{f("NESC-232A", "A3", 140, 155)}
	got := clearance.Resolved(before, after)
	if len(got) != 1 || got[0].RuleID != "NESC-235C2" {
		t.Errorf("resolved = %v", got)
	}
	if len(clearance.Resolved(before, before)) != 0 {
		t.Error("nothing was resolved")
	}
}

func TestClassifyBucketsEverything(t *testing.T) {
	before := clearance.Findings{
		f("NESC-232A", "A3", 150, 155),
		f("NESC-235C2", "A4", 8, 10),
	}
	after := clearance.Findings{
		f("NESC-232A", "A3", 140, 155),
		f("NESC-232A", "A7", 130, 155),
		f("NESC-236A", "A6", 0, 25),
	}
	got := clearance.Classify(before, after)
	counts := got.Counts()
	if counts[clearance.BlameApplicant] != 2 {
		t.Errorf("applicant = %v", got.Applicant)
	}
	if counts[clearance.BlameAggravated] != 1 {
		t.Errorf("aggravated = %v", got.Aggravated)
	}
	if counts[clearance.BlamePreExisting] != 0 {
		t.Errorf("pre existing = %v", got.PreExisting)
	}
	if len(got.Resolved) != 1 {
		t.Errorf("resolved = %v", got.Resolved)
	}
	if len(got.All()) != 3 {
		t.Errorf("all = %v", got.All())
	}
	if len(got.ApplicantsShare()) != 3 {
		t.Errorf("the applicant owns what it caused and what it worsened, got %v", got.ApplicantsShare())
	}
}

func TestClassifyOnACleanApplication(t *testing.T) {
	got := clearance.Classify(nil, nil)
	if len(got.All()) != 0 || len(got.ApplicantsShare()) != 0 || len(got.Resolved) != 0 {
		t.Errorf("a clean application should be empty, got %+v", got)
	}
	for _, n := range got.Counts() {
		if n != 0 {
			t.Errorf("counts = %v", got.Counts())
		}
	}
}

func TestAggravatedByMeasuresHowMuchWorse(t *testing.T) {
	before := clearance.Findings{
		f("NESC-232A", "A3", 150, 155),
		f("NESC-235C2", "A4", 9, 10),
	}
	after := clearance.Findings{
		f("NESC-232A", "A3", 140, 155),
		f("NESC-235C2", "A4", 9, 10),
		f("NESC-236A", "A7", 0, 25),
	}
	got := clearance.AggravatedBy(before, after)
	if len(got) != 1 {
		t.Fatalf("aggravations = %v", got)
	}
	d := got[0]
	if d.RuleID != "NESC-232A" || d.Subject != "A3" {
		t.Errorf("delta = %+v", d)
	}
	if d.Was.Tenths() != 5 || d.Now.Tenths() != 15 || d.Worse.Tenths() != 10 {
		t.Errorf("delta = %+v", d)
	}
	if len(clearance.AggravatedBy(nil, after)) != 0 {
		t.Error("with nothing before, nothing was aggravated")
	}
}

func TestAttributeTakesTheDeepestPriorWhenTheKeyRepeats(t *testing.T) {
	before := clearance.Findings{
		f("NESC-232A", "A3", 150, 155),
		f("NESC-232A", "A3", 130, 155),
	}
	after := clearance.Findings{f("NESC-232A", "A3", 140, 155)}
	got := clearance.Attribute(before, after)
	if got[0].Blame != clearance.BlamePreExisting {
		t.Errorf("the deepest prior reading should govern, got %s", got[0].Blame)
	}
}
