package tests

import (
	"testing"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/rental"
	"github.com/glinskevxer/downguy/internal/unit"
)

func TestFiledYearsAreOrderedAndComplete(t *testing.T) {
	years := rental.FiledYears()
	if len(years) != 3 {
		t.Fatalf("years = %v", years)
	}
	for i := 1; i < len(years); i++ {
		if years[i] <= years[i-1] {
			t.Fatalf("years are not ordered: %v", years)
		}
	}
	for _, y := range years {
		a, ok := rental.FiledAccount(y)
		if !ok {
			t.Errorf("no account for %d", y)
			continue
		}
		if err := a.Validate(); err != nil {
			t.Errorf("%d account: %v", y, err)
		}
		c, ok := rental.FiledCarryingCharge(y)
		if !ok {
			t.Errorf("no carrying charge for %d", y)
			continue
		}
		if err := c.Validate(); err != nil {
			t.Errorf("%d carrying charge: %v", y, err)
		}
	}
	if _, ok := rental.FiledAccount(1999); ok {
		t.Error("nothing was filed for 1999")
	}
	if _, ok := rental.FiledCarryingCharge(1999); ok {
		t.Error("nothing was filed for 1999")
	}
}

func TestPlantAccountValidate(t *testing.T) {
	good, _ := rental.FiledAccount(2025)
	if err := good.Validate(); err != nil {
		t.Fatalf("a filed account failed: %v", err)
	}
	cases := map[string]func(*rental.PlantAccount){
		"no year":     func(a *rental.PlantAccount) { a.Year = 0 },
		"no plant":    func(a *rental.PlantAccount) { a.GrossPoles = 0 },
		"no poles":    func(a *rental.PlantAccount) { a.PoleCount = 0 },
		"neg reserve": func(a *rental.PlantAccount) { a.AccumulatedDepreciation = unit.Cents(-1) },
		"neg tax":     func(a *rental.PlantAccount) { a.AccumulatedDeferredTax = unit.Cents(-1) },
		"bad share":   func(a *rental.PlantAccount) { a.AppurtenanceShare = unit.FullRate },
		"neg share":   func(a *rental.PlantAccount) { a.AppurtenanceShare = -1 },
		"over depr":   func(a *rental.PlantAccount) { a.AccumulatedDepreciation = a.GrossPoles },
	}
	for name, mutate := range cases {
		a := good
		mutate(&a)
		if err := a.Validate(); err == nil {
			t.Errorf("%s should have been rejected", name)
		}
	}
}

func TestNetPolePlantAndBareCost(t *testing.T) {
	a, _ := rental.FiledAccount(2025)
	net, err := a.NetPolePlant()
	if err != nil {
		t.Fatal(err)
	}
	want := a.GrossPoles.Sub(a.AccumulatedDepreciation).Sub(a.AccumulatedDeferredTax)
	if net != want {
		t.Errorf("net plant = %s, want %s", net, want)
	}
	bare, err := a.NetBarePoleCost()
	if err != nil {
		t.Fatal(err)
	}
	if bare <= 0 {
		t.Errorf("bare pole cost = %s", bare)
	}
	if bare.Times(a.PoleCount) >= net {
		t.Error("the appurtenance share should come out before the division")
	}
	// A distribution pole nets out somewhere in the hundreds of dollars.
	if bare.AsCents() < 10000 || bare.AsCents() > 100000 {
		t.Errorf("bare pole cost looks wrong: %s", bare)
	}
	bad := a
	bad.PoleCount = 0
	if _, err := bad.NetBarePoleCost(); err == nil {
		t.Error("an invalid account should be rejected")
	}
	if _, err := bad.NetPolePlant(); err == nil {
		t.Error("an invalid account should be rejected")
	}
}

func TestCarryingChargeTotalsItsComponents(t *testing.T) {
	c, _ := rental.FiledCarryingCharge(2025)
	parts := c.Components()
	if len(parts) != 5 {
		t.Fatalf("components = %v", parts)
	}
	var total unit.BasisPoints
	for _, v := range parts {
		total += v
	}
	if total != c.Total() {
		t.Errorf("the components add to %s but the total is %s", total, c.Total())
	}
	// A filed carrying charge sits in the teens of percent.
	if c.Total() < unit.Percent(10) || c.Total() > unit.Percent(30) {
		t.Errorf("carrying charge = %s", c.Total())
	}
}

func TestCarryingChargeValidate(t *testing.T) {
	good, _ := rental.FiledCarryingCharge(2025)
	if err := good.Validate(); err != nil {
		t.Fatalf("a filed charge failed: %v", err)
	}
	negative := good
	negative.Taxes = -1
	if err := negative.Validate(); err == nil {
		t.Error("a negative component should be rejected")
	}
	empty := rental.CarryingCharge{}
	if err := empty.Validate(); err == nil {
		t.Error("a charge of nothing should be rejected")
	}
	absurd := good
	absurd.ReturnOnRate = unit.FullRate
	if err := absurd.Validate(); err == nil {
		t.Error("a charge over a hundred percent should be rejected")
	}
}

func TestAnnualCostOfAPole(t *testing.T) {
	a, _ := rental.FiledAccount(2025)
	c, _ := rental.FiledCarryingCharge(2025)
	bare, err := a.NetBarePoleCost()
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.AnnualCost(bare)
	if err != nil {
		t.Fatal(err)
	}
	if got != c.Total().Of(bare) {
		t.Errorf("annual cost = %s", got)
	}
	if got >= bare {
		t.Error("a year of carrying charge is less than the pole")
	}
	if _, err := c.AnnualCost(0); err == nil {
		t.Error("a free pole should be rejected")
	}
	var empty rental.CarryingCharge
	if _, err := empty.AnnualCost(bare); err == nil {
		t.Error("an invalid charge should be rejected")
	}
}

func TestAnnualPoleCostRisesWithTheFilings(t *testing.T) {
	var last unit.Money
	for _, y := range rental.FiledYears() {
		got, err := rental.AnnualPoleCost(calendar.RateYear(y))
		if err != nil {
			t.Fatalf("%d: %v", y, err)
		}
		if got <= last {
			t.Errorf("%d cost %s, not above the previous %s", y, got, last)
		}
		last = got
	}
	if _, err := rental.AnnualPoleCost(calendar.RateYear(1999)); err == nil {
		t.Error("nothing was filed for 1999")
	}
}
