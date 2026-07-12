package unit

import "strconv"

// Four packages grew their own copy of "print this number without a unit on
// it": the csv exporter, the json renderer, the command line tables and the
// report writer. They agreed, until one of them was changed. These are the
// one copy.

// Whole prints an integer with no separators and no padding, which is what a
// csv column and a table cell both want.
func Whole(n int) string { return strconv.Itoa(n) }

// Decimal prints a height as decimal feet with one place and no unit. The unit
// belongs to the column heading or the field name, not to every cell.
func Decimal(h Height) string {
	sign := ""
	value := int(h)
	if value < 0 {
		sign = "-"
		value = -value
	}
	return sign + strconv.Itoa(value/TenthsPerFoot) + "." + strconv.Itoa(value%TenthsPerFoot)
}

// Plain prints an amount as a decimal with two places, no currency sign and no
// thousands separator, because a spreadsheet reads a dollar sign as text and a
// comma as a column break.
func Plain(m Money) string {
	sign := ""
	cents := int64(m)
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	whole := strconv.FormatInt(cents/100, 10)
	part := strconv.FormatInt(cents%100, 10)
	if len(part) == 1 {
		part = "0" + part
	}
	return sign + whole + "." + part
}

// RoundToWhole takes a floating point figure to the nearest whole unit, away
// from zero at the half. Moments are worked out in floating point and printing
// fourteen significant figures of one invites an argument about the
// fourteenth.
func RoundToWhole(v float64) float64 {
	if v < 0 {
		return -float64(int64(-v + 0.5))
	}
	return float64(int64(v + 0.5))
}
