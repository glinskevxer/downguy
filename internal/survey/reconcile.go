package survey

import (
	"fmt"
	"sort"

	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/unit"
)

// Discrepancy is one way the field disagrees with the record. It is not an
// error: the field is right and the record is what gets corrected. Recording
// them separately is what turns a survey into an inventory correction as well
// as an engineering input.
type Discrepancy struct {
	Subject  string
	Kind     DiscrepancyKind
	Record   unit.Height
	Measured unit.Height
	Detail   string
}

// DiscrepancyKind says what sort of disagreement it is.
type DiscrepancyKind string

// The kinds of disagreement a survey turns up.
const (
	// DiscrepancyHeight is a facility the record has at a different height.
	DiscrepancyHeight DiscrepancyKind = "height"
	// DiscrepancyUnrecorded is a facility on the pole that the record does not
	// carry at all. In joint use this is usually an unauthorised attachment.
	DiscrepancyUnrecorded DiscrepancyKind = "unrecorded"
	// DiscrepancyMissing is a facility the record carries and the crew could
	// not find, which is usually one that was removed without telling anybody.
	DiscrepancyMissing DiscrepancyKind = "missing"
	// DiscrepancyCondition is a pole graded differently from the record.
	DiscrepancyCondition DiscrepancyKind = "condition"
)

// HeightTolerance is how far the field and the record may differ before it
// counts as a disagreement worth writing down. Half a foot is inside what a
// stick reading and a twenty year old record can honestly differ by.
const HeightTolerance = unit.Height(5)

// Reconcile compares a survey sheet against the assembly the record holds and
// returns every disagreement, sorted by subject.
func Reconcile(s PoleSheet, a *plant.Assembly) ([]Discrepancy, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	if a == nil {
		return nil, fmt.Errorf("%w: no assembly to reconcile against", ErrBadSheet)
	}
	if s.Pole != a.Pole.ID {
		return nil, fmt.Errorf("%w: %s surveys %s, not %s", ErrBadSheet, s.ID, s.Pole, a.Pole.ID)
	}

	var out []Discrepancy
	measured := map[string]Measurement{}
	for _, m := range s.Measurements {
		measured[m.Subject] = m
	}

	for _, att := range a.Attachments {
		m, ok := measured[att.ID]
		if !ok {
			out = append(out, Discrepancy{
				Subject: att.ID, Kind: DiscrepancyMissing, Record: att.Height,
				Detail: fmt.Sprintf("%s carries %s at %s and the crew did not find it",
					a.Pole.ID, att.ID, att.Height),
			})
			continue
		}
		if diff := m.Height.Sub(att.Height).Abs(); diff > HeightTolerance {
			out = append(out, Discrepancy{
				Subject: att.ID, Kind: DiscrepancyHeight,
				Record: att.Height, Measured: m.Height,
				Detail: fmt.Sprintf("record %s, measured %s, out by %s", att.Height, m.Height, diff),
			})
		}
	}

	for _, m := range s.Measurements {
		if _, ok := a.Find(m.Subject); ok {
			continue
		}
		out = append(out, Discrepancy{
			Subject: m.Subject, Kind: DiscrepancyUnrecorded, Measured: m.Height,
			Detail: fmt.Sprintf("%s at %s is not on the record for %s", m.Subject, m.Height, a.Pole.ID),
		})
	}

	if s.Condition != a.Pole.Condition {
		out = append(out, Discrepancy{
			Subject: string(a.Pole.ID), Kind: DiscrepancyCondition,
			Detail: fmt.Sprintf("record grades it %s, the crew graded it %s", a.Pole.Condition, s.Condition),
		})
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Subject != out[j].Subject {
			return out[i].Subject < out[j].Subject
		}
		return out[i].Kind < out[j].Kind
	})
	return out, nil
}

// OfKind returns the discrepancies of one kind.
func OfKind(ds []Discrepancy, k DiscrepancyKind) []Discrepancy {
	var out []Discrepancy
	for _, d := range ds {
		if d.Kind == k {
			out = append(out, d)
		}
	}
	return out
}

// Unrecorded returns the subjects found in the field that the record does not
// carry, sorted. These are the candidates for an unauthorised attachment
// claim, which is why they come back on their own.
func Unrecorded(ds []Discrepancy) []string {
	var out []string
	for _, d := range OfKind(ds, DiscrepancyUnrecorded) {
		out = append(out, d.Subject)
	}
	sort.Strings(out)
	return out
}

// WorstHeightGap returns the largest height disagreement and reports whether
// there was one.
func WorstHeightGap(ds []Discrepancy) (Discrepancy, bool) {
	var worst Discrepancy
	found := false
	for _, d := range OfKind(ds, DiscrepancyHeight) {
		gap := d.Measured.Sub(d.Record).Abs()
		if !found || gap > worst.Measured.Sub(worst.Record).Abs() {
			worst, found = d, true
		}
	}
	return worst, found
}

// String prints the discrepancy the way a correction sheet lists it.
func (d Discrepancy) String() string {
	return fmt.Sprintf("%s %s: %s", d.Subject, d.Kind, d.Detail)
}
