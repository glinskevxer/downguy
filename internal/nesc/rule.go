package nesc

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ErrUnknownRule is returned when a rule id is not in the catalogue.
var ErrUnknownRule = errors.New("nesc: unknown rule id")

// Severity says what a failure against a rule means for the work. It is not a
// judgement about how dangerous the condition is; it is what has to happen
// next, which is the thing the make ready planner reads.
type Severity string

// The severities, from the one that stops work to the one that only gets
// written down.
const (
	SeverityStop     Severity = "stop"
	SeverityCorrect  Severity = "correct"
	SeverityRecord   Severity = "record"
	SeverityAdvisory Severity = "advisory"
)

var severityRank = map[Severity]int{
	SeverityStop:     0,
	SeverityCorrect:  1,
	SeverityRecord:   2,
	SeverityAdvisory: 3,
}

// WorseThan reports whether s demands more than other.
func (s Severity) WorseThan(other Severity) bool {
	a, oka := severityRank[s]
	b, okb := severityRank[other]
	if !oka || !okb {
		return false
	}
	return a < b
}

// KnownSeverity reports whether the severity is one the planner handles.
func KnownSeverity(s Severity) bool {
	_, ok := severityRank[s]
	return ok
}

// Rule is one checkable requirement. The id is what a finding quotes and what
// an attacher argues against, so it is stable and it never gets reused.
type Rule struct {
	ID       string
	Section  string
	Title    string
	Severity Severity
	Applies  string
}

var catalogue = map[string]Rule{
	"NESC-232A": {
		ID: "NESC-232A", Section: "232", Severity: SeverityCorrect,
		Title:   "vertical clearance of wires above ground",
		Applies: "any span carrying facility over any ground cover",
	},
	"NESC-232B": {
		ID: "NESC-232B", Section: "232", Severity: SeverityCorrect,
		Title:   "vertical clearance above rails",
		Applies: "a span crossing a railroad track",
	},
	"NESC-233A": {
		ID: "NESC-233A", Section: "233", Severity: SeverityCorrect,
		Title:   "clearance between wires crossing at a different level",
		Applies: "two lines crossing in the same span",
	},
	"NESC-234C": {
		ID: "NESC-234C", Section: "234", Severity: SeverityCorrect,
		Title:   "clearance from buildings and signs",
		Applies: "a span passing a building or a sign",
	},
	"NESC-235C1": {
		ID: "NESC-235C1", Section: "235", Severity: SeverityStop,
		Title:   "vertical separation between supply and communication on a structure",
		Applies: "any joint use pole",
	},
	"NESC-235C2": {
		ID: "NESC-235C2", Section: "235", Severity: SeverityCorrect,
		Title:   "vertical separation between communication facilities",
		Applies: "two attachers in communication space",
	},
	"NESC-235E": {
		ID: "NESC-235E", Section: "235", Severity: SeverityCorrect,
		Title:   "separation at mid span between conductors of different circuits",
		Applies: "a span with facilities of more than one owner",
	},
	"NESC-236A": {
		ID: "NESC-236A", Section: "236", Severity: SeverityStop,
		Title:   "climbing space through communication and supply space",
		Applies: "any pole a worker climbs",
	},
	"NESC-238A": {
		ID: "NESC-238A", Section: "238", Severity: SeverityCorrect,
		Title:   "clearance between equipment and the conductors above it",
		Applies: "a pole carrying equipment in communication space",
	},
	"NESC-239E": {
		ID: "NESC-239E", Section: "239", Severity: SeverityRecord,
		Title:   "clearance of a vertical run from other facilities",
		Applies: "a riser or a vertical run on the pole face",
	},
	"NESC-250B": {
		ID: "NESC-250B", Section: "250", Severity: SeverityStop,
		Title:   "structure strength under the district loading case",
		Applies: "every pole in the application",
	},
	"NESC-261A": {
		ID: "NESC-261A", Section: "261", Severity: SeverityCorrect,
		Title:   "guying where the structure cannot carry the unbalanced pull",
		Applies: "a pole at a dead end or a line angle",
	},
}

// LookupRule returns a rule by id.
func LookupRule(id string) (Rule, error) {
	r, ok := catalogue[strings.ToUpper(strings.TrimSpace(id))]
	if !ok {
		return Rule{}, fmt.Errorf("%w: %q", ErrUnknownRule, id)
	}
	return r, nil
}

// MustRule returns a rule by id and panics if it is not in the catalogue. It
// is for the rule implementations, which name their own id as a constant.
func MustRule(id string) Rule {
	r, err := LookupRule(id)
	if err != nil {
		panic(err)
	}
	return r
}

// RuleIDs returns every id in the catalogue, sorted.
func RuleIDs() []string {
	out := make([]string, 0, len(catalogue))
	for id := range catalogue {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// RulesInSection returns the rules from one section of the code, sorted by id.
func RulesInSection(section string) []Rule {
	var out []Rule
	for _, r := range catalogue {
		if r.Section == section {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// String prints the rule the way a finding cites it.
func (r Rule) String() string { return fmt.Sprintf("%s %s", r.ID, r.Title) }
