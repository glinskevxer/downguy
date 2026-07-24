package tests

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glinskevxer/downguy/internal/allocate"
	"github.com/glinskevxer/downguy/internal/apiserver"
	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/config"
	"github.com/glinskevxer/downguy/internal/makeready"
	"github.com/glinskevxer/downguy/internal/nesc"
	"github.com/glinskevxer/downguy/internal/rental"
	"github.com/glinskevxer/downguy/internal/sagten"
	"github.com/glinskevxer/downguy/internal/unit"
)

// The corners the ordinary tests walk past: a printer nothing prints in a
// happy path, a date accessor nothing reads, a branch that only fires on a
// pole nobody built. They are here rather than left uncovered, because an
// uncovered branch is one nobody has ever watched run.

func TestDisputeStringReadsAsALedgerLine(t *testing.T) {
	t.Parallel()
	d := allocate.Dispute{
		ID: "DIS-1", Invoice: "MRI-001", Party: "MERIDIAN FIBER",
		Ground: allocate.GroundPricing, Raised: calendar.MustNew(2025, 8, 12),
		Claimed: unit.Dollars(600), Outcome: allocate.OutcomeOpen,
	}
	got := d.String()
	for _, want := range []string{"DIS-1", "MERIDIAN FIBER", "MRI-001", "pricing", "$600.00", "open"} {
		if !strings.Contains(got, want) {
			t.Errorf("String does not mention %q: %q", want, got)
		}
	}
}

func TestDatePartsReadBack(t *testing.T) {
	t.Parallel()
	d := calendar.MustNew(2025, 3, 9)
	if d.Year() != 2025 || d.Month() != 3 || d.Day() != 9 {
		t.Errorf("parts = %d %d %d", d.Year(), d.Month(), d.Day())
	}
	zero := calendar.Zero
	if zero.Year() != 0 || zero.Month() != 0 || zero.Day() != 0 {
		t.Errorf("the zero date has no parts, got %d %d %d", zero.Year(), zero.Month(), zero.Day())
	}
}

func TestStructuralSummaryOnAGuyablePole(t *testing.T) {
	t.Parallel()
	// The summary has a line per outcome and only two of them are reached by
	// the ordinary fixtures.
	passing := makeready.Structural{Pole: "NW-441829", Passes: true, After: 0.62}
	if !strings.Contains(passing.Summary(), "62%") {
		t.Errorf("Summary = %q", passing.Summary())
	}
	guyable := makeready.Structural{Pole: "NW-441829", Guyable: true, After: 1.14}
	if !strings.Contains(guyable.Summary(), "needs guying") {
		t.Errorf("Summary = %q", guyable.Summary())
	}
	stuck := makeready.Structural{Pole: "NW-441829", After: 3.2}
	if !strings.Contains(stuck.Summary(), "nothing in stock") {
		t.Errorf("Summary = %q", stuck.Summary())
	}
}

func TestInventoryUnauthorizedOnACleanSystem(t *testing.T) {
	t.Parallel()
	inv := rental.NewInventory(calendar.RateYear(2025))
	if err := inv.Add(occ("A3", "NW-441829", "RIDGEPORT CABLEVISION", true, "2019-04-01", "")); err != nil {
		t.Fatal(err)
	}
	if got := inv.Unauthorized(); got != nil {
		t.Errorf("nothing is unauthorized, got %v", got)
	}
	claims, err := rental.AssessAll(inv, rental.DiscoveryAudit, calendar.MustNew(2026, 5, 1), calendar.Zero, nil)
	if err != nil {
		t.Fatal(err)
	}
	if claims != nil {
		t.Errorf("no claims, got %v", claims)
	}
}

func TestSagLowPointOnATautWire(t *testing.T) {
	t.Parallel()
	// A wire with no sag at all has its low point at whichever end is lower,
	// which is the branch the parabola never reaches.
	level, err := sagten.NewProfile(200, 0, unit.Feet(25), unit.Feet(25), 0)
	if err != nil {
		t.Fatal(err)
	}
	if level.LowPointFeet != 0 {
		t.Errorf("a taut level span has nothing hanging, got %v", level.LowPointFeet)
	}
	downhill, err := sagten.NewProfile(200, 0, unit.Feet(30), unit.Feet(20), 0)
	if err != nil {
		t.Fatal(err)
	}
	if downhill.LowPointFeet != 200 {
		t.Errorf("a taut wire is lowest at its low end, got %v", downhill.LowPointFeet)
	}
	if got := level.HeightAt(100); got != 25 {
		t.Errorf("a taut wire is a straight line, got %v", got)
	}
}

func TestReadinessReportsAnEmptyDatabase(t *testing.T) {
	t.Parallel()
	s, err := apiserver.New(openStore(t), config.Default())
	if err != nil {
		t.Fatal(err)
	}
	rec, _ := get(t, s.Handler(), "/readyz")
	if rec.Code != http.StatusOK {
		t.Errorf("code = %d", rec.Code)
	}
}

func TestALongPathIsTrimmedInTheAccessLog(t *testing.T) {
	t.Parallel()
	s, err := apiserver.New(openStore(t), config.Default())
	if err != nil {
		t.Fatal(err)
	}
	log := &apiserver.CollectingLogger{}
	h := s.WithLogger(log).Handler()
	req := httptest.NewRequest(http.MethodGet, "/"+strings.Repeat("a", 400), nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if len(log.Lines) != 1 {
		t.Fatalf("lines = %v", log.Lines)
	}
	if len(log.Lines[0].Path) > 210 {
		t.Errorf("a four hundred character path went into the log whole: %d characters", len(log.Lines[0].Path))
	}
	if !strings.HasSuffix(log.Lines[0].Path, "...") {
		t.Errorf("a trimmed path should say it was trimmed, got %q", log.Lines[0].Path)
	}
}

func TestRuleSectionsAllResolve(t *testing.T) {
	t.Parallel()
	// Every catalogued rule has to belong to a section that lists it back, or
	// the rules command can show a rule that its own section filter hides.
	for _, id := range nesc.RuleIDs() {
		rule, err := nesc.LookupRule(id)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, sibling := range nesc.RulesInSection(rule.Section) {
			if sibling.ID == id {
				found = true
			}
		}
		if !found {
			t.Errorf("%s is in section %s and that section does not list it", id, rule.Section)
		}
	}
}
