package tests

import (
	"testing"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/unit"
)

func att(id, owner string, k plant.Kind, tenths int) plant.Attachment {
	a := plant.Attachment{
		ID: id, Pole: "NW-441829", Owner: owner, Kind: k,
		Height: unit.Tenths(tenths), Authorized: true,
		InForce: calendar.Range{From: calendar.MustNew(2019, 1, 1)},
	}
	if k == plant.KindPrimary {
		a.Voltage = 7620
	}
	return a
}

func sampleAssembly() *plant.Assembly {
	return plant.NewAssembly(samplePole(), []plant.Attachment{
		att("A3", "RIDGEPORT CABLEVISION", plant.KindCommCable, 200),
		att("A1", "SUMMIT VALLEY ELECTRIC", plant.KindPrimary, 335),
		att("A4", "NORTHVALE TELEPHONE", plant.KindCommCable, 192),
		att("A2", "SUMMIT VALLEY ELECTRIC", plant.KindNeutral, 285),
	})
}

func TestAssemblySortsFromTheTopDown(t *testing.T) {
	a := sampleAssembly()
	want := []string{"A1", "A2", "A3", "A4"}
	for i, id := range want {
		if a.Attachments[i].ID != id {
			t.Fatalf("position %d = %s, want %s", i, a.Attachments[i].ID, id)
		}
	}
}

func TestAssemblyAddRemoveFind(t *testing.T) {
	a := sampleAssembly()
	a.Add(att("A5", "MERIDIAN FIBER", plant.KindFiber, 210))
	if a.Attachments[2].ID != "A5" {
		t.Errorf("A5 should sort between the neutral and the cable, got %s", a.Attachments[2].ID)
	}
	got, ok := a.Find("A5")
	if !ok || got.Owner != "MERIDIAN FIBER" {
		t.Errorf("Find = %v ok=%v", got, ok)
	}
	if _, ok := a.Find("nope"); ok {
		t.Error("Find should miss")
	}
	if !a.Remove("A5") || len(a.Attachments) != 4 {
		t.Error("remove failed")
	}
	if a.Remove("A5") {
		t.Error("removing twice should report false")
	}
}

func TestAssemblySplitsSupplyFromCommunication(t *testing.T) {
	a := sampleAssembly()
	if got := len(a.Supply()); got != 2 {
		t.Errorf("supply count = %d", got)
	}
	if got := len(a.Communication()); got != 2 {
		t.Errorf("communication count = %d", got)
	}
	low, ok := a.LowestSupply()
	if !ok || low.ID != "A2" {
		t.Errorf("lowest supply = %v ok=%v", low.ID, ok)
	}
	high, ok := a.HighestCommunication()
	if !ok || high.ID != "A3" {
		t.Errorf("highest communication = %v ok=%v", high.ID, ok)
	}
	lowComm, ok := a.LowestCommunication()
	if !ok || lowComm.ID != "A4" {
		t.Errorf("lowest communication = %v", lowComm.ID)
	}
	n, ok := a.Neutral()
	if !ok || n.ID != "A2" {
		t.Errorf("neutral = %v ok=%v", n.ID, ok)
	}
}

func TestAssemblySafetyZone(t *testing.T) {
	a := sampleAssembly()
	gap, ok := a.SafetyZone()
	if !ok || gap.Tenths() != 85 {
		t.Errorf("safety zone = %d tenths ok=%v, want 85", gap.Tenths(), ok)
	}
	bare := plant.NewAssembly(samplePole(), []plant.Attachment{att("A1", "SUMMIT VALLEY ELECTRIC", plant.KindPrimary, 335)})
	if _, ok := bare.SafetyZone(); ok {
		t.Error("a pole with no communication has no safety zone to measure")
	}
	commOnly := plant.NewAssembly(samplePole(), []plant.Attachment{att("A3", "RIDGEPORT CABLEVISION", plant.KindCommCable, 200)})
	if _, ok := commOnly.SafetyZone(); ok {
		t.Error("a pole with no supply has no safety zone to measure")
	}
}

func TestAssemblyOwners(t *testing.T) {
	a := sampleAssembly()
	owners := a.Owners()
	want := []string{"NORTHVALE TELEPHONE", "RIDGEPORT CABLEVISION", "SUMMIT VALLEY ELECTRIC"}
	if len(owners) != len(want) {
		t.Fatalf("owners = %v", owners)
	}
	for i := range want {
		if owners[i] != want[i] {
			t.Errorf("owner %d = %q, want %q", i, owners[i], want[i])
		}
	}
	if got := len(a.OwnedBy("  ridgeport cablevision ")); got != 1 {
		t.Errorf("owned by count = %d", got)
	}
	if got := len(a.OwnedBy("nobody")); got != 0 {
		t.Errorf("unknown owner count = %d", got)
	}
}

func TestAssemblyActiveOn(t *testing.T) {
	a := sampleAssembly()
	removed := att("A6", "OLD TELEPHONE CO", plant.KindCommCable, 180)
	removed.InForce = calendar.Range{From: calendar.MustNew(2010, 1, 1), To: calendar.MustNew(2018, 6, 30)}
	a.Add(removed)
	if got := len(a.ActiveOn(calendar.MustNew(2025, 1, 1))); got != 4 {
		t.Errorf("active in 2025 = %d, want 4", got)
	}
	if got := len(a.ActiveOn(calendar.MustNew(2015, 1, 1))); got != 1 {
		t.Errorf("active in 2015 = %d, want 1", got)
	}
}

func TestAssemblyValidate(t *testing.T) {
	if err := sampleAssembly().Validate(); err != nil {
		t.Fatalf("a good assembly failed: %v", err)
	}
	wrongPole := sampleAssembly()
	wrongPole.Attachments[0].Pole = "SE-1129043"
	if err := wrongPole.Validate(); err == nil {
		t.Error("an attachment filed on another pole should be rejected")
	}
	dup := sampleAssembly()
	dup.Add(att("A1", "SUMMIT VALLEY ELECTRIC", plant.KindPrimary, 330))
	if err := dup.Validate(); err == nil {
		t.Error("a duplicate id should be rejected")
	}
	tooHigh := sampleAssembly()
	tooHigh.Add(att("A9", "MERIDIAN FIBER", plant.KindFiber, 400))
	if err := tooHigh.Validate(); err == nil {
		t.Error("an attachment above the pole top should be rejected")
	}
	badPole := sampleAssembly()
	badPole.Pole.Owner = ""
	if err := badPole.Validate(); err == nil {
		t.Error("a bad pole should fail the assembly")
	}
}
