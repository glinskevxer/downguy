package tests

import (
	"errors"
	"testing"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/costing"
	"github.com/glinskevxer/downguy/internal/unit"
)

func TestBookAddAndPrice(t *testing.T) {
	b := costing.NewBook("test")
	err := b.Add(calendar.MustNew(2025, 1, 1), costing.Entry{
		Code: costing.CodeGuyGuard, Description: "guard", Unit: "each", Price: unit.Dollars(38),
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := b.Price(costing.CodeGuyGuard, calendar.MustNew(2025, 6, 1))
	if err != nil {
		t.Fatal(err)
	}
	if got.AsCents() != 3800 {
		t.Errorf("price = %s", got)
	}
	if _, err := b.Price(costing.CodeGuyGuard, calendar.MustNew(2024, 6, 1)); !errors.Is(err, costing.ErrNoPrice) {
		t.Errorf("a date before the first entry has no price: %v", err)
	}
	if _, err := b.Price(costing.CodeAnchorRod, calendar.MustNew(2025, 6, 1)); !errors.Is(err, costing.ErrNoPrice) {
		t.Errorf("an uncoded item has no price: %v", err)
	}
}

func TestBookRejectsABadEntry(t *testing.T) {
	b := costing.NewBook("test")
	on := calendar.MustNew(2025, 1, 1)
	cases := map[string]costing.Entry{
		"no code":  {Description: "x", Unit: "each", Price: unit.Dollars(1)},
		"no unit":  {Code: costing.CodeGuyGuard, Description: "x", Price: unit.Dollars(1)},
		"negative": {Code: costing.CodeGuyGuard, Description: "x", Unit: "each", Price: unit.Cents(-1)},
	}
	for name, e := range cases {
		if err := b.Add(on, e); err == nil {
			t.Errorf("%s should have been rejected", name)
		}
	}
	if err := b.Add(on, costing.Entry{Code: costing.CodeGuyGuard, Unit: "each", Price: 0}); err != nil {
		t.Errorf("a free item is allowed: %v", err)
	}
}

func TestBookVersionsAPrice(t *testing.T) {
	b := costing.NewBook("test")
	if got := b.Versions(costing.CodeGuyGuard); got != 0 {
		t.Errorf("versions = %d", got)
	}
	base := costing.Entry{Code: costing.CodeGuyGuard, Description: "guard", Unit: "each", Price: unit.Dollars(38)}
	if err := b.Add(calendar.MustNew(2025, 1, 1), base); err != nil {
		t.Fatal(err)
	}
	raised := base
	raised.Price = unit.Dollars(41)
	if err := b.Add(calendar.MustNew(2026, 1, 1), raised); err != nil {
		t.Fatal(err)
	}
	if got := b.Versions(costing.CodeGuyGuard); got != 2 {
		t.Errorf("versions = %d", got)
	}
	before, err := b.Price(costing.CodeGuyGuard, calendar.MustNew(2025, 12, 31))
	if err != nil {
		t.Fatal(err)
	}
	after, err := b.Price(costing.CodeGuyGuard, calendar.MustNew(2026, 1, 1))
	if err != nil {
		t.Fatal(err)
	}
	if before.AsCents() != 3800 || after.AsCents() != 4100 {
		t.Errorf("prices = %s and %s", before, after)
	}
}

func TestStandardBookIsCompleteOnBothSidesOfTheRise(t *testing.T) {
	b := costing.Standard()
	codes := b.Codes()
	if len(codes) != 16 {
		t.Fatalf("codes = %v", codes)
	}
	for i := 1; i < len(codes); i++ {
		if codes[i] < codes[i-1] {
			t.Fatalf("codes are not sorted: %v", codes)
		}
	}
	old := calendar.MustNew(2025, 6, 1)
	fresh := calendar.MustNew(2026, 6, 1)
	for _, c := range codes {
		wasEntry, err := b.Entry(c, old)
		if err != nil {
			t.Errorf("%s has no 2025 price: %v", c, err)
			continue
		}
		if wasEntry.Description == "" || wasEntry.Unit == "" {
			t.Errorf("%s is missing a field: %+v", c, wasEntry)
		}
		now, err := b.Price(c, fresh)
		if err != nil {
			t.Errorf("%s has no 2026 price: %v", c, err)
			continue
		}
		if now <= wasEntry.Price {
			t.Errorf("%s did not rise: %s to %s", c, wasEntry.Price, now)
		}
		if b.Versions(c) != 2 {
			t.Errorf("%s carries %d versions", c, b.Versions(c))
		}
	}
}

func TestStandardBookHasNoPriceBeforeItStarts(t *testing.T) {
	b := costing.Standard()
	if _, err := b.Price(costing.CodeMobilisation, calendar.MustNew(2024, 12, 31)); err == nil {
		t.Error("the book does not reach back before 2025")
	}
	got, err := b.Price(costing.CodeMobilisation, calendar.MustNew(2025, 1, 1))
	if err != nil {
		t.Fatal(err)
	}
	if got.AsCents() != 47500 {
		t.Errorf("mobilisation = %s", got)
	}
}
