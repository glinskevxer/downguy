// Package clearance runs the code rules against a surveyed pole and span, and
// says what is wrong with them.
//
// Every finding names the rule it came from, what was measured, what was
// required, and who it belongs to. The last of those is the one that makes the
// difference commercially: a violation that was already there when the
// applicant arrived is not the applicant's to pay for, and the engine decides
// that here rather than leaving it to an argument later.
package clearance

import (
	"fmt"
	"sort"
	"strings"

	"github.com/glinskevxer/downguy/internal/nesc"
	"github.com/glinskevxer/downguy/internal/unit"
)

// Blame says whose problem a finding is.
type Blame string

// The three answers, and there are only three.
const (
	// BlamePreExisting means the condition was there before the application
	// and is not made worse by it.
	BlamePreExisting Blame = "pre_existing"
	// BlameApplicant means the condition appears only once the new attachment
	// goes on.
	BlameApplicant Blame = "applicant"
	// BlameAggravated means the condition was already there and the new
	// attachment makes it worse.
	BlameAggravated Blame = "aggravated"
	// BlameUnattributed is what a rule emits. A rule sees one pole in one
	// state and cannot tell a condition it caused from one it inherited, so it
	// does not try; Attribute decides by comparing the two states.
	BlameUnattributed Blame = "unattributed"
)

var blameOrder = map[Blame]int{
	BlameApplicant:    0,
	BlameAggravated:   1,
	BlamePreExisting:  2,
	BlameUnattributed: 3,
}

// KnownBlame reports whether the attribution is one the allocator handles.
func KnownBlame(b Blame) bool {
	_, ok := blameOrder[b]
	return ok
}

// Finding is one rule failing against one surveyed pole or span.
type Finding struct {
	RuleID   string
	Severity nesc.Severity
	Pole     string
	Span     string
	Subject  string
	Against  string
	Owner    string
	Blame    Blame
	Measured unit.Height
	Required unit.Height
	Detail   string
}

// Deficit returns how far short the finding is.
func (f Finding) Deficit() unit.Height { return nesc.Deficit(f.Measured, f.Required) }

// Rule returns the catalogue entry the finding cites.
func (f Finding) Rule() (nesc.Rule, error) { return nesc.LookupRule(f.RuleID) }

// Key returns a stable identity for the finding, so that the same condition
// found on two runs is recognised as the same condition rather than as two.
func (f Finding) Key() string {
	parts := []string{f.RuleID, f.Pole, f.Span, f.Subject, f.Against}
	return strings.Join(parts, "|")
}

// String prints the finding the way a make ready sheet lists it.
func (f Finding) String() string {
	where := f.Pole
	if f.Span != "" {
		where = f.Span
	}
	return fmt.Sprintf("%s %s %s: %s", f.RuleID, where, f.Subject, f.Detail)
}

// Findings is a set of findings with the orderings a report needs.
type Findings []Finding

// Sort orders findings worst first: severity, then blame, then the size of the
// deficit, then the rule id so that the order is stable.
func (fs Findings) Sort() {
	sort.SliceStable(fs, func(i, j int) bool {
		a, b := fs[i], fs[j]
		if a.Severity != b.Severity {
			return a.Severity.WorseThan(b.Severity)
		}
		if a.Blame != b.Blame {
			return blameOrder[a.Blame] < blameOrder[b.Blame]
		}
		if a.Deficit() != b.Deficit() {
			return a.Deficit() > b.Deficit()
		}
		return a.Key() < b.Key()
	})
}

// OfSeverity returns the findings at one severity.
func (fs Findings) OfSeverity(s nesc.Severity) Findings {
	var out Findings
	for _, f := range fs {
		if f.Severity == s {
			out = append(out, f)
		}
	}
	return out
}

// OfBlame returns the findings attributed one way.
func (fs Findings) OfBlame(b Blame) Findings {
	var out Findings
	for _, f := range fs {
		if f.Blame == b {
			out = append(out, f)
		}
	}
	return out
}

// OfOwner returns the findings belonging to one attacher, matched without
// regard to case.
func (fs Findings) OfOwner(owner string) Findings {
	want := strings.ToUpper(strings.TrimSpace(owner))
	var out Findings
	for _, f := range fs {
		if strings.ToUpper(strings.TrimSpace(f.Owner)) == want {
			out = append(out, f)
		}
	}
	return out
}

// Blocking reports whether anything in the set stops the work going ahead.
func (fs Findings) Blocking() bool {
	for _, f := range fs {
		if f.Severity == nesc.SeverityStop {
			return true
		}
	}
	return false
}

// Owners returns every attacher named in the set, sorted.
func (fs Findings) Owners() []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range fs {
		name := strings.TrimSpace(f.Owner)
		if name == "" || seen[strings.ToUpper(name)] {
			continue
		}
		seen[strings.ToUpper(name)] = true
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// Rules returns every rule id in the set, sorted.
func (fs Findings) Rules() []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range fs {
		if seen[f.RuleID] {
			continue
		}
		seen[f.RuleID] = true
		out = append(out, f.RuleID)
	}
	sort.Strings(out)
	return out
}
