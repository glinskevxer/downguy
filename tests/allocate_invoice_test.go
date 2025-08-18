package tests

import (
	"testing"

	"github.com/glinskevxer/downguy/internal/allocate"
	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/clearance"
	"github.com/glinskevxer/downguy/internal/makeready"
	"github.com/glinskevxer/downguy/internal/unit"
)

func sharedSplit(t *testing.T) *allocate.Split {
	t.Helper()
	f := blamed("NESC-235C2", "A3", clearance.BlamePreExisting)
	return splitFor(t, allocate.StatePolicy(), clearance.Findings{f},
		[]makeready.Item{itemFor("MR-0001", f, "RIDGEPORT CABLEVISION")})
}

func TestInvoicesOnePerParty(t *testing.T) {
	issued := calendar.MustNew(2025, 8, 1)
	got, err := allocate.Invoices(sharedSplit(t), sampleRoster(t), issued, "MRI-2025")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("invoices = %v", got)
	}
	if got[0].Party != "MERIDIAN FIBER" || got[1].Party != "RIDGEPORT CABLEVISION" {
		t.Errorf("invoices should be sorted by party, got %v", got)
	}
	if got[0].Number != "MRI-2025-001" || got[1].Number != "MRI-2025-002" {
		t.Errorf("numbers = %q %q", got[0].Number, got[1].Number)
	}
	if got[0].Account != "ATT-9" || got[0].Role != allocate.RoleApplicant {
		t.Errorf("account details = %+v", got[0])
	}
	if got[0].Due.String() != "2025-08-31" {
		t.Errorf("due %s", got[0].Due)
	}
	for _, i := range got {
		if err := i.Validate(); err != nil {
			t.Errorf("%s does not validate: %v", i.Number, err)
		}
	}
	if allocate.TotalOf(got) != sharedSplit(t).Total {
		t.Errorf("the invoices should add to the split total, got %s", allocate.TotalOf(got))
	}
}

func TestInvoiceLinesCarryThePole(t *testing.T) {
	got, err := allocate.Invoices(sharedSplit(t), sampleRoster(t), calendar.MustNew(2025, 8, 1), "MRI")
	if err != nil {
		t.Fatal(err)
	}
	poles := got[0].Poles()
	if len(poles) != 1 || poles[0] != "NW-441829" {
		t.Errorf("poles = %v", poles)
	}
	if got[0].Lines[0].ItemID != "MR-0001" {
		t.Errorf("line = %+v", got[0].Lines[0])
	}
	if got[0].String() == "" {
		t.Error("String should say something")
	}
}

func TestAPartyOffTheRosterStillGetsAnInvoice(t *testing.T) {
	f := blamed("NESC-235C2", "A9", clearance.BlamePreExisting)
	s := splitFor(t, allocate.FederalPolicy(), clearance.Findings{f},
		[]makeready.Item{itemFor("MR-0001", f, "UNIDENTIFIED")})
	got, err := allocate.Invoices(s, allocate.NewRoster(), calendar.MustNew(2025, 8, 1), "MRI")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("invoices = %v", got)
	}
	if got[0].Account != "" || got[0].Role != allocate.RoleUnknown {
		t.Errorf("an unrostered party carries no account, got %+v", got[0])
	}
}

func TestInvoicesRejectsMissingInput(t *testing.T) {
	s := sharedSplit(t)
	r := sampleRoster(t)
	issued := calendar.MustNew(2025, 8, 1)
	if _, err := allocate.Invoices(nil, r, issued, "MRI"); err == nil {
		t.Error("no split should be rejected")
	}
	if _, err := allocate.Invoices(s, nil, issued, "MRI"); err == nil {
		t.Error("no roster should be rejected")
	}
	if _, err := allocate.Invoices(s, r, calendar.Zero, "MRI"); err == nil {
		t.Error("no issue date should be rejected")
	}
	if _, err := allocate.Invoices(s, r, issued, "  "); err == nil {
		t.Error("no prefix should be rejected")
	}
}

func TestInvoiceAging(t *testing.T) {
	got, err := allocate.Invoices(sharedSplit(t), sampleRoster(t), calendar.MustNew(2025, 8, 1), "MRI")
	if err != nil {
		t.Fatal(err)
	}
	i := got[0]
	if i.Overdue(i.Due, false) {
		t.Error("an invoice is not overdue on its due date")
	}
	if !i.Overdue(i.Due.AddDays(1), false) {
		t.Error("it is the day after")
	}
	if i.Overdue(i.Due.AddDays(30), true) {
		t.Error("a paid invoice is never overdue")
	}
	if got := i.DaysLate(i.Due.AddDays(14)); got != 14 {
		t.Errorf("days late = %d", got)
	}
	if got := i.DaysLate(i.Due); got != 0 {
		t.Errorf("days late = %d", got)
	}
	undated := allocate.Invoice{Number: "MRI-1", Party: "X"}
	if undated.Overdue(calendar.MustNew(2030, 1, 1), false) {
		t.Error("an undated invoice cannot be overdue")
	}
	if undated.DaysLate(calendar.MustNew(2030, 1, 1)) != 0 {
		t.Error("nor late")
	}
}

func TestInvoiceValidateCatchesABadInvoice(t *testing.T) {
	good := allocate.Invoice{
		Number: "MRI-1", Application: "APP-1042", Party: "MERIDIAN FIBER",
		Issued: calendar.MustNew(2025, 8, 1), Due: calendar.MustNew(2025, 8, 31),
		Lines: []allocate.InvoiceLine{{ItemID: "MR-0001", Amount: unit.Dollars(100)}},
		Total: unit.Dollars(100),
	}
	if err := good.Validate(); err != nil {
		t.Fatalf("a good invoice failed: %v", err)
	}
	cases := map[string]func(*allocate.Invoice){
		"no number": func(i *allocate.Invoice) { i.Number = " " },
		"no party":  func(i *allocate.Invoice) { i.Party = "" },
		"no dates":  func(i *allocate.Invoice) { i.Issued = calendar.Zero },
		"no due":    func(i *allocate.Invoice) { i.Due = calendar.Zero },
		"backwards": func(i *allocate.Invoice) { i.Due = calendar.MustNew(2025, 7, 1) },
		"no lines":  func(i *allocate.Invoice) { i.Lines = nil },
		"bad total": func(i *allocate.Invoice) { i.Total = unit.Dollars(200) },
	}
	for name, mutate := range cases {
		inv := good
		inv.Lines = append([]allocate.InvoiceLine{}, good.Lines...)
		mutate(&inv)
		if err := inv.Validate(); err == nil {
			t.Errorf("%s should have been rejected", name)
		}
	}
}

func TestTotalOfNoInvoices(t *testing.T) {
	if !allocate.TotalOf(nil).IsZero() {
		t.Error("no invoices, no money")
	}
}
