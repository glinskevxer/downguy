package tests

import (
	"testing"

	"github.com/glinskevxer/downguy/internal/allocate"
	"github.com/glinskevxer/downguy/internal/clearance"
	"github.com/glinskevxer/downguy/internal/unit"
)

func TestKnownPayers(t *testing.T) {
	for _, p := range []allocate.Payer{allocate.PayerApplicant, allocate.PayerOwner, allocate.PayerExisting, allocate.PayerShared} {
		if !allocate.KnownPayer(p) {
			t.Errorf("%s should be known", p)
		}
	}
	if allocate.KnownPayer(allocate.Payer("the state")) {
		t.Error("the state is not a payer here")
	}
}

func TestPolicyValidate(t *testing.T) {
	if err := allocate.FederalPolicy().Validate(); err != nil {
		t.Errorf("the federal policy failed: %v", err)
	}
	if err := allocate.StatePolicy().Validate(); err != nil {
		t.Errorf("the state policy failed: %v", err)
	}
	bad := allocate.FederalPolicy()
	bad.Name = ""
	if err := bad.Validate(); err == nil {
		t.Error("a nameless policy should be rejected")
	}
	bad = allocate.FederalPolicy()
	bad.PreExistingPayer = allocate.Payer("the state")
	if err := bad.Validate(); err == nil {
		t.Error("an unknown payer should be rejected")
	}
	bad = allocate.StatePolicy()
	bad.ApplicantShare = 0
	if err := bad.Validate(); err == nil {
		t.Error("a shared policy with no share should be rejected")
	}
	bad.ApplicantShare = unit.FullRate
	if err := bad.Validate(); err == nil {
		t.Error("a share of the whole thing is not a share")
	}
}

func TestPolicyLookup(t *testing.T) {
	all := allocate.Policies()
	if len(all) != 2 || all[0].Name != "federal" {
		t.Errorf("policies = %v", all)
	}
	got, err := allocate.PolicyNamed("state commission")
	if err != nil || got.PreExistingPayer != allocate.PayerShared {
		t.Errorf("policy = %+v %v", got, err)
	}
	if _, err := allocate.PolicyNamed("municipal"); err == nil {
		t.Error("there is no municipal policy")
	}
}

func TestFederalPolicyPutsCausedWorkOnTheApplicant(t *testing.T) {
	p := allocate.FederalPolicy()
	for _, blame := range []clearance.Blame{clearance.BlameApplicant, clearance.BlameAggravated} {
		s, err := p.ShareFor(blame)
		if err != nil {
			t.Fatal(err)
		}
		if s[allocate.PayerApplicant] != unit.FullRate || s.Total() != unit.FullRate {
			t.Errorf("%s = %v", blame, s)
		}
	}
	s, err := p.ShareFor(clearance.BlamePreExisting)
	if err != nil {
		t.Fatal(err)
	}
	if s[allocate.PayerExisting] != unit.FullRate {
		t.Errorf("pre existing = %v", s)
	}
	if len(s.Payers()) != 1 {
		t.Errorf("payers = %v", s.Payers())
	}
}

func TestStatePolicySharesPreExistingWork(t *testing.T) {
	p := allocate.StatePolicy()
	s, err := p.ShareFor(clearance.BlamePreExisting)
	if err != nil {
		t.Fatal(err)
	}
	if s[allocate.PayerApplicant] != unit.Percent(25) {
		t.Errorf("applicant share = %s", s[allocate.PayerApplicant])
	}
	if s[allocate.PayerExisting] != unit.Percent(75) {
		t.Errorf("existing share = %s", s[allocate.PayerExisting])
	}
	if s.Total() != unit.FullRate {
		t.Errorf("shares add to %s", s.Total())
	}
	payers := s.Payers()
	if len(payers) != 2 || payers[0] != allocate.PayerApplicant {
		t.Errorf("payers = %v", payers)
	}
}

func TestAPolicyThatDoesNotBillTheApplicant(t *testing.T) {
	p := allocate.FederalPolicy()
	p.ApplicantPaysCaused = false
	p.ApplicantPaysAggravated = false
	s, err := p.ShareFor(clearance.BlameApplicant)
	if err != nil {
		t.Fatal(err)
	}
	if s[allocate.PayerOwner] != unit.FullRate {
		t.Errorf("caused = %v", s)
	}
	s, err = p.ShareFor(clearance.BlameAggravated)
	if err != nil {
		t.Fatal(err)
	}
	if s[allocate.PayerExisting] != unit.FullRate {
		t.Errorf("aggravated = %v", s)
	}
}

func TestShareForRejectsWhatCannotBeBilled(t *testing.T) {
	p := allocate.FederalPolicy()
	if _, err := p.ShareFor(clearance.BlameUnattributed); err == nil {
		t.Error("an unattributed finding cannot be billed")
	}
	if _, err := p.ShareFor(clearance.Blame("the weather")); err == nil {
		t.Error("the weather is not an attribution")
	}
	bad := p
	bad.Name = ""
	if _, err := bad.ShareFor(clearance.BlameApplicant); err == nil {
		t.Error("an invalid policy should be rejected")
	}
}

func TestDivideAlwaysAddsBackToTheWhole(t *testing.T) {
	share := allocate.Share{
		allocate.PayerApplicant: unit.Percent(25),
		allocate.PayerExisting:  unit.Percent(75),
	}
	for _, cents := range []int64{0, 1, 3, 99, 100, 12345, 1000001} {
		amount := unit.Cents(cents)
		got, err := allocate.Divide(amount, share)
		if err != nil {
			t.Fatal(err)
		}
		var total unit.Money
		for _, v := range got {
			total = total.Add(v)
		}
		if total != amount {
			t.Errorf("%d cents split to %d", cents, total.AsCents())
		}
	}
}

func TestDivideGivesTheRemainderToTheLargestShare(t *testing.T) {
	share := allocate.Share{
		allocate.PayerApplicant: unit.Percent(25),
		allocate.PayerExisting:  unit.Percent(75),
	}
	got, err := allocate.Divide(unit.Cents(3), share)
	if err != nil {
		t.Fatal(err)
	}
	if got[allocate.PayerApplicant].AsCents()+got[allocate.PayerExisting].AsCents() != 3 {
		t.Errorf("split = %v", got)
	}
	if got[allocate.PayerExisting] < got[allocate.PayerApplicant] {
		t.Errorf("the larger share should carry at least as much, got %v", got)
	}
}

func TestDivideRejectsAnIncompleteShare(t *testing.T) {
	half := allocate.Share{allocate.PayerApplicant: unit.Percent(50)}
	if _, err := allocate.Divide(unit.Dollars(100), half); err == nil {
		t.Error("shares that do not add to a hundred percent should be rejected")
	}
	over := allocate.Share{
		allocate.PayerApplicant: unit.Percent(60),
		allocate.PayerExisting:  unit.Percent(60),
	}
	if _, err := allocate.Divide(unit.Dollars(100), over); err == nil {
		t.Error("shares over a hundred percent should be rejected")
	}
}
