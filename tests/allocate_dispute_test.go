package tests

import (
	"testing"

	"github.com/glinskevxer/downguy/internal/allocate"
	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/unit"
)

func sampleInvoice() allocate.Invoice {
	return allocate.Invoice{
		Number: "MRI-001", Application: "APP-1042", Party: "MERIDIAN FIBER",
		Account: "ATT-9", Role: allocate.RoleApplicant,
		Issued: calendar.MustNew(2025, 8, 1), Due: calendar.MustNew(2025, 8, 31),
		Lines: []allocate.InvoiceLine{
			{ItemID: "MR-0001", Pole: "NW-441829", Amount: unit.Dollars(600)},
			{ItemID: "MR-0002", Pole: "NW-441829", Amount: unit.Dollars(400)},
		},
		Total: unit.Dollars(1000),
	}
}

func openDispute() allocate.Dispute {
	return allocate.Dispute{
		ID: "DIS-1", Invoice: "MRI-001", ItemID: "MR-0001", Party: "MERIDIAN FIBER",
		Ground: allocate.GroundAttribution, Raised: calendar.MustNew(2025, 8, 12),
		Claimed: unit.Dollars(600), Outcome: allocate.OutcomeOpen,
	}
}

func TestGroundsAreNamed(t *testing.T) {
	t.Parallel()
	grounds := allocate.Grounds()
	if len(grounds) != 4 {
		t.Fatalf("grounds = %v", grounds)
	}
	for i := 1; i < len(grounds); i++ {
		if grounds[i] < grounds[i-1] {
			t.Fatalf("grounds are not sorted: %v", grounds)
		}
	}
	for _, g := range grounds {
		if !allocate.KnownGround(g) || g.Name() == "" {
			t.Errorf("ground = %q", g)
		}
	}
	if allocate.KnownGround(allocate.Ground("we do not like it")) {
		t.Error("that is not a ground")
	}
	if allocate.Ground("x").Name() != "unrecognised ground x" {
		t.Errorf("name = %q", allocate.Ground("x").Name())
	}
	for _, o := range []allocate.Outcome{allocate.OutcomeOpen, allocate.OutcomeUpheld,
		allocate.OutcomeRefused, allocate.OutcomeWithdrawn} {
		if !allocate.KnownOutcome(o) {
			t.Errorf("%s should be a known outcome", o)
		}
	}
	if allocate.KnownOutcome(allocate.Outcome("pending")) {
		t.Error("pending is not an outcome")
	}
}

func TestDisputeValidateAccepts(t *testing.T) {
	t.Parallel()
	if err := openDispute().Validate(); err != nil {
		t.Fatalf("an open dispute failed: %v", err)
	}
	upheld := openDispute()
	upheld.Outcome = allocate.OutcomeUpheld
	upheld.Decided = calendar.MustNew(2025, 9, 1)
	upheld.Credited = unit.Dollars(600)
	upheld.Reason = "the pole was already over before the application"
	if err := upheld.Validate(); err != nil {
		t.Fatalf("an upheld dispute failed: %v", err)
	}
	refused := openDispute()
	refused.Outcome = allocate.OutcomeRefused
	refused.Decided = calendar.MustNew(2025, 9, 1)
	refused.Reason = "the fiber is what put the pole over"
	if err := refused.Validate(); err != nil {
		t.Fatalf("a refused dispute failed: %v", err)
	}
}

func TestDisputeValidateRejects(t *testing.T) {
	t.Parallel()
	cases := map[string]func(*allocate.Dispute){
		"no id":       func(d *allocate.Dispute) { d.ID = " " },
		"no invoice":  func(d *allocate.Dispute) { d.Invoice = "" },
		"no party":    func(d *allocate.Dispute) { d.Party = "" },
		"bad ground":  func(d *allocate.Dispute) { d.Ground = allocate.Ground("vibes") },
		"no date":     func(d *allocate.Dispute) { d.Raised = calendar.Zero },
		"no claim":    func(d *allocate.Dispute) { d.Claimed = 0 },
		"bad outcome": func(d *allocate.Dispute) { d.Outcome = allocate.Outcome("pending") },
		"open decided": func(d *allocate.Dispute) {
			d.Decided = calendar.MustNew(2025, 9, 1)
		},
		"open credited": func(d *allocate.Dispute) { d.Credited = unit.Dollars(1) },
		"decided undated": func(d *allocate.Dispute) {
			d.Outcome = allocate.OutcomeRefused
			d.Reason = "no"
		},
		"decided backwards": func(d *allocate.Dispute) {
			d.Outcome = allocate.OutcomeRefused
			d.Decided = calendar.MustNew(2025, 8, 1)
			d.Reason = "no"
		},
		"decided unreasoned": func(d *allocate.Dispute) {
			d.Outcome = allocate.OutcomeRefused
			d.Decided = calendar.MustNew(2025, 9, 1)
		},
		"over credited": func(d *allocate.Dispute) {
			d.Outcome = allocate.OutcomeUpheld
			d.Decided = calendar.MustNew(2025, 9, 1)
			d.Reason = "yes"
			d.Credited = unit.Dollars(900)
		},
		"refused with credit": func(d *allocate.Dispute) {
			d.Outcome = allocate.OutcomeRefused
			d.Decided = calendar.MustNew(2025, 9, 1)
			d.Reason = "no"
			d.Credited = unit.Dollars(100)
		},
		"upheld with nothing": func(d *allocate.Dispute) {
			d.Outcome = allocate.OutcomeUpheld
			d.Decided = calendar.MustNew(2025, 9, 1)
			d.Reason = "yes"
		},
	}
	for name, mutate := range cases {
		d := openDispute()
		mutate(&d)
		if err := d.Validate(); err == nil {
			t.Errorf("%s should have been rejected", name)
		}
	}
}

