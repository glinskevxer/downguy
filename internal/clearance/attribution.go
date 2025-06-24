package clearance

import (
	"sort"

	"github.com/glinskevxer/downguy/internal/unit"
)

// Attribute decides whose problem each finding in the "after" set is, by
// comparing it against the same check run without the applicant's attachment
// on the pole. It replaces whatever guess the individual rules made, because a
// rule looking at one pole in one state cannot tell a condition it caused from
// one it inherited.
//
// The three answers, and the rule for each:
//
//   - not in the before set at all: the applicant caused it
//   - in the before set with the same or a smaller deficit: pre existing
//   - in the before set with a smaller deficit than now: aggravated
func Attribute(before, after Findings) Findings {
	prior := map[string]unit.Height{}
	for _, f := range before {
		d := f.Deficit()
		if existing, ok := prior[f.Key()]; !ok || d > existing {
			prior[f.Key()] = d
		}
	}
	out := make(Findings, len(after))
	copy(out, after)
	for i := range out {
		was, ok := prior[out[i].Key()]
		switch {
		case !ok:
			out[i].Blame = BlameApplicant
		case out[i].Deficit() > was:
			out[i].Blame = BlameAggravated
		default:
			out[i].Blame = BlamePreExisting
		}
	}
	out.Sort()
	return out
}

// Resolved returns the findings that were in the before set and are not in the
// after set. Work that removes an existing violation is worth reporting: it is
// how an attacher argues that its make ready left the pole better than it
// found it.
func Resolved(before, after Findings) Findings {
	present := map[string]bool{}
	for _, f := range after {
		present[f.Key()] = true
	}
	var out Findings
	for _, f := range before {
		if !present[f.Key()] {
			out = append(out, f)
		}
	}
	out.Sort()
	return out
}

// Attribution is the summary of who owns what across a whole application.
type Attribution struct {
	Applicant   Findings
	Aggravated  Findings
	PreExisting Findings
	Resolved    Findings
}

// Classify runs the comparison and buckets the findings.
func Classify(before, after Findings) Attribution {
	attributed := Attribute(before, after)
	return Attribution{
		Applicant:   attributed.OfBlame(BlameApplicant),
		Aggravated:  attributed.OfBlame(BlameAggravated),
		PreExisting: attributed.OfBlame(BlamePreExisting),
		Resolved:    Resolved(before, after),
	}
}

// All returns every attributed finding, worst first.
func (a Attribution) All() Findings {
	out := make(Findings, 0, len(a.Applicant)+len(a.Aggravated)+len(a.PreExisting))
	out = append(out, a.Applicant...)
	out = append(out, a.Aggravated...)
	out = append(out, a.PreExisting...)
	out.Sort()
	return out
}

// ApplicantsShare returns the findings the applicant is on the hook for, which
// is everything it caused plus everything it made worse.
func (a Attribution) ApplicantsShare() Findings {
	out := make(Findings, 0, len(a.Applicant)+len(a.Aggravated))
	out = append(out, a.Applicant...)
	out = append(out, a.Aggravated...)
	out.Sort()
	return out
}

// Counts returns how many findings fell into each bucket, keyed by blame.
func (a Attribution) Counts() map[Blame]int {
	return map[Blame]int{
		BlameApplicant:   len(a.Applicant),
		BlameAggravated:  len(a.Aggravated),
		BlamePreExisting: len(a.PreExisting),
	}
}

// AggravatedBy returns how much deeper each aggravated finding got, keyed by
// the finding key and sorted by key so a report reads the same twice.
func AggravatedBy(before, after Findings) []AggravationDelta {
	prior := map[string]unit.Height{}
	for _, f := range before {
		prior[f.Key()] = f.Deficit()
	}
	var out []AggravationDelta
	for _, f := range after {
		was, ok := prior[f.Key()]
		if !ok || f.Deficit() <= was {
			continue
		}
		out = append(out, AggravationDelta{
			Key: f.Key(), RuleID: f.RuleID, Subject: f.Subject,
			Was: was, Now: f.Deficit(), Worse: f.Deficit().Sub(was),
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// AggravationDelta says how much worse one condition got.
type AggravationDelta struct {
	Key     string
	RuleID  string
	Subject string
	Was     unit.Height
	Now     unit.Height
	Worse   unit.Height
}
