package tests

import (
	"testing"

	"github.com/glinskevxer/downguy/internal/allocate"
	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/clearance"
	"github.com/glinskevxer/downguy/internal/costing"
	"github.com/glinskevxer/downguy/internal/makeready"
	"github.com/glinskevxer/downguy/internal/unit"
)

func blamed(rule, subject string, blame clearance.Blame) clearance.Finding {
	return clearance.Finding{
		RuleID: rule, Pole: "NW-441829", Subject: subject,
		Owner: "RIDGEPORT CABLEVISION", Blame: blame,
		Measured: unit.Tenths(180), Required: unit.Tenths(200),
	}
}

func itemFor(id string, f clearance.Finding, owner string) makeready.Item {
	return makeready.Item{
		ID: id, Action: makeready.ActionRaise, Pole: "NW-441829",
		Subject: f.Subject, Owner: owner,
		FromHeight: unit.Tenths(180), ToHeight: unit.Tenths(200),
		Because: []string{f.Key()},
	}
}

func splitFor(t *testing.T, p allocate.Policy, findings clearance.Findings, items []makeready.Item) *allocate.Split {
	t.Helper()
	plan := &makeready.Plan{Application: "APP-1042", Items: makeready.Order(items)}
	e, err := costing.Price(plan, costing.Standard(), calendar.MustNew(2025, 6, 1))
	if err != nil {
		t.Fatal(err)
	}
	s, err := allocate.Allocate("APP-1042", p, sampleRoster(t), plan, e, findings)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestAllocateBillsTheApplicantForWhatItCaused(t *testing.T) {
	f := blamed("NESC-235C2", "A3", clearance.BlameApplicant)
	s := splitFor(t, allocate.FederalPolicy(), clearance.Findings{f},
		[]makeready.Item{itemFor("MR-0001", f, "RIDGEPORT CABLEVISION")})
	if len(s.Charges) != 1 {
		t.Fatalf("charges = %v", s.Charges)
	}
	c := s.Charges[0]
	if c.Payer != allocate.PayerApplicant || c.Party != "MERIDIAN FIBER" {
		t.Errorf("charge = %+v", c)
	}
	if c.Amount != s.Total || s.Total.IsZero() {
		t.Errorf("total = %s", s.Total)
	}
	if s.String() == "" {
		t.Error("String should say something")
	}
}

func TestAllocateBillsAnExistingAttacherForAPreExistingCondition(t *testing.T) {
	f := blamed("NESC-235C2", "A3", clearance.BlamePreExisting)
	s := splitFor(t, allocate.FederalPolicy(), clearance.Findings{f},
		[]makeready.Item{itemFor("MR-0001", f, "RIDGEPORT CABLEVISION")})
	if len(s.Charges) != 1 {
		t.Fatalf("charges = %v", s.Charges)
	}
	if s.Charges[0].Payer != allocate.PayerExisting {
		t.Errorf("payer = %s", s.Charges[0].Payer)
	}
	if s.Charges[0].Party != "RIDGEPORT CABLEVISION" {
		t.Errorf("party = %q", s.Charges[0].Party)
	}
}

func TestAPreExistingConditionNobodyOwnsFallsToTheOwner(t *testing.T) {
	f := blamed("NESC-235C2", "A9", clearance.BlamePreExisting)
	s := splitFor(t, allocate.FederalPolicy(), clearance.Findings{f},
		[]makeready.Item{itemFor("MR-0001", f, "UNIDENTIFIED")})
	if len(s.Charges) != 1 {
		t.Fatalf("charges = %v", s.Charges)
	}
	if s.Charges[0].Party != "SUMMIT VALLEY ELECTRIC" {
		t.Errorf("party = %q", s.Charges[0].Party)
	}
}

func TestTheStatePolicySplitsAPreExistingCondition(t *testing.T) {
	f := blamed("NESC-235C2", "A3", clearance.BlamePreExisting)
	s := splitFor(t, allocate.StatePolicy(), clearance.Findings{f},
		[]makeready.Item{itemFor("MR-0001", f, "RIDGEPORT CABLEVISION")})
	if len(s.Charges) != 2 {
		t.Fatalf("charges = %v", s.Charges)
	}
	byPayer := s.ByPayer()
	applicant := byPayer[allocate.PayerApplicant]
	existing := byPayer[allocate.PayerExisting]
	if applicant.IsZero() || existing.IsZero() {
		t.Fatalf("split = %v", byPayer)
	}
	if applicant.Add(existing) != s.Total {
		t.Errorf("the parts do not add to the total: %s and %s against %s", applicant, existing, s.Total)
	}
	if applicant >= existing {
		t.Error("the applicant carries the quarter share")
	}
	byParty := s.ByParty()
	if len(byParty) != 2 {
		t.Errorf("by party = %v", byParty)
	}
	if len(s.ForParty("meridian fiber")) != 1 {
		t.Errorf("for party = %v", s.ForParty("meridian fiber"))
	}
	if len(s.ForParty("nobody")) != 0 {
		t.Error("nobody has no charges")
	}
}

func TestAggravatedWorkGoesToTheApplicant(t *testing.T) {
	f := blamed("NESC-235C2", "A3", clearance.BlameAggravated)
	s := splitFor(t, allocate.FederalPolicy(), clearance.Findings{f},
		[]makeready.Item{itemFor("MR-0001", f, "RIDGEPORT CABLEVISION")})
	if len(s.Charges) != 1 || s.Charges[0].Payer != allocate.PayerApplicant {
		t.Errorf("charges = %v", s.Charges)
	}
	if s.Charges[0].Blame != clearance.BlameAggravated {
		t.Errorf("blame = %s", s.Charges[0].Blame)
	}
}

func TestAnItemCitingTwoFindingsTakesTheHeavierAttribution(t *testing.T) {
	pre := blamed("NESC-235C2", "A3", clearance.BlamePreExisting)
	caused := blamed("NESC-238A", "A3", clearance.BlameApplicant)
	item := itemFor("MR-0001", pre, "RIDGEPORT CABLEVISION")
	item.Because = []string{pre.Key(), caused.Key()}
	s := splitFor(t, allocate.FederalPolicy(), clearance.Findings{pre, caused}, []makeready.Item{item})
	if len(s.Charges) != 1 {
		t.Fatalf("charges = %v", s.Charges)
	}
	if s.Charges[0].Blame != clearance.BlameApplicant {
		t.Errorf("blame = %s, want the heavier of the two", s.Charges[0].Blame)
	}
}

func TestAnUnattributedItemIsNotBilled(t *testing.T) {
	f := blamed("NESC-235C2", "A3", clearance.BlameUnattributed)
	s := splitFor(t, allocate.FederalPolicy(), clearance.Findings{f},
		[]makeready.Item{itemFor("MR-0001", f, "RIDGEPORT CABLEVISION")})
	if len(s.Charges) != 0 {
		t.Errorf("charges = %v", s.Charges)
	}
	if len(s.Unallocated) != 1 || s.Unallocated[0] != "MR-0001" {
		t.Errorf("unallocated = %v", s.Unallocated)
	}
	orphan := itemFor("MR-0002", f, "RIDGEPORT CABLEVISION")
	orphan.Because = []string{"a key nothing carries"}
	s = splitFor(t, allocate.FederalPolicy(), nil, []makeready.Item{orphan})
	if len(s.Unallocated) != 1 {
		t.Errorf("an item citing an unknown finding cannot be billed, got %v", s.Charges)
	}
}

func TestAllocateRejectsMissingInput(t *testing.T) {
	f := blamed("NESC-235C2", "A3", clearance.BlameApplicant)
	plan := &makeready.Plan{Application: "APP-1042", Items: []makeready.Item{itemFor("MR-0001", f, "X")}}
	e, err := costing.Price(plan, costing.Standard(), calendar.MustNew(2025, 6, 1))
	if err != nil {
		t.Fatal(err)
	}
	p := allocate.FederalPolicy()
	if _, err := allocate.Allocate("APP-1042", p, nil, plan, e, nil); err == nil {
		t.Error("no roster should be rejected")
	}
	if _, err := allocate.Allocate("APP-1042", p, allocate.NewRoster(), plan, e, nil); err == nil {
		t.Error("an incomplete roster should be rejected")
	}
	if _, err := allocate.Allocate("APP-1042", p, sampleRoster(t), nil, e, nil); err == nil {
		t.Error("no plan should be rejected")
	}
	if _, err := allocate.Allocate("APP-1042", p, sampleRoster(t), plan, nil, nil); err == nil {
		t.Error("no estimate should be rejected")
	}
	bad := p
	bad.Name = ""
	if _, err := allocate.Allocate("APP-1042", bad, sampleRoster(t), plan, e, nil); err == nil {
		t.Error("an invalid policy should be rejected")
	}
}