func TestDisputeWindow(t *testing.T) {
	t.Parallel()
	invoiced := calendar.MustNew(2025, 8, 1)
	d := openDispute()
	if !d.InTime(invoiced) {
		t.Error("eleven days is inside a thirty day window")
	}
	d.Raised = invoiced.AddDays(allocate.DisputeWindowDays)
	if !d.InTime(invoiced) {
		t.Error("the last day of the window is inside it")
	}
	d.Raised = invoiced.AddDays(allocate.DisputeWindowDays + 1)
	if d.InTime(invoiced) {
		t.Error("the day after is not")
	}
	d.Raised = invoiced.AddDays(-1)
	if d.InTime(invoiced) {
		t.Error("a dispute cannot be raised before the invoice")
	}
	if d.InTime(calendar.Zero) {
		t.Error("with no invoice date there is no window")
	}
}

func TestSettleAppliesUpheldAndWithholdsOpen(t *testing.T) {
	t.Parallel()
	upheld := openDispute()
	upheld.ID = "DIS-2"
	upheld.ItemID = "MR-0002"
	upheld.Claimed = unit.Dollars(400)
	upheld.Outcome = allocate.OutcomeUpheld
	upheld.Decided = calendar.MustNew(2025, 9, 1)
	upheld.Credited = unit.Dollars(250)
	upheld.Reason = "priced above the book"

	got, err := allocate.Settle(sampleInvoice(), []allocate.Dispute{openDispute(), upheld})
	if err != nil {
		t.Fatal(err)
	}
	if got.Credited != unit.Dollars(250) {
		t.Errorf("credited = %s", got.Credited)
	}
	if got.Withheld != unit.Dollars(600) {
		t.Errorf("withheld = %s", got.Withheld)
	}
	if got.Payable != unit.Dollars(150) {
		t.Errorf("payable = %s", got.Payable)
	}
	if got.Settled() {
		t.Error("one dispute is still open")
	}
	if len(got.OpenDisputes()) != 1 {
		t.Errorf("open = %v", got.OpenDisputes())
	}
	if got.Disputes[0].ID != "DIS-1" {
		t.Errorf("disputes should be sorted by id, got %v", got.Disputes)
	}
	if got.Summary() == "" {
		t.Error("Summary should say something")
	}
}

func TestSettleIgnoresWhatDoesNotApply(t *testing.T) {
	t.Parallel()
	other := openDispute()
	other.ID = "DIS-9"
	other.Invoice = "MRI-002"

	late := openDispute()
	late.ID = "DIS-8"
	late.Raised = calendar.MustNew(2025, 10, 1)

	got, err := allocate.Settle(sampleInvoice(), []allocate.Dispute{other, late})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Disputes) != 0 {
		t.Errorf("neither applies, got %v", got.Disputes)
	}
	if got.Payable != unit.Dollars(1000) {
		t.Errorf("payable = %s", got.Payable)
	}
	if !got.Settled() {
		t.Error("nothing is open")
	}
}

func TestSettleNeverGoesNegative(t *testing.T) {
	t.Parallel()
	big := openDispute()
	big.Claimed = unit.Dollars(1000)
	second := openDispute()
	second.ID = "DIS-2"
	second.Claimed = unit.Dollars(1000)
	got, err := allocate.Settle(sampleInvoice(), []allocate.Dispute{big, second})
	if err != nil {
		t.Fatal(err)
	}
	if got.Payable.AsCents() != 0 {
		t.Errorf("payable = %s", got.Payable)
	}
}

func TestSettleRejectsBadInput(t *testing.T) {
	t.Parallel()
	bad := sampleInvoice()
	bad.Total = unit.Dollars(1)
	if _, err := allocate.Settle(bad, nil); err == nil {
		t.Error("an invoice whose lines do not add up should be rejected")
	}
	broken := openDispute()
	broken.Ground = allocate.Ground("vibes")
	if _, err := allocate.Settle(sampleInvoice(), []allocate.Dispute{broken}); err == nil {
		t.Error("an invalid dispute should be rejected")
	}
}
