package plant

import (
	"fmt"
	"sort"
	"strings"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/unit"
)

// Assembly is a pole together with everything on it. Every engineering package
// takes an Assembly rather than a bag of records, so the ordering questions
// that the clearance rules keep asking are answered in one place.
type Assembly struct {
	Pole        Pole
	Attachments []Attachment
	Anchors     []Anchor
}

// NewAssembly builds an assembly with its attachments sorted from the highest
// facility down, which is the order a field crew writes a survey sheet in.
func NewAssembly(p Pole, atts []Attachment) *Assembly {
	a := &Assembly{Pole: p, Attachments: make([]Attachment, len(atts))}
	copy(a.Attachments, atts)
	a.sortDown()
	return a
}

func (a *Assembly) sortDown() {
	sort.SliceStable(a.Attachments, func(i, j int) bool {
		if a.Attachments[i].Height != a.Attachments[j].Height {
			return a.Attachments[i].Height > a.Attachments[j].Height
		}
		return a.Attachments[i].ID < a.Attachments[j].ID
	})
}

// Add puts a facility on the pole and keeps the ordering.
func (a *Assembly) Add(att Attachment) {
	a.Attachments = append(a.Attachments, att)
	a.sortDown()
}

// Remove takes a facility off by id and reports whether it was there.
func (a *Assembly) Remove(id string) bool {
	for i, att := range a.Attachments {
		if att.ID == id {
			a.Attachments = append(a.Attachments[:i], a.Attachments[i+1:]...)
			return true
		}
	}
	return false
}

// Find returns the attachment with the id and reports whether it is on the
// pole.
func (a *Assembly) Find(id string) (Attachment, bool) {
	for _, att := range a.Attachments {
		if att.ID == id {
			return att, true
		}
	}
	return Attachment{}, false
}

// Where returns the attachments the predicate accepts, in height order.
func (a *Assembly) Where(keep func(Attachment) bool) []Attachment {
	var out []Attachment
	for _, att := range a.Attachments {
		if keep(att) {
			out = append(out, att)
		}
	}
	return out
}

// Supply returns the energised facilities from the top down.
func (a *Assembly) Supply() []Attachment {
	return a.Where(func(att Attachment) bool { return att.Kind.IsSupply() })
}

// Communication returns the communication facilities from the top down.
func (a *Assembly) Communication() []Attachment {
	return a.Where(func(att Attachment) bool { return att.Kind.IsCommunication() })
}

// ActiveOn returns the facilities that were on the pole on the day.
func (a *Assembly) ActiveOn(d calendar.Date) []Attachment {
	return a.Where(func(att Attachment) bool { return att.ActiveOn(d) })
}

// OwnedBy returns the facilities belonging to one attacher, matched without
// regard to case or surrounding space.
func (a *Assembly) OwnedBy(owner string) []Attachment {
	want := strings.ToUpper(strings.TrimSpace(owner))
	return a.Where(func(att Attachment) bool {
		return strings.ToUpper(strings.TrimSpace(att.Owner)) == want
	})
}

// Owners returns every distinct attacher on the pole, sorted.
func (a *Assembly) Owners() []string {
	seen := map[string]bool{}
	var out []string
	for _, att := range a.Attachments {
		name := strings.TrimSpace(att.Owner)
		if name == "" || seen[strings.ToUpper(name)] {
			continue
		}
		seen[strings.ToUpper(name)] = true
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// Lowest returns the lowest facility matching the predicate and reports
// whether there is one.
func (a *Assembly) Lowest(keep func(Attachment) bool) (Attachment, bool) {
	for i := len(a.Attachments) - 1; i >= 0; i-- {
		if keep(a.Attachments[i]) {
			return a.Attachments[i], true
		}
	}
	return Attachment{}, false
}

// Highest returns the highest facility matching the predicate and reports
// whether there is one.
func (a *Assembly) Highest(keep func(Attachment) bool) (Attachment, bool) {
	for _, att := range a.Attachments {
		if keep(att) {
			return att, true
		}
	}
	return Attachment{}, false
}

// LowestSupply returns the lowest energised facility on the pole. It is the
// bottom of supply space and the top of the safety zone, so it is the single
// most asked for reading in the whole system.
func (a *Assembly) LowestSupply() (Attachment, bool) {
	return a.Lowest(func(att Attachment) bool { return att.Kind.IsSupply() })
}

// HighestCommunication returns the top facility in communication space.
func (a *Assembly) HighestCommunication() (Attachment, bool) {
	return a.Highest(func(att Attachment) bool { return att.Kind.IsCommunication() })
}

// LowestCommunication returns the bottom facility in communication space.
func (a *Assembly) LowestCommunication() (Attachment, bool) {
	return a.Lowest(func(att Attachment) bool { return att.Kind.IsCommunication() })
}

// Neutral returns the neutral conductor, which is the reference the safety
// zone is usually measured from when one is present.
func (a *Assembly) Neutral() (Attachment, bool) {
	return a.Lowest(func(att Attachment) bool { return att.Kind == KindNeutral })
}

// SafetyZone returns the gap between the lowest supply facility and the
// highest communication facility, and reports whether both are present.
func (a *Assembly) SafetyZone() (unit.Height, bool) {
	supply, ok := a.LowestSupply()
	if !ok {
		return 0, false
	}
	comm, ok := a.HighestCommunication()
	if !ok {
		return 0, false
	}
	return supply.Height.Sub(comm.Height), true
}

// Validate checks the pole and every facility on it, and reports the first
// problem it finds.
func (a *Assembly) Validate() error {
	if err := a.Pole.Validate(); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, att := range a.Attachments {
		if err := att.Validate(); err != nil {
			return err
		}
		if att.Pole != a.Pole.ID {
			return fmt.Errorf("%w: %s is filed on %s but sits on %s", ErrBadAttachment, att.ID, att.Pole, a.Pole.ID)
		}
		if seen[att.ID] {
			return fmt.Errorf("%w: %s appears twice", ErrBadAttachment, att.ID)
		}
		seen[att.ID] = true
		if att.Height >= a.Pole.AboveGrade() {
			return fmt.Errorf("%w: %s is at %s on a pole standing %s", ErrBadAttachment, att.ID, att.Height, a.Pole.AboveGrade())
		}
	}
	return nil
}
