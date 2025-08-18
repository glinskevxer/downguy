package allocate

import (
	"fmt"
	"sort"
	"strings"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/unit"
)

// InvoiceLine is one line of a make ready invoice.
type InvoiceLine struct {
	ItemID string
	Pole   string
	Detail string
	Amount unit.Money
}

// Invoice is what one party is billed for one application.
type Invoice struct {
	Number      string
	Application string
	Party       string
	Account     string
	Role        Role
	Issued      calendar.Date
	Due         calendar.Date
	Lines       []InvoiceLine
	Total       unit.Money
}

// TermsDays is how long a party has to pay a make ready invoice.
const TermsDays = 30

// Invoices turns a split into one invoice per party. A party with no charges
// gets no invoice at all rather than a zero one, because a zero invoice is a
// letter somebody has to read and then ignore.
func Invoices(s *Split, roster *Roster, issued calendar.Date, prefix string) ([]Invoice, error) {
	if s == nil {
		return nil, fmt.Errorf("%w: no split to invoice", ErrBadParty)
	}
	if roster == nil {
		return nil, fmt.Errorf("%w: no roster", ErrBadParty)
	}
	if issued.IsZero() {
		return nil, fmt.Errorf("%w: an invoice needs an issue date", ErrBadParty)
	}
	if strings.TrimSpace(prefix) == "" {
		return nil, fmt.Errorf("%w: an invoice needs a number prefix", ErrBadParty)
	}

	byParty := map[string][]InvoiceLine{}
	for _, c := range s.Charges {
		byParty[c.Party] = append(byParty[c.Party], InvoiceLine{
			ItemID: c.ItemID, Pole: c.Pole, Detail: c.Because, Amount: c.Amount,
		})
	}

	names := make([]string, 0, len(byParty))
	for name := range byParty {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]Invoice, 0, len(names))
	for i, name := range names {
		lines := byParty[name]
		sort.SliceStable(lines, func(a, b int) bool { return lines[a].ItemID < lines[b].ItemID })
		inv := Invoice{
			Number:      fmt.Sprintf("%s-%03d", prefix, i+1),
			Application: s.Application,
			Party:       name,
			Issued:      issued,
			Due:         issued.AddDays(TermsDays),
			Lines:       lines,
		}
		if p, ok := roster.Lookup(name); ok {
			inv.Account = p.Account
			inv.Role = p.Role
		} else {
			inv.Role = RoleUnknown
		}
		for _, l := range lines {
			inv.Total = inv.Total.Add(l.Amount)
		}
		out = append(out, inv)
	}
	return out, nil
}

// Overdue reports whether the invoice is past its due date and unpaid.
func (i Invoice) Overdue(on calendar.Date, paid bool) bool {
	if paid || i.Due.IsZero() {
		return false
	}
	return on.After(i.Due)
}

// DaysLate returns how far past due the invoice is, or zero when it is not.
func (i Invoice) DaysLate(on calendar.Date) int {
	if i.Due.IsZero() || !on.After(i.Due) {
		return 0
	}
	return i.Due.DaysUntil(on)
}

// Validate checks an invoice before it goes out.
func (i Invoice) Validate() error {
	if strings.TrimSpace(i.Number) == "" {
		return fmt.Errorf("%w: an invoice with no number", ErrBadParty)
	}
	if strings.TrimSpace(i.Party) == "" {
		return fmt.Errorf("%w: %s names no party", ErrBadParty, i.Number)
	}
	if i.Issued.IsZero() || i.Due.IsZero() {
		return fmt.Errorf("%w: %s is not dated", ErrBadParty, i.Number)
	}
	if i.Due.Before(i.Issued) {
		return fmt.Errorf("%w: %s falls due before it is issued", ErrBadParty, i.Number)
	}
	if len(i.Lines) == 0 {
		return fmt.Errorf("%w: %s has no lines", ErrBadParty, i.Number)
	}
	var total unit.Money
	for _, l := range i.Lines {
		total = total.Add(l.Amount)
	}
	if total != i.Total {
		return fmt.Errorf("%w: %s totals %s but its lines add to %s", ErrBadParty, i.Number, i.Total, total)
	}
	return nil
}

// Poles returns every pole billed on the invoice, sorted.
func (i Invoice) Poles() []string {
	seen := map[string]bool{}
	var out []string
	for _, l := range i.Lines {
		if l.Pole == "" || seen[l.Pole] {
			continue
		}
		seen[l.Pole] = true
		out = append(out, l.Pole)
	}
	sort.Strings(out)
	return out
}

// String prints the invoice header.
func (i Invoice) String() string {
	return fmt.Sprintf("%s to %s for %s: %s due %s", i.Number, i.Party, i.Application, i.Total, i.Due)
}

// TotalOf adds up a set of invoices.
func TotalOf(invoices []Invoice) unit.Money {
	var total unit.Money
	for _, i := range invoices {
		total = total.Add(i.Total)
	}
	return total
}
