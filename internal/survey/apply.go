package survey

import (
	"fmt"
	"sort"

	"github.com/glinskevxer/downguy/internal/plant"
)

// Applied is the assembly as the field found it, with the record's heights
// replaced by the measured ones and anything the crew found but the record
// does not carry added in. It is what the clearance engine runs on, because a
// clearance check against the record rather than the field is a check of the
// paperwork.
type Applied struct {
	Assembly     *plant.Assembly
	Sheet        PoleSheet
	Discrepancy  []Discrepancy
	Methods      map[string]Method
	AddedSubject []string
}

// Apply builds the surveyed assembly. Subjects found in the field that the
// record does not carry are added as unauthorised communication attachments,
// which is what an unrecorded facility in communication space nearly always
// turns out to be, and they are listed by name so nothing is silently
// invented.
func Apply(s PoleSheet, a *plant.Assembly) (*Applied, error) {
	ds, err := Reconcile(s, a)
	if err != nil {
		return nil, err
	}
	out := &Applied{Sheet: s, Discrepancy: ds, Methods: map[string]Method{}}

	atts := make([]plant.Attachment, 0, len(a.Attachments))
	for _, att := range a.Attachments {
		m, ok := s.For(att.ID)
		if !ok {
			// The crew could not find it, so it does not go on the surveyed
			// assembly at all. The discrepancy list already says why.
			continue
		}
		att.Height = m.Height
		out.Methods[att.ID] = m.Method
		atts = append(atts, att)
	}

	for _, m := range s.Measurements {
		if _, ok := a.Find(m.Subject); ok {
			continue
		}
		atts = append(atts, plant.Attachment{
			ID:         m.Subject,
			Pole:       a.Pole.ID,
			Owner:      UnknownOwner,
			Kind:       plant.KindCommCable,
			Height:     m.Height,
			Authorized: false,
			Note:       "found in the field, not on the record",
		})
		out.Methods[m.Subject] = m.Method
		out.AddedSubject = append(out.AddedSubject, m.Subject)
	}
	sort.Strings(out.AddedSubject)

	pole := a.Pole
	pole.Condition = s.Condition
	out.Assembly = plant.NewAssembly(pole, atts)
	return out, nil
}

// UnknownOwner is what an unrecorded attachment is filed under until somebody
// works out whose it is.
const UnknownOwner = "UNIDENTIFIED"

// MethodFor returns how a facility's height was obtained on this survey.
func (a *Applied) MethodFor(subject string) (Method, bool) {
	m, ok := a.Methods[subject]
	return m, ok
}

// AllReliable reports whether every height on the surveyed assembly came from
// a method good enough to decide a clearance on.
func (a *Applied) AllReliable() bool {
	for _, m := range a.Methods {
		if !m.Reliable() {
			return false
		}
	}
	return true
}

// Unauthorized returns the facilities on the surveyed assembly that are not
// authorised, from the highest down.
func (a *Applied) Unauthorized() []plant.Attachment {
	return a.Assembly.Where(func(att plant.Attachment) bool { return !att.Authorized })
}

// Removed returns the subjects the record carries that the crew could not
// find, sorted.
func (a *Applied) Removed() []string {
	var out []string
	for _, d := range OfKind(a.Discrepancy, DiscrepancyMissing) {
		out = append(out, d.Subject)
	}
	sort.Strings(out)
	return out
}

// Summary writes the one line a survey report leads with.
func (a *Applied) Summary() string {
	return fmt.Sprintf("%s: %d facilities surveyed, %d added, %d gone, %d discrepancies",
		a.Assembly.Pole.ID, len(a.Assembly.Attachments),
		len(a.AddedSubject), len(a.Removed()), len(a.Discrepancy))
}
