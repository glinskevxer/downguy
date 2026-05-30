package clearance

import (
	"sort"
	"strings"
)

// A pole with a cable eighteen inches into the safety zone fails the safety
// zone rule, and then the same cable fails its separation from the one under
// it, and then the equipment below that fails its clearance to the cable. One
// condition, three findings, and a make ready plan that moves three things
// where one would do.
//
// Suppression is the pass that collapses that. It is deliberately conservative:
// it only ever hides a finding that another finding on the same facility
// already forces work on, and it records what it hid so a report can say so.

// Suppression is one finding hidden behind another.
type Suppression struct {
	Hidden  Finding
	Because Finding
	Reason  string
}

// The rules that force a facility to move outright. A finding against any of
// these on a facility makes every softer finding against the same facility
// redundant, because the facility is moving anyway.
var governingRules = map[string]bool{
	"NESC-235C1": true,
	"NESC-236A":  true,
	"NESC-250B":  true,
}

// Governing reports whether a rule forces the facility to move.
func Governing(ruleID string) bool { return governingRules[strings.ToUpper(ruleID)] }

// GoverningRules returns the rules that force a move, sorted.
func GoverningRules() []string {
	out := make([]string, 0, len(governingRules))
	for id := range governingRules {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// Suppress collapses the findings that another finding on the same facility
// already forces work on. It returns what is left and what it hid.
//
// A finding is hidden when all of these hold: it is on the same pole and the
// same subject as a governing finding, it is not itself governing, and it is
// not more severe than the one hiding it. The last condition is what stops a
// stop finding disappearing behind a correct one.
func Suppress(findings Findings) (Findings, []Suppression) {
	governing := map[string]Finding{}
	for _, f := range findings {
		if !Governing(f.RuleID) {
			continue
		}
		key := f.Pole + "|" + f.Subject
		if existing, seen := governing[key]; !seen || f.Severity.WorseThan(existing.Severity) {
			governing[key] = f
		}
	}

	var kept Findings
	var hidden []Suppression
	for _, f := range findings {
		if Governing(f.RuleID) {
			kept = append(kept, f)
			continue
		}
		behind, ok := governing[f.Pole+"|"+f.Subject]
		if !ok || f.Severity.WorseThan(behind.Severity) {
			kept = append(kept, f)
			continue
		}
		hidden = append(hidden, Suppression{
			Hidden: f, Because: behind,
			Reason: f.Subject + " is already being moved for " + behind.RuleID,
		})
	}
	kept.Sort()
	sort.SliceStable(hidden, func(i, j int) bool {
		return hidden[i].Hidden.Key() < hidden[j].Hidden.Key()
	})
	return kept, hidden
}

// SuppressedRules returns the rules that were hidden, sorted, which is what a
// report footnotes.
func SuppressedRules(hidden []Suppression) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range hidden {
		if seen[s.Hidden.RuleID] {
			continue
		}
		seen[s.Hidden.RuleID] = true
		out = append(out, s.Hidden.RuleID)
	}
	sort.Strings(out)
	return out
}
