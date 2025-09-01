package tests

import (
	"testing"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/rental"
)

func TestFormulaAssignmentDefaultsToCable(t *testing.T) {
	a := rental.Assignment{"NORTHVALE TELEPHONE": rental.FormulaTelecom}
	if got := a.FormulaFor(" northvale telephone "); got != rental.FormulaTelecom {
		t.Errorf("assigned = %s", got)
	}
	if got := a.FormulaFor("RIDGEPORT CABLEVISION"); got != rental.FormulaCable {
		t.Errorf("unassigned should default to cable, got %s", got)
	}
	if got := rental.Assignment(nil).FormulaFor("anyone"); got != rental.FormulaCable {
		t.Errorf("an empty assignment defaults to cable, got %s", got)
	}
}

func TestBillsOnePerAttacher(t *testing.T) {
	issued := calendar.MustNew(2026, 1, 15)
	got, err := rental.Bills(sampleInventory(t), rental.Assignment{"NORTHVALE TELEPHONE": rental.FormulaTelecom}, issued, "RB-2025")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("bills = %v", got)
	}
	if got[0].Owner != "MERIDIAN FIBER" {
		t.Errorf("bills should be sorted by attacher, got %v", got)
	}
	if got[0].Number != "RB-2025-001" {
		t.Errorf("number = %q", got[0].Number)
	}
	if got[0].Due.String() != "2026-03-01" {
		t.Errorf("due %s", got[0].Due)
	}
	for _, b := range got {
		if err := b.Validate(); err != nil {
			t.Errorf("%s does not validate: %v", b.Number, err)
		}
		if b.String() == "" {
			t.Errorf("%s prints nothing", b.Number)
		}
	}
	if rental.BillTotal(got).IsZero() {
		t.Error("the run should bill something")
	}
}

func TestTheTelecomAttacherPaysMore(t *testing.T) {
	issued := calendar.MustNew(2026, 1, 15)
	cable, err := rental.Bills(sampleInventory(t), nil, issued, "RB")
	if err != nil {
		t.Fatal(err)
	}
	telecom, err := rental.Bills(sampleInventory(t), rental.Assignment{
		"NORTHVALE TELEPHONE": rental.FormulaTelecom,
	}, issued, "RB")
	if err != nil {
		t.Fatal(err)
	}
	var cableAmount, telecomAmount = findBill(t, cable, "NORTHVALE TELEPHONE"), findBill(t, telecom, "NORTHVALE TELEPHONE")
	if telecomAmount.Total <= cableAmount.Total {
		t.Errorf("the telecom formula bills more here: %s against %s", telecomAmount.Total, cableAmount.Total)
	}
	if telecomAmount.Formula != rental.FormulaTelecom {
		t.Errorf("formula = %s", telecomAmount.Formula)
	}
}

func findBill(t *testing.T, bills []rental.Bill, owner string) rental.Bill {
	t.Helper()
	for _, b := range bills {
		if b.Owner == owner {
			return b
		}
	}
	t.Fatalf("no bill for %s", owner)
	return rental.Bill{}
}

func TestAPartYearAttachmentIsProrated(t *testing.T) {
	got, err := rental.Bills(sampleInventory(t), nil, calendar.MustNew(2026, 1, 15), "RB")
	if err != nil {
		t.Fatal(err)
	}
	meridian := findBill(t, got, "MERIDIAN FIBER")
	if len(meridian.Lines) != 1 {
		t.Fatalf("lines = %v", meridian.Lines)
	}
	line := meridian.Lines[0]
	if line.FullYear() {
		t.Error("the fiber went up in July")
	}
	if line.Amount >= line.FullRate {
		t.Errorf("a part year should cost less than a full one: %s against %s", line.Amount, line.FullRate)
	}
	if line.Days != 184 || line.OfDays != 365 {
		t.Errorf("days = %d of %d", line.Days, line.OfDays)
	}
	if len(meridian.Prorated()) != 1 {
		t.Errorf("prorated = %v", meridian.Prorated())
	}
	ridgeport := findBill(t, got, "RIDGEPORT CABLEVISION")
	if len(ridgeport.Prorated()) != 0 {
		t.Errorf("the cable company was up all year, got %v", ridgeport.Prorated())
	}
}

func TestAnUnauthorizedAttachmentIsFlagged(t *testing.T) {
	got, err := rental.Bills(sampleInventory(t), nil, calendar.MustNew(2026, 1, 15), "RB")
	if err != nil {
		t.Fatal(err)
	}
	unidentified := findBill(t, got, "UNIDENTIFIED")
	flagged := unidentified.Flagged()
	if len(flagged) != 1 || flagged[0].AttachmentID != "B9" {
		t.Errorf("flagged = %v", flagged)
	}
	if flagged[0].Amount.IsZero() {
		t.Error("an unauthorised attachment is still billed")
	}
	ridgeport := findBill(t, got, "RIDGEPORT CABLEVISION")
	if len(ridgeport.Flagged()) != 0 {
		t.Errorf("nothing to flag, got %v", ridgeport.Flagged())
	}
	if len(ridgeport.Poles()) != 2 {
		t.Errorf("poles = %v", ridgeport.Poles())
	}
}

func TestBillsRejectsBadInput(t *testing.T) {
	inv := sampleInventory(t)
	issued := calendar.MustNew(2026, 1, 15)
	if _, err := rental.Bills(nil, nil, issued, "RB"); err == nil {
		t.Error("no inventory should be rejected")
	}
	if _, err := rental.Bills(inv, nil, calendar.Zero, "RB"); err == nil {
		t.Error("no issue date should be rejected")
	}
	if _, err := rental.Bills(inv, nil, issued, " "); err == nil {
		t.Error("no prefix should be rejected")
	}
	unfiled := rental.NewInventory(calendar.RateYear(1999))
	if err := unfiled.Add(occ("A3", "NW-441829", "X", true, "1999-01-01", "")); err != nil {
		t.Fatal(err)
	}
	if _, err := rental.Bills(unfiled, nil, issued, "RB"); err == nil {
		t.Error("a year with no filing cannot be billed")
	}
}

func TestBillValidateCatchesABadBill(t *testing.T) {
	got, err := rental.Bills(sampleInventory(t), nil, calendar.MustNew(2026, 1, 15), "RB")
	if err != nil {
		t.Fatal(err)
	}
	good := got[0]
	cases := map[string]func(*rental.Bill){
		"no number":  func(b *rental.Bill) { b.Number = " " },
		"no owner":   func(b *rental.Bill) { b.Owner = "" },
		"bad method": func(b *rental.Bill) { b.Formula = rental.Formula("wireless") },
		"undated":    func(b *rental.Bill) { b.Issued = calendar.Zero },
		"backwards":  func(b *rental.Bill) { b.Due = b.Issued.AddDays(-1) },
		"no lines":   func(b *rental.Bill) { b.Lines = nil },
		"bad total":  func(b *rental.Bill) { b.Total = b.Total.Add(b.Total) },
		"credit":     func(b *rental.Bill) { b.Lines[0].Amount = -1 },
	}
	for name, mutate := range cases {
		b := good
		b.Lines = append([]rental.BillLine{}, good.Lines...)
		mutate(&b)
		if err := b.Validate(); err == nil {
			t.Errorf("%s should have been rejected", name)
		}
	}
	if rental.BillTotal(nil).IsZero() != true {
		t.Error("no bills, no money")
	}
}
