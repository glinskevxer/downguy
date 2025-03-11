package tests

import (
	"errors"
	"sort"
	"testing"

	"github.com/glinskevxer/downguy/internal/nesc"
)

func TestSeverityOrdering(t *testing.T) {
	if !nesc.SeverityStop.WorseThan(nesc.SeverityCorrect) {
		t.Error("stop beats correct")
	}
	if !nesc.SeverityCorrect.WorseThan(nesc.SeverityRecord) {
		t.Error("correct beats record")
	}
	if !nesc.SeverityRecord.WorseThan(nesc.SeverityAdvisory) {
		t.Error("record beats advisory")
	}
	if nesc.SeverityAdvisory.WorseThan(nesc.SeverityStop) {
		t.Error("advisory beats nothing")
	}
	if nesc.Severity("panic").WorseThan(nesc.SeverityStop) {
		t.Error("an unknown severity beats nothing")
	}
	if !nesc.KnownSeverity(nesc.SeverityRecord) || nesc.KnownSeverity(nesc.Severity("panic")) {
		t.Error("known severity check")
	}
}

func TestLookupRule(t *testing.T) {
	r, err := nesc.LookupRule("NESC-235C1")
	if err != nil {
		t.Fatal(err)
	}
	if r.Section != "235" || r.Severity != nesc.SeverityStop {
		t.Errorf("rule = %+v", r)
	}
	if r.String() != "NESC-235C1 vertical separation between supply and communication on a structure" {
		t.Errorf("String = %q", r.String())
	}
	if _, err := nesc.LookupRule(" nesc-235c1 "); err != nil {
		t.Errorf("lookup should normalise: %v", err)
	}
	if _, err := nesc.LookupRule("NESC-999"); !errors.Is(err, nesc.ErrUnknownRule) {
		t.Errorf("unknown rule error = %v", err)
	}
}

func TestMustRulePanicsOnAnUnknownID(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("MustRule should have panicked")
		}
	}()
	nesc.MustRule("NESC-999")
}

func TestEveryCatalogueEntryIsWellFormed(t *testing.T) {
	ids := nesc.RuleIDs()
	if len(ids) < 10 {
		t.Fatalf("catalogue holds only %d rules", len(ids))
	}
	if !sort.StringsAreSorted(ids) {
		t.Error("RuleIDs should come back sorted")
	}
	for _, id := range ids {
		r, err := nesc.LookupRule(id)
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		if r.ID != id {
			t.Errorf("%s carries id %q", id, r.ID)
		}
		if r.Title == "" || r.Section == "" || r.Applies == "" {
			t.Errorf("%s is missing a field: %+v", id, r)
		}
		if !nesc.KnownSeverity(r.Severity) {
			t.Errorf("%s has severity %q", id, r.Severity)
		}
	}
}

func TestRulesInSection(t *testing.T) {
	got := nesc.RulesInSection("235")
	if len(got) != 3 {
		t.Fatalf("section 235 holds %d rules", len(got))
	}
	for i := 1; i < len(got); i++ {
		if got[i].ID < got[i-1].ID {
			t.Fatalf("section rules are not sorted: %v", got)
		}
	}
	for _, r := range got {
		if r.Section != "235" {
			t.Errorf("%s is in section %s", r.ID, r.Section)
		}
	}
	if len(nesc.RulesInSection("999")) != 0 {
		t.Error("an unused section should come back empty")
	}
}
