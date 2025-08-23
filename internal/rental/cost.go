package rental

import (
	"errors"
	"fmt"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/unit"
)

// ErrBadCost is returned when the cost side of the rate cannot be worked out.
var ErrBadCost = errors.New("rental: cannot compute pole cost")

// PlantAccount is the utility's books for its distribution poles, as filed.
// Every figure here comes off a regulatory report rather than out of this
// system, which is why they are inputs rather than something computed.
type PlantAccount struct {
	Year int
	// GrossPoles is the original cost of the poles in service.
	GrossPoles unit.Money
	// AccumulatedDepreciation is the reserve against them.
	AccumulatedDepreciation unit.Money
	// AccumulatedDeferredTax is the deferred income tax reserve.
	AccumulatedDeferredTax unit.Money
	// PoleCount is how many poles that money bought.
	PoleCount int
	// AppurtenanceShare is the part of the pole account that is crossarms and
	// hardware rather than the pole itself. The rate is charged on a bare
	// pole, so this comes out.
	AppurtenanceShare unit.BasisPoints
}

// Validate checks a plant account can produce a net bare pole cost.
func (a PlantAccount) Validate() error {
	switch {
	case a.Year < 1900:
		return fmt.Errorf("%w: filed for year %d", ErrBadCost, a.Year)
	case a.GrossPoles <= 0:
		return fmt.Errorf("%w: gross pole plant is %s", ErrBadCost, a.GrossPoles)
	case a.PoleCount <= 0:
		return fmt.Errorf("%w: %d poles", ErrBadCost, a.PoleCount)
	case a.AccumulatedDepreciation < 0 || a.AccumulatedDeferredTax < 0:
		return fmt.Errorf("%w: a negative reserve", ErrBadCost)
	case a.AppurtenanceShare < 0 || a.AppurtenanceShare >= unit.FullRate:
		return fmt.Errorf("%w: appurtenances are %s of the account", ErrBadCost, a.AppurtenanceShare)
	}
	if a.AccumulatedDepreciation.Add(a.AccumulatedDeferredTax) >= a.GrossPoles {
		return fmt.Errorf("%w: the reserves exceed the gross plant", ErrBadCost)
	}
	return nil
}

// NetPolePlant returns the gross pole account less its reserves.
func (a PlantAccount) NetPolePlant() (unit.Money, error) {
	if err := a.Validate(); err != nil {
		return 0, err
	}
	return a.GrossPoles.Sub(a.AccumulatedDepreciation).Sub(a.AccumulatedDeferredTax), nil
}

// NetBarePoleCost returns the average net cost of one bare pole: the net pole
// plant with the appurtenance share taken out, divided by the pole count.
func (a PlantAccount) NetBarePoleCost() (unit.Money, error) {
	net, err := a.NetPolePlant()
	if err != nil {
		return 0, err
	}
	bare := net.Sub(a.AppurtenanceShare.Of(net))
	return unit.Cents(bare.AsCents() / int64(a.PoleCount)), nil
}

// CarryingCharge is the annual cost of owning a pole, expressed as a rate on
// the net investment. Each component is separate because attachers challenge
// them one at a time.
type CarryingCharge struct {
	Depreciation   unit.BasisPoints
	ReturnOnRate   unit.BasisPoints
	Taxes          unit.BasisPoints
	Maintenance    unit.BasisPoints
	Administrative unit.BasisPoints
}

// Total returns the whole carrying charge rate.
func (c CarryingCharge) Total() unit.BasisPoints {
	return c.Depreciation + c.ReturnOnRate + c.Taxes + c.Maintenance + c.Administrative
}

// Validate checks every component and the total.
func (c CarryingCharge) Validate() error {
	components := map[string]unit.BasisPoints{
		"depreciation":   c.Depreciation,
		"return":         c.ReturnOnRate,
		"taxes":          c.Taxes,
		"maintenance":    c.Maintenance,
		"administrative": c.Administrative,
	}
	for name, v := range components {
		if v < 0 {
			return fmt.Errorf("%w: the %s component is %s", ErrBadCost, name, v)
		}
	}
	total := c.Total()
	if total <= 0 {
		return fmt.Errorf("%w: the carrying charge totals %s", ErrBadCost, total)
	}
	if total >= unit.FullRate {
		return fmt.Errorf("%w: a carrying charge of %s would retire the pole every year", ErrBadCost, total)
	}
	return nil
}

