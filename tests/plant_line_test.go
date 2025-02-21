package tests

import (
	"math"
	"testing"

	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/unit"
)

func span(id, from, to string, feet int) plant.SpanRecord {
	return plant.SpanRecord{
		ID: id, FromPole: plant.PoleID(from), ToPole: plant.PoleID(to),
		Length: unit.SpanFeet(feet), Cover: plant.CoverField,
	}
}

func sampleLine(t *testing.T) *plant.Line {
	t.Helper()
	l, err := plant.NewLine("LN-7", []plant.SpanRecord{
		span("SP-1", "NW-441829", "NW-441830", 200),
		span("SP-2", "NW-441830", "NW-441831", 260),
		span("SP-3", "NW-441831", "NW-441832", 140),
	})
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func TestNewLineDerivesPoleOrder(t *testing.T) {
	l := sampleLine(t)
	want := []plant.PoleID{"NW-441829", "NW-441830", "NW-441831", "NW-441832"}
	if len(l.Poles) != len(want) {
		t.Fatalf("poles = %v", l.Poles)
	}
	for i := range want {
		if l.Poles[i] != want[i] {
			t.Errorf("pole %d = %s, want %s", i, l.Poles[i], want[i])
		}
	}
	if l.Len() != 3 {
		t.Errorf("len = %d", l.Len())
	}
}

func TestNewLineRejectsABrokenRun(t *testing.T) {
	if _, err := plant.NewLine("LN-0", nil); err == nil {
		t.Error("an empty line should be rejected")
	}
	_, err := plant.NewLine("LN-8", []plant.SpanRecord{
		span("SP-1", "NW-441829", "NW-441830", 200),
		span("SP-2", "NW-441840", "NW-441841", 200),
	})
	if err == nil {
		t.Error("a run with a gap should be rejected")
	}
	_, err = plant.NewLine("LN-9", []plant.SpanRecord{
		span("SP-1", "NW-441829", "NW-441830", 200),
		span("SP-2", "NW-441830", "NW-441829", 200),
	})
	if err == nil {
		t.Error("a run that doubles back should be rejected")
	}
	_, err = plant.NewLine("LN-10", []plant.SpanRecord{span("SP-1", "NW-441829", "NW-441830", 4)})
	if err == nil {
		t.Error("a bad span should fail the line")
	}
}

func TestLineTotalAndLongest(t *testing.T) {
	l := sampleLine(t)
	if got := l.Total().Feet(); got != 600 {
		t.Errorf("total = %d", got)
	}
	if got := l.Longest().ID; got != "SP-2" {
		t.Errorf("longest = %s", got)
	}
}

func TestRulingSpan(t *testing.T) {
	l := sampleLine(t)
	cubes := 200.0*200*200 + 260.0*260*260 + 140.0*140*140
	want := math.Sqrt(cubes / 600)
	if got := l.RulingSpan(); math.Abs(got-want) > 1e-9 {
		t.Errorf("ruling span = %v, want %v", got, want)
	}
	single, err := plant.NewLine("LN-1", []plant.SpanRecord{span("SP-1", "NW-441829", "NW-441830", 225)})
	if err != nil {
		t.Fatal(err)
	}
	if got := single.RulingSpan(); math.Abs(got-225) > 1e-9 {
		t.Errorf("one span rules at its own length, got %v", got)
	}
	equal, err := plant.NewLine("LN-2", []plant.SpanRecord{
		span("SP-1", "NW-441829", "NW-441830", 180),
		span("SP-2", "NW-441830", "NW-441831", 180),
		span("SP-3", "NW-441831", "NW-441832", 180),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := equal.RulingSpan(); math.Abs(got-180) > 1e-9 {
		t.Errorf("equal spans rule at that length, got %v", got)
	}
	if got := equal.RulingSpan(); got > float64(equal.Longest().Length.Feet())+1e-9 {
		t.Error("the ruling span never exceeds the longest span")
	}
}

func TestLineLookups(t *testing.T) {
	l := sampleLine(t)
	s, ok := l.SpanBetween("NW-441830", "NW-441831")
	if !ok || s.ID != "SP-2" {
		t.Errorf("span between = %v ok=%v", s.ID, ok)
	}
	s, ok = l.SpanBetween("NW-441831", "NW-441830")
	if !ok || s.ID != "SP-2" {
		t.Error("the lookup should not care about direction")
	}
	if _, ok := l.SpanBetween("NW-441829", "NW-441832"); ok {
		t.Error("poles two apart do not share a span")
	}
	if got := len(l.SpansAt("NW-441830")); got != 2 {
		t.Errorf("interior pole spans = %d", got)
	}
	if got := len(l.SpansAt("NW-441829")); got != 1 {
		t.Errorf("dead end spans = %d", got)
	}
	if got := len(l.SpansAt("SE-1129043")); got != 0 {
		t.Errorf("foreign pole spans = %d", got)
	}
}

func TestLineDeadEndsAndUnbalance(t *testing.T) {
	l := sampleLine(t)
	if !l.IsDeadEnd("NW-441829") || !l.IsDeadEnd("NW-441832") {
		t.Error("both ends of the section are dead ends")
	}
	if l.IsDeadEnd("NW-441830") {
		t.Error("an interior pole is not a dead end")
	}
	if got := l.UnbalancedAt("NW-441830"); got != 60 {
		t.Errorf("unbalance at the second pole = %d, want 60", got)
	}
	if got := l.UnbalancedAt("NW-441831"); got != 120 {
		t.Errorf("unbalance at the third pole = %d, want 120", got)
	}
	if got := l.UnbalancedAt("NW-441829"); got != 200 {
		t.Errorf("a dead end carries the whole span, got %d", got)
	}
	if got := l.UnbalancedAt("SE-1129043"); got != 0 {
		t.Errorf("a foreign pole has no unbalance, got %d", got)
	}
}
