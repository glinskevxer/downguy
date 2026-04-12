package rental

import (
	"fmt"
	"sort"
	"strings"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/unit"
)

// A billing run and the plant records drift apart. An attachment goes up in
// March and reaches the inventory in September; one comes down in June and
// nobody tells the billing side at all. The true up is the annual comparison
// between what was billed and what the records now say, and it produces a
// credit or a further charge rather than an argument.

// Variance is one attacher's difference between billed and billable.
type Variance struct {
	Owner     string
	Billed    unit.Money
	Billable  unit.Money
	Delta     unit.Money
	BilledFor int
	NowOn     int
	Added     []string
	Removed   []string
}

// Owes reports whether the attacher owes more than it was billed.
func (v Variance) Owes() bool { return v.Delta > 0 }

// Credit reports whether the attacher is owed a credit.
func (v Variance) Credit() bool { return v.Delta < 0 }

// String prints the variance the way a true up statement lists it.
func (v Variance) String() string {
	direction := "in balance"
	switch {
	case v.Owes():
		direction = "owes " + v.Delta.String()
	case v.Credit():
		direction = "is owed " + v.Delta.Abs().String()
	}
	return fmt.Sprintf("%s: billed %s against %s billable, %s", v.Owner, v.Billed, v.Billable, direction)
}

// TrueUp is the whole comparison for a rate year.
type TrueUp struct {
	Year      calendar.RateYear
	Variances []Variance
	Billed    unit.Money
	Billable  unit.Money
	Delta     unit.Money
}

// Reconcile compares a set of bills already raised against the inventory as it
// now stands, and reports the difference per attacher. An attacher that was
// billed and is no longer on the system still appears, with everything it was
// billed as a credit, because that is exactly the case somebody has to see.
func Reconcile(year calendar.RateYear, billed []Bill, inv *Inventory, assign Assignment) (*TrueUp, error) {
	if inv == nil {
		return nil, fmt.Errorf("%w: no inventory to reconcile against", ErrBadRate)
	}
	if inv.Year != year {
		return nil, fmt.Errorf("%w: reconciling %s against a %s inventory", ErrBadRate, year, inv.Year)
	}
	fresh, err := Bills(inv, assign, calendar.MustNew(int(year)+1, 1, 1), "TU")
	if err != nil {
		return nil, err
	}

	billedBy := map[string]Bill{}
	for _, b := range billed {
		if b.Year != year {
			continue
		}
		billedBy[foldOwner(b.Owner)] = b
	}
	billableBy := map[string]Bill{}
	for _, b := range fresh {
		billableBy[foldOwner(b.Owner)] = b
	}

	names := map[string]string{}
	for key, b := range billedBy {
		names[key] = b.Owner
	}
	for key, b := range billableBy {
		if _, seen := names[key]; !seen {
			names[key] = b.Owner
		}
	}
	keys := make([]string, 0, len(names))
	for key := range names {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	out := &TrueUp{Year: year}
	for _, key := range keys {
		was, hadBill := billedBy[key]
		now, hasBill := billableBy[key]
		v := Variance{Owner: names[key]}
		if hadBill {
			v.Billed = was.Total
			v.BilledFor = len(was.Lines)
		}
		if hasBill {
			v.Billable = now.Total
			v.NowOn = len(now.Lines)
		}
		v.Delta = v.Billable.Sub(v.Billed)
		v.Added, v.Removed = differ(was, now)
		out.Variances = append(out.Variances, v)
		out.Billed = out.Billed.Add(v.Billed)
		out.Billable = out.Billable.Add(v.Billable)
	}
	out.Delta = out.Billable.Sub(out.Billed)
	return out, nil
}

// differ returns the attachments on the new bill that were not on the old one,
// and the ones on the old bill that are gone.
func differ(was, now Bill) ([]string, []string) {
	before := map[string]bool{}
	for _, l := range was.Lines {
		before[l.AttachmentID] = true
	}
	after := map[string]bool{}
	for _, l := range now.Lines {
		after[l.AttachmentID] = true
	}
	var added, removed []string
	for id := range after {
		if !before[id] {
			added = append(added, id)
		}
	}
	for id := range before {
		if !after[id] {
			removed = append(removed, id)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	return added, removed
}

func foldOwner(name string) string {
	return strings.ToUpper(strings.Join(strings.Fields(name), " "))
}

// Balanced reports whether nothing moved.
func (t *TrueUp) Balanced() bool { return t.Delta.IsZero() && len(t.Moved()) == 0 }

// Moved returns the attachers whose position changed at all.
func (t *TrueUp) Moved() []Variance {
	var out []Variance
	for _, v := range t.Variances {
		if !v.Delta.IsZero() || len(v.Added) > 0 || len(v.Removed) > 0 {
			out = append(out, v)
		}
	}
	return out
}

// Owing returns the attachers that owe more.
func (t *TrueUp) Owing() []Variance {
	var out []Variance
	for _, v := range t.Variances {
		if v.Owes() {
			out = append(out, v)
		}
	}
	return out
}

// Credits returns the attachers owed a credit.
func (t *TrueUp) Credits() []Variance {
	var out []Variance
	for _, v := range t.Variances {
		if v.Credit() {
			out = append(out, v)
		}
	}
	return out
}

// Summary writes the line a true up statement opens with.
func (t *TrueUp) Summary() string {
	return fmt.Sprintf("%s true up: %s billed against %s billable, %s to settle across %d attachers",
		t.Year, t.Billed, t.Billable, t.Delta, len(t.Moved()))
}