// Components returns the parts and their rates, for a rate filing that has to
// show its working.
func (c CarryingCharge) Components() map[string]unit.BasisPoints {
	return map[string]unit.BasisPoints{
		"depreciation":   c.Depreciation,
		"return":         c.ReturnOnRate,
		"taxes":          c.Taxes,
		"maintenance":    c.Maintenance,
		"administrative": c.Administrative,
	}
}

// AnnualCost returns what one pole costs the owner in a year.
func (c CarryingCharge) AnnualCost(netBarePole unit.Money) (unit.Money, error) {
	if err := c.Validate(); err != nil {
		return 0, err
	}
	if netBarePole <= 0 {
		return 0, fmt.Errorf("%w: a bare pole costing %s", ErrBadCost, netBarePole)
	}
	return c.Total().Of(netBarePole), nil
}

// FiledAccount returns the account this utility filed for a year, and reports
// whether it filed one at all.
func FiledAccount(year int) (PlantAccount, bool) {
	a, ok := filedAccounts[year]
	return a, ok
}

var filedAccounts = map[int]PlantAccount{
	2024: {
		Year: 2024, GrossPoles: unit.Dollars(41_820_000),
		AccumulatedDepreciation: unit.Dollars(16_100_000),
		AccumulatedDeferredTax:  unit.Dollars(2_940_000),
		PoleCount:               58_400, AppurtenanceShare: unit.BasisPoints(1500),
	},
	2025: {
		Year: 2025, GrossPoles: unit.Dollars(43_610_000),
		AccumulatedDepreciation: unit.Dollars(16_880_000),
		AccumulatedDeferredTax:  unit.Dollars(3_070_000),
		PoleCount:               59_150, AppurtenanceShare: unit.BasisPoints(1500),
	},
	2026: {
		Year: 2026, GrossPoles: unit.Dollars(45_290_000),
		AccumulatedDepreciation: unit.Dollars(17_640_000),
		AccumulatedDeferredTax:  unit.Dollars(3_210_000),
		PoleCount:               59_880, AppurtenanceShare: unit.BasisPoints(1500),
	},
}

// FiledCarryingCharge returns the carrying charge this utility filed for a
// year, and reports whether it filed one.
func FiledCarryingCharge(year int) (CarryingCharge, bool) {
	c, ok := filedCharges[year]
	return c, ok
}

var filedCharges = map[int]CarryingCharge{
	2024: {Depreciation: 380, ReturnOnRate: 745, Taxes: 195, Maintenance: 310, Administrative: 160},
	2025: {Depreciation: 385, ReturnOnRate: 762, Taxes: 198, Maintenance: 318, Administrative: 164},
	2026: {Depreciation: 391, ReturnOnRate: 780, Taxes: 203, Maintenance: 327, Administrative: 169},
}

// FiledYears returns the years this utility has filed, oldest first.
func FiledYears() []int {
	out := make([]int, 0, len(filedAccounts))
	for y := range filedAccounts {
		out = append(out, y)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// AnnualPoleCost returns what a pole cost the owner in a rate year, from the
// filings for that year.
func AnnualPoleCost(year calendar.RateYear) (unit.Money, error) {
	account, ok := FiledAccount(int(year))
	if !ok {
		return 0, fmt.Errorf("%w: no plant account filed for %s", ErrBadCost, year)
	}
	charge, ok := FiledCarryingCharge(int(year))
	if !ok {
		return 0, fmt.Errorf("%w: no carrying charge filed for %s", ErrBadCost, year)
	}
	bare, err := account.NetBarePoleCost()
	if err != nil {
		return 0, err
	}
	return charge.AnnualCost(bare)
}
