package rental

import (
	"fmt"
	"sort"
	"strings"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/plant"
)

// Occupancy is one attachment as the billing system sees it: whose it is,
// which pole it is on, and the part of the rate year it was there for.
type Occupancy struct {
	AttachmentID string
	Pole         plant.PoleID
	Owner        string
	Authorized   bool
	Active       calendar.Range
}

// Inventory is every billable attachment for one rate year.
type Inventory struct {
	Year        calendar.RateYear
	Occupancies []Occupancy
}

// NewInventory builds an inventory for a year.
func NewInventory(year calendar.RateYear) *Inventory {
	return &Inventory{Year: year}
}

// Add files an occupancy, keeping only what overlaps the rate year at all.
func (i *Inventory) Add(o Occupancy) error {
	if strings.TrimSpace(o.AttachmentID) == "" {
		return fmt.Errorf("%w: an occupancy with no attachment", ErrBadRate)
	}
	if strings.TrimSpace(o.Owner) == "" {
		return fmt.Errorf("%w: %s has no owner", ErrBadRate, o.AttachmentID)
	}
	if !o.Active.Valid() {
		return fmt.Errorf("%w: %s was active %s", ErrBadRate, o.AttachmentID, o.Active)
	}
	if _, ok := i.Year.Range().Intersect(o.Active); !ok {
		return nil
	}
	i.Occupancies = append(i.Occupancies, o)
	return nil
}

// FromAssembly files every communication attachment on a pole.
func (i *Inventory) FromAssembly(a *plant.Assembly) error {
	if a == nil {
		return fmt.Errorf("%w: no assembly", ErrBadRate)
	}
	for _, att := range a.Communication() {
		err := i.Add(Occupancy{
			AttachmentID: att.ID, Pole: att.Pole, Owner: att.Owner,
			Authorized: att.Authorized, Active: att.InForce,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// Owners returns every attacher in the inventory, sorted.
func (i *Inventory) Owners() []string {
	seen := map[string]bool{}
	var out []string
	for _, o := range i.Occupancies {
		key := strings.ToUpper(strings.TrimSpace(o.Owner))
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, o.Owner)
	}
	sort.Strings(out)
	return out
}

// ForOwner returns the occupancies belonging to one attacher, sorted by pole
// and then attachment.
func (i *Inventory) ForOwner(owner string) []Occupancy {
	want := strings.ToUpper(strings.TrimSpace(owner))
	var out []Occupancy
	for _, o := range i.Occupancies {
		if strings.ToUpper(strings.TrimSpace(o.Owner)) == want {
			out = append(out, o)
		}
	}
	sort.SliceStable(out, func(a, b int) bool {
		if out[a].Pole != out[b].Pole {
			return out[a].Pole < out[b].Pole
		}
		return out[a].AttachmentID < out[b].AttachmentID
	})
	return out
}

// Poles returns every pole in the inventory, sorted.
func (i *Inventory) Poles() []plant.PoleID {
	seen := map[plant.PoleID]bool{}
	var out []plant.PoleID
	for _, o := range i.Occupancies {
		if seen[o.Pole] {
			continue
		}
		seen[o.Pole] = true
		out = append(out, o.Pole)
	}
	sort.Slice(out, func(a, b int) bool { return out[a] < out[b] })
	return out
}

// AttachersOn returns how many distinct attaching entities are on a pole. It
// is the divisor in the telecom formula, so it is counted from the inventory
// rather than assumed.
func (i *Inventory) AttachersOn(pole plant.PoleID) int {
	seen := map[string]bool{}
	for _, o := range i.Occupancies {
		if o.Pole != pole {
			continue
		}
		seen[strings.ToUpper(strings.TrimSpace(o.Owner))] = true
	}
	return len(seen)
}

// Unauthorized returns the occupancies nobody has permission for, sorted the
// same way ForOwner sorts.
func (i *Inventory) Unauthorized() []Occupancy {
	var out []Occupancy
	for _, o := range i.Occupancies {
		if !o.Authorized {
			out = append(out, o)
		}
	}
	sort.SliceStable(out, func(a, b int) bool {
		if out[a].Pole != out[b].Pole {
			return out[a].Pole < out[b].Pole
		}
		return out[a].AttachmentID < out[b].AttachmentID
	})
	return out
}

// Count returns how many occupancies are in the inventory.
func (i *Inventory) Count() int { return len(i.Occupancies) }

// CountFor returns how many attachments an owner has.
func (i *Inventory) CountFor(owner string) int { return len(i.ForOwner(owner)) }

// PartYear returns the occupancies that were not up for the whole rate year,
// which are the ones a bill has to prorate.
func (i *Inventory) PartYear() []Occupancy {
	var out []Occupancy
	for _, o := range i.Occupancies {
		num, den := calendar.ProrationOf(i.Year, o.Active)
		if num < den {
			out = append(out, o)
		}
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].AttachmentID < out[b].AttachmentID })
	return out
}

// String prints the inventory header.
func (i *Inventory) String() string {
	return fmt.Sprintf("%s inventory: %d attachments across %d poles for %d attachers",
		i.Year, i.Count(), len(i.Poles()), len(i.Owners()))
}
