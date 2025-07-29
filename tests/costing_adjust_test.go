package tests

import (
	"testing"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/costing"
	"github.com/glinskevxer/downguy/internal/makeready"
	"github.com/glinskevxer/downguy/internal/unit"
)

func sampleEstimate(t *testing.T, items ...makeready.Item) *costing.Estimate {
	t.Helper()
	if len(items) == 0 {
		items = []makeready.Item{raise("MR-0001", "NW-441829", "RIDGEPORT CABLEVISION")}
	}
	e, err := costing.Price(planWith(items...), costing.Standard(), calendar.MustNew(2025, 6, 1))
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestMoneyAbs(t *testing.T) {
	if unit.Cents(-250).Abs().AsCents() != 250 {
		t.Error("abs of a debit")
	}
	if unit.Cents(250).Abs().AsCents() != 250 {
		t.Error("abs of a credit")
	}
	if !unit.Cents(0).Abs().IsZero() {
		t.Error("abs of nothing")
	}
}

func TestQuoteAppliesTheStandardAdjustments(t *testing.T) {
	e := sampleEstimate(t)
	q, err := costing.Quote(e, costing.StandardAdjustments(), calendar.MustNew(2025, 6, 1))
	if err != nil {
		t.Fatal(err)
	}
	if q.Direct != e.Total {
		t.Errorf("direct = %s, want %s", q.Direct, e.Total)
	}
	if len(q.Adjustments) != 3 {
		t.Fatalf("adjustments = %v", q.Adjustments)
	}
	contingency, ok := q.Named("contingency")
	if !ok {
		t.Fatal("no contingency line")
	}
	if contingency.On != q.Direct {
		t.Errorf("contingency should apply to the direct cost, got %s", contingency.On)
	}
	if contingency.Amount != unit.Percent(10).Of(q.Direct) {
		t.Errorf("contingency = %s", contingency.Amount)
	}
	admin, ok := q.Named("administrative overhead")
	if !ok {
		t.Fatal("no admin line")
	}
	if admin.On <= q.Direct {
		t.Errorf("the admin overhead runs on the marked up total, got %s against %s", admin.On, q.Direct)
	}
	if q.Total <= q.Direct {
		t.Error("the quoted total should exceed the direct cost")
	}
	if q.AdjustmentTotal() != q.Total.Sub(q.Direct) {
		t.Errorf("adjustment total = %s", q.AdjustmentTotal())
	}
	if _, ok := q.Named("nothing"); ok {
		t.Error("Named should miss")
	}
}

func TestQuoteWithNoAdjustments(t *testing.T) {
	e := sampleEstimate(t)
	q, err := costing.Quote(e, nil, calendar.MustNew(2025, 6, 1))
	if err != nil {
		t.Fatal(err)
	}
	if q.Total != e.Total || !q.AdjustmentTotal().IsZero() {
		t.Errorf("quote = %+v", q)
	}
}

func TestQuoteValidity(t *testing.T) {
	e := sampleEstimate(t)
	issued := calendar.MustNew(2025, 6, 1)
	q, err := costing.Quote(e, costing.StandardAdjustments(), issued)
	if err != nil {
		t.Fatal(err)
	}
	if q.GoodUntil.String() != "2025-08-30" {
		t.Errorf("good until %s", q.GoodUntil)
	}
	if q.Expired(issued) || q.Expired(q.GoodUntil) {
		t.Error("a quote is good on the day it expires")
	}
	if !q.Expired(q.GoodUntil.AddDays(1)) {
		t.Error("and not the day after")
	}
}

func TestQuoteRejectsBadInput(t *testing.T) {
	e := sampleEstimate(t)
	on := calendar.MustNew(2025, 6, 1)
	if _, err := costing.Quote(nil, nil, on); err == nil {
		t.Error("no estimate should be rejected")
	}
	if _, err := costing.Quote(e, nil, calendar.Zero); err == nil {
		t.Error("no issue date should be rejected")
	}
	bad := []costing.Adjustment{{Name: "mystery", Rate: unit.Percent(5), Basis: costing.Basis("vibes")}}
	if _, err := costing.Quote(e, bad, on); err == nil {
		t.Error("an unknown basis should be rejected")
	}
	negative := []costing.Adjustment{{Name: "discount", Rate: unit.Percent(-5), Basis: costing.BasisDirect}}
	if _, err := costing.Quote(e, negative, on); err == nil {
		t.Error("a negative rate should be rejected")
	}
	if !costing.KnownBasis(costing.BasisDirect) || costing.KnownBasis(costing.Basis("vibes")) {
		t.Error("known basis check")
	}
}

func TestReviseShowsWhatMoved(t *testing.T) {
	on := calendar.MustNew(2025, 6, 1)
	small := sampleEstimate(t)
	big := sampleEstimate(t,
		raise("MR-0001", "NW-441829", "RIDGEPORT CABLEVISION"),
		raise("MR-0002", "NW-441829", "NORTHVALE TELEPHONE"),
	)
	from, err := costing.Quote(small, costing.StandardAdjustments(), on)
	if err != nil {
		t.Fatal(err)
	}
	to, err := costing.Quote(big, costing.StandardAdjustments(), on)
	if err != nil {
		t.Fatal(err)
	}
	r, err := costing.Revise(from, to)
	if err != nil {
		t.Fatal(err)
	}
	if r.Cheaper() {
		t.Error("adding work is not cheaper")
	}
	if r.Delta <= 0 {
		t.Errorf("delta = %s", r.Delta)
	}
	if len(r.NewItems) != 1 || r.NewItems[0] != "MR-0002" {
		t.Errorf("new items = %v", r.NewItems)
	}
	if len(r.GoneItems) != 0 {
		t.Errorf("gone items = %v", r.GoneItems)
	}
	back, err := costing.Revise(to, from)
	if err != nil {
		t.Fatal(err)
	}
	if !back.Cheaper() {
		t.Error("dropping work is cheaper")
	}
	if len(back.GoneItems) != 1 || back.GoneItems[0] != "MR-0002" {
		t.Errorf("gone items = %v", back.GoneItems)
	}
	if back.String() == "" {
		t.Error("String should say something")
	}
}

func TestReviseRejectsMissingQuotations(t *testing.T) {
	q, err := costing.Quote(sampleEstimate(t), nil, calendar.MustNew(2025, 6, 1))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := costing.Revise(nil, q); err == nil {
		t.Error("no from should be rejected")
	}
	if _, err := costing.Revise(q, nil); err == nil {
		t.Error("no to should be rejected")
	}
}
