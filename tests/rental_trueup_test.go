package tests

import (
	"testing"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/rental"
)

func billedRun(t *testing.T, inv *rental.Inventory) []rental.Bill {
	t.Helper()
	bills, err := rental.Bills(inv, nil, calendar.MustNew(2026, 1, 15), "RB")
	if err != nil {
		t.Fatal(err)
	}
	return bills
}

func TestReconcileOnAnUnchangedSystem(t *testing.T) {
	t.Parallel()
	inv := sampleInventory(t)
	got, err := rental.Reconcile(calendar.RateYear(2025), billedRun(t, inv), inv, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Balanced() {
		t.Errorf("nothing moved, so nothing should be owed: %v", got.Moved())
	}
	if !got.Delta.IsZero() {
		t.Errorf("delta = %s", got.Delta)
	}
	if got.Billed != got.Billable || got.Billed.IsZero() {
		t.Errorf("billed %s billable %s", got.Billed, got.Billable)
	}
	if got.Summary() == "" {
		t.Error("Summary should say something")
	}
}

func TestReconcileFindsAnAttachmentThatArrivedLate(t *testing.T) {
	t.Parallel()
	billed := billedRun(t, sampleInventory(t))

	now := sampleInventory(t)
	if err := now.Add(occ("A8", "NW-441829", "MERIDIAN FIBER", true, "2025-01-01", "")); err != nil {
		t.Fatal(err)
	}
	got, err := rental.Reconcile(calendar.RateYear(2025), billed, now, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Balanced() {
		t.Fatal("an attachment appeared, so something moved")
	}
	owing := got.Owing()
	if len(owing) == 0 {
		t.Fatalf("somebody owes more: %v", got.Variances)
	}
	found := false
	for _, v := range owing {
		if v.Owner == "MERIDIAN FIBER" {
			found = true
			if len(v.Added) != 1 || v.Added[0] != "A8" {
				t.Errorf("added = %v", v.Added)
			}
			if v.NowOn <= v.BilledFor {
				t.Errorf("variance = %+v", v)
			}
			if v.String() == "" {
				t.Error("String should say something")
			}
		}
	}
	if !found {
		t.Errorf("the fiber company owes more, got %v", owing)
	}
}

func TestReconcileCreditsAnAttacherThatCameDown(t *testing.T) {
	t.Parallel()
	billed := billedRun(t, sampleInventory(t))

	now := rental.NewInventory(calendar.RateYear(2025))
	for _, o := range sampleInventory(t).Occupancies {
		if o.Owner == "NORTHVALE TELEPHONE" {
			continue
		}
		if err := now.Add(o); err != nil {
			t.Fatal(err)
		}
	}
	got, err := rental.Reconcile(calendar.RateYear(2025), billed, now, nil)
	if err != nil {
		t.Fatal(err)
	}
	credits := got.Credits()
	if len(credits) == 0 {
		t.Fatalf("the telephone company is owed a credit: %v", got.Variances)
	}
	var telephone rental.Variance
	for _, v := range credits {
		if v.Owner == "NORTHVALE TELEPHONE" {
			telephone = v
		}
	}
	if telephone.Owner == "" {
		t.Fatalf("credits = %v", credits)
	}
	if !telephone.Billable.IsZero() || telephone.Billed.IsZero() {
		t.Errorf("variance = %+v", telephone)
	}
	if len(telephone.Removed) != 1 || telephone.Removed[0] != "A4" {
		t.Errorf("removed = %v", telephone.Removed)
	}
	if !telephone.Credit() || telephone.Owes() {
		t.Errorf("variance = %+v", telephone)
	}
}

func TestReconcileFoldsAnAttacherNameWrittenTwoWays(t *testing.T) {
	t.Parallel()
	billed := billedRun(t, sampleInventory(t))
	for i := range billed {
		if billed[i].Owner == "RIDGEPORT CABLEVISION" {
			billed[i].Owner = "  ridgeport   cablevision "
		}
	}
	got, err := rental.Reconcile(calendar.RateYear(2025), billed, sampleInventory(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Variances) != 4 {
		t.Errorf("one company written two ways is one company, got %v", got.Variances)
	}
	if !got.Balanced() {
		t.Errorf("nothing actually moved: %v", got.Moved())
	}
}

func TestReconcileIgnoresAnotherYearsBills(t *testing.T) {
	t.Parallel()
	inv := sampleInventory(t)
	billed := billedRun(t, inv)
	for i := range billed {
		billed[i].Year = 2024
	}
	got, err := rental.Reconcile(calendar.RateYear(2025), billed, inv, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Billed.AsCents() != 0 {
		t.Errorf("nothing was billed for 2025, got %s", got.Billed)
	}
	if len(got.Owing()) == 0 {
		t.Error("so everything on the system is owed")
	}
}

func TestReconcileRejectsBadInput(t *testing.T) {
	t.Parallel()
	if _, err := rental.Reconcile(calendar.RateYear(2025), nil, nil, nil); err == nil {
		t.Error("no inventory should be rejected")
	}
	wrongYear := rental.NewInventory(calendar.RateYear(2024))
	if _, err := rental.Reconcile(calendar.RateYear(2025), nil, wrongYear, nil); err == nil {
		t.Error("an inventory for another year should be rejected")
	}
	unfiled := rental.NewInventory(calendar.RateYear(1999))
	if err := unfiled.Add(occ("A3", "NW-441829", "X", true, "1999-01-01", "")); err != nil {
		t.Fatal(err)
	}
	if _, err := rental.Reconcile(calendar.RateYear(1999), nil, unfiled, nil); err == nil {
		t.Error("a year with no filed rate cannot be reconciled")
	}
}
