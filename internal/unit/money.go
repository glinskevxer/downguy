package unit

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Money is a whole number of cents. Make ready estimates and rental invoices
// are added up thousands of times across a build, and a float would drift, so
// every amount in this system is an integer and every division states how it
// rounds.
type Money int64

// ErrBadMoney is returned when an amount cannot be read.
var ErrBadMoney = errors.New("unit: cannot read money")

// Dollars builds Money from whole dollars.
func Dollars(d int) Money { return Money(d) * 100 }

// Cents builds Money from whole cents.
func Cents(c int64) Money { return Money(c) }

// AsCents returns the raw cents.
func (m Money) AsCents() int64 { return int64(m) }

// Float returns the amount in dollars. For display and for ratios only.
func (m Money) Float() float64 { return float64(m) / 100 }

// Add returns m plus o.
func (m Money) Add(o Money) Money { return m + o }

// Sub returns m minus o.
func (m Money) Sub(o Money) Money { return m - o }

// Times multiplies the amount by a whole count.
func (m Money) Times(n int) Money { return m * Money(n) }

// Neg returns the amount with its sign flipped.
func (m Money) Neg() Money { return -m }

// IsZero reports whether the amount is exactly nothing.
func (m Money) IsZero() bool { return m == 0 }

// Abs returns the magnitude of the amount.
func (m Money) Abs() Money {
	if m < 0 {
		return -m
	}
	return m
}

// String prints the amount with a currency sign and two decimal places.
func (m Money) String() string {
	sign := ""
	v := int64(m)
	if v < 0 {
		sign = "-"
		v = -v
	}
	return fmt.Sprintf("%s$%d.%02d", sign, v/100, v%100)
}

// ParseMoney reads an amount written with or without a dollar sign, with or
// without thousands separators, and with zero, one or two decimal places.
func ParseMoney(text string) (Money, error) {
	t := strings.TrimSpace(text)
	if t == "" {
		return 0, fmt.Errorf("%w: empty", ErrBadMoney)
	}
	neg := false
	if strings.HasPrefix(t, "-") {
		neg = true
		t = t[1:]
	}
	t = strings.TrimPrefix(strings.TrimSpace(t), "$")
	t = strings.ReplaceAll(strings.TrimSpace(t), ",", "")
	if t == "" {
		return 0, fmt.Errorf("%w: %q", ErrBadMoney, text)
	}
	whole := t
	frac := ""
	if dot := strings.Index(t, "."); dot >= 0 {
		whole = t[:dot]
		frac = t[dot+1:]
	}
	if whole == "" {
		whole = "0"
	}
	if !allDigits(whole) {
		return 0, fmt.Errorf("%w: %q", ErrBadMoney, text)
	}
	w, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %q", ErrBadMoney, text)
	}
	cents := int64(0)
	if frac != "" {
		if len(frac) > 2 || !allDigits(frac) {
			return 0, fmt.Errorf("%w: %q has more than cents", ErrBadMoney, text)
		}
		if len(frac) == 1 {
			frac += "0"
		}
		cents, err = strconv.ParseInt(frac, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("%w: %q", ErrBadMoney, text)
		}
	}
	total := Money(w*100 + cents)
	if neg {
		total = -total
	}
	return total, nil
}

// SumMoney adds a list of amounts.
func SumMoney(amounts []Money) Money {
	var total Money
	for _, a := range amounts {
		total += a
	}
	return total
}

// BasisPoints is a rate held in hundredths of a percent. Carrying charge rates
// and cost allocation shares are quoted to two decimal places of a percent and
// nothing finer, so this holds them exactly.
type BasisPoints int32

// FullRate is one hundred percent expressed in basis points.
const FullRate BasisPoints = 10000

// Percent builds BasisPoints from whole percent.
func Percent(p int) BasisPoints { return BasisPoints(p) * 100 }

// ParsePercent reads a rate written as a percent, with up to two decimals and
// an optional trailing sign.
func ParsePercent(text string) (BasisPoints, error) {
	t := strings.TrimSuffix(strings.TrimSpace(text), "%")
	t = strings.TrimSpace(t)
	if t == "" {
		return 0, fmt.Errorf("%w: empty percent", ErrBadMoney)
	}
	neg := false
	if strings.HasPrefix(t, "-") {
		neg = true
		t = t[1:]
	}
	whole := t
	frac := ""
	if dot := strings.Index(t, "."); dot >= 0 {
		whole = t[:dot]
		frac = t[dot+1:]
	}
	if whole == "" {
		whole = "0"
	}
	if !allDigits(whole) {
		return 0, fmt.Errorf("%w: %q", ErrBadMoney, text)
	}
	w, err := strconv.Atoi(whole)
	if err != nil {
		return 0, fmt.Errorf("%w: %q", ErrBadMoney, text)
	}
	hundredths := 0
	if frac != "" {
		if len(frac) > 2 || !allDigits(frac) {
			return 0, fmt.Errorf("%w: %q is finer than a hundredth of a percent", ErrBadMoney, text)
		}
		if len(frac) == 1 {
			frac += "0"
		}
		hundredths, err = strconv.Atoi(frac)
		if err != nil {
			return 0, fmt.Errorf("%w: %q", ErrBadMoney, text)
		}
	}
	bp := BasisPoints(w*100 + hundredths)
	if neg {
		bp = -bp
	}
	return bp, nil
}

// String prints the rate as a percent with two decimals.
func (b BasisPoints) String() string {
	sign := ""
	v := int32(b)
	if v < 0 {
		sign = "-"
		v = -v
	}
	return fmt.Sprintf("%s%d.%02d%%", sign, v/100, v%100)
}

// Float returns the rate as a fraction of one.
func (b BasisPoints) Float() float64 { return float64(b) / 10000 }

// Of applies the rate to an amount, rounding half away from zero so that a
// share of a dollar never quietly disappears.
func (b BasisPoints) Of(m Money) Money {
	num := int64(m) * int64(b)
	if num >= 0 {
		return Money((num + 5000) / 10000)
	}
	return Money((num - 5000) / 10000)
}
