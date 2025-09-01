package rental

import (
	"fmt"
	"sort"
	"strings"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/unit"
)

// BillLine is one attachment on an annual rental bill.
type BillLine struct {
	AttachmentID string
	Pole         plant.PoleID
	Formula      Formula
	Attachers    int
	FullRate     unit.Money
	Days         int
	OfDays       int
	Amount       unit.Money
	Note         string
}

// FullYear reports whether the line was up for the whole rate year.
func (l BillLine) FullYear() bool { return l.Days >= l.OfDays }

// Bill is one attacher's rental for one rate year.
type Bill struct {
	Number  string
	Year    calendar.RateYear
	Owner   string
	Formula Formula
	Lines   []BillLine
	Total   unit.Money
	Issued  calendar.Date
	Due     calendar.Date
}

// BillingTermsDays is how long an attacher has to pay an annual rental bill.
const BillingTermsDays = 45

// Assignment says which formula each attacher is billed under. An attacher
// that is not in the map is billed under the cable formula, which is the
// lower of the two and the one this utility applies until somebody proves
// otherwise.
type Assignment map[string]Formula

// FormulaFor returns the formula an attacher is billed under.
func (a Assignment) FormulaFor(owner string) Formula {
	if f, ok := a[strings.ToUpper(strings.TrimSpace(owner))]; ok {
		return f
	}
	return FormulaCable
}

// Bills works out one bill per attacher from an inventory. The attacher count
// on each pole comes from the inventory rather than from a system wide
// average, because the telecom formula divides the unusable space by it and a
// pole with two attachers is a materially different rate from one with six.
func Bills(inv *Inventory, assign Assignment, issued calendar.Date, prefix string) ([]Bill, error) {
	if inv == nil {
		return nil, fmt.Errorf("%w: no inventory", ErrBadRate)
	}
	if issued.IsZero() {
		return nil, fmt.Errorf("%w: a bill needs an issue date", ErrBadRate)
	}
	if strings.TrimSpace(prefix) == "" {
		return nil, fmt.Errorf("%w: a bill needs a number prefix", ErrBadRate)
	}
	space := StandardSpace()
	owners := inv.Owners()
	out := make([]Bill, 0, len(owners))

	for i, owner := range owners {
		formula := assign.FormulaFor(owner)
		bill := Bill{
			Number: fmt.Sprintf("%s-%03d", prefix, i+1),
			Year:   inv.Year, Owner: owner, Formula: formula,
			Issued: issued, Due: issued.AddDays(BillingTermsDays),
		}
		for _, o := range inv.ForOwner(owner) {
			attachers := inv.AttachersOn(o.Pole)
			rate, err := Compute(inv.Year, formula, space, StandardOccupied, attachers)
			if err != nil {
				return nil, fmt.Errorf("%s on %s: %w", o.AttachmentID, o.Pole, err)
			}
			days, ofDays := calendar.ProrationOf(inv.Year, o.Active)
			line := BillLine{
				AttachmentID: o.AttachmentID, Pole: o.Pole, Formula: formula,
				Attachers: attachers, FullRate: rate.Annual,
				Days: days, OfDays: ofDays, Amount: rate.Prorated(o.Active),
			}
			if !o.Authorized {
				line.Note = "unauthorized attachment, billed pending resolution"
			}
			bill.Lines = append(bill.Lines, line)
			bill.Total = bill.Total.Add(line.Amount)
		}
		sort.SliceStable(bill.Lines, func(a, b int) bool {
			if bill.Lines[a].Pole != bill.Lines[b].Pole {
				return bill.Lines[a].Pole < bill.Lines[b].Pole
			}
			return bill.Lines[a].AttachmentID < bill.Lines[b].AttachmentID
		})
		out = append(out, bill)
	}
	return out, nil
}

// Validate checks a bill before it goes out.
func (b Bill) Validate() error {
	if strings.TrimSpace(b.Number) == "" {
		return fmt.Errorf("%w: a bill with no number", ErrBadRate)
	}
	if strings.TrimSpace(b.Owner) == "" {
		return fmt.Errorf("%w: %s names no attacher", ErrBadRate, b.Number)
	}
	if !KnownFormula(b.Formula) {
		return fmt.Errorf("%w: %s is billed under %q", ErrBadRate, b.Number, b.Formula)
	}
	if b.Issued.IsZero() || b.Due.Before(b.Issued) {
		return fmt.Errorf("%w: %s is dated %s to %s", ErrBadRate, b.Number, b.Issued, b.Due)
	}
	if len(b.Lines) == 0 {
		return fmt.Errorf("%w: %s has no lines", ErrBadRate, b.Number)
	}
	var total unit.Money
	for _, l := range b.Lines {
		if l.Amount < 0 {
			return fmt.Errorf("%w: %s bills %s for %s", ErrBadRate, b.Number, l.AttachmentID, l.Amount)
		}
		total = total.Add(l.Amount)
	}
	if total != b.Total {
		return fmt.Errorf("%w: %s totals %s but its lines add to %s", ErrBadRate, b.Number, b.Total, total)
	}
	return nil
}

// Poles returns every pole on the bill, sorted.
func (b Bill) Poles() []plant.PoleID {
	seen := map[plant.PoleID]bool{}
	var out []plant.PoleID
	for _, l := range b.Lines {
		if seen[l.Pole] {
			continue
		}
		seen[l.Pole] = true
		out = append(out, l.Pole)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Prorated returns the lines that were not up for the whole year.
func (b Bill) Prorated() []BillLine {
	var out []BillLine
	for _, l := range b.Lines {
		if !l.FullYear() {
			out = append(out, l)
		}
	}
	return out
}

// Flagged returns the lines carrying a note, which is how an unauthorised
// attachment reaches the person who has to chase it.
func (b Bill) Flagged() []BillLine {
	var out []BillLine
	for _, l := range b.Lines {
		if l.Note != "" {
			out = append(out, l)
		}
	}
	return out
}

// String prints the bill header.
func (b Bill) String() string {
	return fmt.Sprintf("%s to %s for %s: %d attachments, %s due %s",
		b.Number, b.Owner, b.Year, len(b.Lines), b.Total, b.Due)
}

// BillTotal adds up a run of bills.
func BillTotal(bills []Bill) unit.Money {
	var total unit.Money
	for _, b := range bills {
		total = total.Add(b.Total)
	}
	return total
}
