package tests

import (
	"context"
	"errors"
	"testing"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/rental"
	"github.com/glinskevxer/downguy/internal/store"
	"github.com/glinskevxer/downguy/internal/unit"
)

func billingRepo(t *testing.T) (*store.Billing, context.Context) {
	t.Helper()
	return store.NewBilling(openStore(t)), context.Background()
}

func publishedRate(year int, cents int64) store.PublishedRate {
	return store.PublishedRate{
		Year: calendar.RateYear(year), Formula: rental.FormulaCable,
		Annual: unit.Cents(cents), Published: calendar.MustNew(year, 1, 15),
	}
}

func TestPublishAndReadARate(t *testing.T) {
	t.Parallel()
	repo, ctx := billingRepo(t)
	if err := repo.PublishRate(ctx, publishedRate(2025, 1842)); err != nil {
		t.Fatal(err)
	}
	got, err := repo.Rate(ctx, calendar.RateYear(2025))
	if err != nil {
		t.Fatal(err)
	}
	if got.Annual.AsCents() != 1842 || got.Formula != rental.FormulaCable {
		t.Errorf("rate = %+v", got)
	}
	if got.Published.String() != "2025-01-15" {
		t.Errorf("published %s", got.Published)
	}
	if _, err := repo.Rate(ctx, calendar.RateYear(1999)); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("error = %v", err)
	}
}

func TestPublishRefusesABadRate(t *testing.T) {
	t.Parallel()
	repo, ctx := billingRepo(t)
	bad := publishedRate(2025, 1842)
	bad.Formula = rental.Formula("wireless")
	if err := repo.PublishRate(ctx, bad); err == nil {
		t.Error("an unknown formula should be rejected")
	}
	bad = publishedRate(2025, 0)
	if err := repo.PublishRate(ctx, bad); err == nil {
		t.Error("a free rate should be rejected")
	}
	bad = publishedRate(2025, 1842)
	bad.Published = calendar.Zero
	if err := repo.PublishRate(ctx, bad); err == nil {
		t.Error("an undated rate should be rejected")
	}
	years, err := repo.PublishedYears(ctx)
	if err != nil || len(years) != 0 {
		t.Errorf("years = %v %v", years, err)
	}
}

func TestPublishReplacesAnEarlierFiling(t *testing.T) {
	t.Parallel()
	repo, ctx := billingRepo(t)
	if err := repo.PublishRate(ctx, publishedRate(2025, 1842)); err != nil {
		t.Fatal(err)
	}
	corrected := publishedRate(2025, 1908)
	corrected.Formula = rental.FormulaTelecom
	if err := repo.PublishRate(ctx, corrected); err != nil {
		t.Fatal(err)
	}
	got, err := repo.Rate(ctx, calendar.RateYear(2025))
	if err != nil {
		t.Fatal(err)
	}
	if got.Annual.AsCents() != 1908 || got.Formula != rental.FormulaTelecom {
		t.Errorf("the later filing should win, got %+v", got)
	}
	years, err := repo.PublishedYears(ctx)
	if err != nil || len(years) != 1 {
		t.Errorf("years = %v %v", years, err)
	}
}

func TestPublishedYearsAreOrdered(t *testing.T) {
	t.Parallel()
	repo, ctx := billingRepo(t)
	for _, y := range []int{2026, 2024, 2025} {
		if err := repo.PublishRate(ctx, publishedRate(y, 1800)); err != nil {
			t.Fatal(err)
		}
	}
	got, err := repo.PublishedYears(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0] != 2024 || got[2] != 2026 {
		t.Errorf("years = %v", got)
	}
}

func billRun(t *testing.T) []rental.Bill {
	t.Helper()
	bills, err := rental.Bills(sampleInventory(t), nil, calendar.MustNew(2026, 1, 15), "RB-2025")
	if err != nil {
		t.Fatal(err)
	}
	return bills
}

func TestSaveAndReadBills(t *testing.T) {
	t.Parallel()
	repo, ctx := billingRepo(t)
	bills := billRun(t)
	if err := repo.SaveBills(ctx, bills); err != nil {
		t.Fatal(err)
	}
	inYear, err := repo.BillsInYear(ctx, calendar.RateYear(2025))
	if err != nil {
		t.Fatal(err)
	}
	if len(inYear) != len(bills) {
		t.Fatalf("bills = %v", inYear)
	}
	if inYear[0].Owner != "MERIDIAN FIBER" {
		t.Errorf("bills should come back by attacher, got %v", inYear)
	}
	if inYear[0].Issued.String() != "2026-01-15" || inYear[0].Due.String() != "2026-03-01" {
		t.Errorf("dates = %s to %s", inYear[0].Issued, inYear[0].Due)
	}
	total, err := repo.YearTotal(ctx, calendar.RateYear(2025))
	if err != nil {
		t.Fatal(err)
	}
	if total != rental.BillTotal(bills) {
		t.Errorf("total = %s, want %s", total, rental.BillTotal(bills))
	}
	empty, err := repo.YearTotal(ctx, calendar.RateYear(2024))
	if err != nil || !empty.IsZero() {
		t.Errorf("an unbilled year = %s %v", empty, err)
	}
}

func TestBillsForOneAttacher(t *testing.T) {
	t.Parallel()
	repo, ctx := billingRepo(t)
	if err := repo.SaveBills(ctx, billRun(t)); err != nil {
		t.Fatal(err)
	}
	got, err := repo.BillsFor(ctx, "RIDGEPORT CABLEVISION")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("bills = %v", got)
	}
	if got[0].Year != 2025 || got[0].Formula != rental.FormulaCable {
		t.Errorf("bill = %+v", got[0])
	}
	if none, err := repo.BillsFor(ctx, "nobody"); err != nil || len(none) != 0 {
		t.Errorf("nobody is billed nothing, got %v %v", none, err)
	}
}

func TestSaveBillsRefusesABadRun(t *testing.T) {
	t.Parallel()
	repo, ctx := billingRepo(t)
	bills := billRun(t)
	bills[0].Number = ""
	if err := repo.SaveBills(ctx, bills); err == nil {
		t.Error("a run with a bad bill should be refused whole")
	}
	inYear, err := repo.BillsInYear(ctx, calendar.RateYear(2025))
	if err != nil || len(inYear) != 0 {
		t.Errorf("nothing should have been written, got %v %v", inYear, err)
	}
}

func TestSaveBillsIsRepeatable(t *testing.T) {
	t.Parallel()
	repo, ctx := billingRepo(t)
	bills := billRun(t)
	if err := repo.SaveBills(ctx, bills); err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveBills(ctx, bills); err != nil {
		t.Fatal(err)
	}
	got, err := repo.BillsInYear(ctx, calendar.RateYear(2025))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(bills) {
		t.Errorf("running the same billing twice should not double it, got %d", len(got))
	}
}
