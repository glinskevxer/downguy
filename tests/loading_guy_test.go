package tests

import (
	"math"
	"testing"

	"github.com/glinskevxer/downguy/internal/loading"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/unit"
)

func TestResultantOfOpposedPullsCancels(t *testing.T) {
	pulls := []loading.Pull{
		{Source: "a", Force: 1000, Bearing: 0, Height: unit.Feet(30)},
		{Source: "b", Force: 1000, Bearing: 180, Height: unit.Feet(30)},
	}
	force, bearing := loading.Resultant(pulls)
	if force > 1e-6 {
		t.Errorf("a straight line should cancel, got %v at %d", force, bearing)
	}
	if bearing != 0 {
		t.Errorf("a cancelled pull has no bearing, got %d", bearing)
	}
}

func TestResultantOfPerpendicularPulls(t *testing.T) {
	pulls := []loading.Pull{
		{Source: "a", Force: 300, Bearing: 0, Height: unit.Feet(30)},
		{Source: "b", Force: 400, Bearing: 90, Height: unit.Feet(30)},
	}
	force, bearing := loading.Resultant(pulls)
	if math.Abs(force-500) > 1e-6 {
		t.Errorf("resultant = %v, want 500", force)
	}
	if bearing != 53 {
		t.Errorf("bearing = %d, want 53", bearing)
	}
}

func TestResultantWrapsNegativeBearings(t *testing.T) {
	pulls := []loading.Pull{
		{Source: "a", Force: 300, Bearing: 0, Height: unit.Feet(30)},
		{Source: "b", Force: 300, Bearing: 270, Height: unit.Feet(30)},
	}
	_, bearing := loading.Resultant(pulls)
	if bearing != 315 {
		t.Errorf("bearing = %d, want 315", bearing)
	}
	if force, _ := loading.Resultant(nil); force != 0 {
		t.Errorf("no pulls, no resultant, got %v", force)
	}
}

func TestMomentOfTakesEachPullAtItsOwnHeight(t *testing.T) {
	pulls := []loading.Pull{
		{Source: "a", Force: 100, Bearing: 0, Height: unit.Feet(30)},
		{Source: "b", Force: 200, Bearing: 0, Height: unit.Feet(10)},
	}
	if got := loading.MomentOf(pulls); math.Abs(got-5000) > 1e-9 {
		t.Errorf("moment = %v, want 5000", got)
	}
	if got := loading.MomentOf(nil); got != 0 {
		t.Errorf("no pulls, no moment, got %v", got)
	}
}

func TestEffectiveBearingIsTheOpposite(t *testing.T) {
	cases := map[int]int{0: 180, 90: 270, 180: 0, 270: 90, 359: 179}
	for in, want := range cases {
		if got := loading.EffectiveBearing(in); got != want {
			t.Errorf("EffectiveBearing(%d) = %d, want %d", in, got, want)
		}
	}
}

func TestUnguyedDeadEndFails(t *testing.T) {
	pulls := []loading.Pull{{Source: "A1", Force: 2000, Bearing: 90, Height: unit.Feet(30)}}
	got, err := loading.CheckGuying(samplePole(), pulls, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Adequate {
		t.Error("an unguyed dead end should fail")
	}
	if got.Deficit() != got.PullMoment {
		t.Errorf("deficit = %v, want the whole pull moment %v", got.Deficit(), got.PullMoment)
	}
	if got.Note != "unguyed and carrying an unbalanced pull" {
		t.Errorf("note = %q", got.Note)
	}
	if got.Rule() != "NESC-261A" {
		t.Errorf("rule = %q", got.Rule())
	}
}

func TestAGuyInTheRightDirectionAnswersThePull(t *testing.T) {
	pulls := []loading.Pull{{Source: "A1", Force: 2000, Bearing: 90, Height: unit.Feet(30)}}
	anchor := plant.Anchor{
		ID: "ANC-1", Pole: "NW-441829", Lead: 30, Bearing: 270,
		Guys: []plant.Guy{{ID: "G1", Height: unit.Feet(30), Size: "3/8 EHS"}},
	}
	got, err := loading.CheckGuying(samplePole(), pulls, []plant.Anchor{anchor})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Adequate {
		t.Errorf("a guy set opposite the pull should answer it: %+v", got)
	}
	if got.Deficit() != 0 {
		t.Errorf("deficit = %v", got.Deficit())
	}
	if got.Note != "guyed" {
		t.Errorf("note = %q", got.Note)
	}
}

func TestAGuySetSidewaysDoesNothing(t *testing.T) {
	pulls := []loading.Pull{{Source: "A1", Force: 2000, Bearing: 90, Height: unit.Feet(30)}}
	sideways := plant.Anchor{
		ID: "ANC-1", Pole: "NW-441829", Lead: 30, Bearing: 0,
		Guys: []plant.Guy{{ID: "G1", Height: unit.Feet(30), Size: "3/8 EHS"}},
	}
	got, err := loading.CheckGuying(samplePole(), pulls, []plant.Anchor{sideways})
	if err != nil {
		t.Fatal(err)
	}
	if got.GuyMoment != 0 {
		t.Errorf("a guy at right angles should contribute nothing, got %v", got.GuyMoment)
	}
	if got.Adequate {
		t.Error("and the pole should still fail")
	}
	if got.Note != "guyed, but not enough for the pull" {
		t.Errorf("note = %q", got.Note)
	}
}

func TestABalancedPoleNeedsNoGuy(t *testing.T) {
	pulls := []loading.Pull{
		{Source: "A1", Force: 2000, Bearing: 0, Height: unit.Feet(30)},
		{Source: "A2", Force: 2000, Bearing: 180, Height: unit.Feet(30)},
	}
	got, err := loading.CheckGuying(samplePole(), pulls, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Adequate {
		t.Error("a balanced pole needs no guy")
	}
	if got.Note != "nothing unbalanced to guy against" {
		t.Errorf("note = %q", got.Note)
	}
}

func TestCheckGuyingRejectsAForeignAnchor(t *testing.T) {
	pulls := []loading.Pull{{Source: "A1", Force: 2000, Bearing: 90, Height: unit.Feet(30)}}
	foreign := plant.Anchor{
		ID: "ANC-1", Pole: "SE-1129043", Lead: 30, Bearing: 270,
		Guys: []plant.Guy{{ID: "G1", Height: unit.Feet(30), Size: "3/8 EHS"}},
	}
	if _, err := loading.CheckGuying(samplePole(), pulls, []plant.Anchor{foreign}); err == nil {
		t.Error("an anchor filed on another pole should be rejected")
	}
	broken := plant.Anchor{ID: "ANC-2", Pole: "NW-441829", Lead: 30, Bearing: 270}
	if _, err := loading.CheckGuying(samplePole(), pulls, []plant.Anchor{broken}); err == nil {
		t.Error("an anchor with no guys should be rejected")
	}
	bad := samplePole()
	bad.Owner = ""
	if _, err := loading.CheckGuying(bad, pulls, nil); err == nil {
		t.Error("an invalid pole should be rejected")
	}
}

func TestDeadEndPullsComeBackHighestFirst(t *testing.T) {
	wires := sampleWires(t)
	for i := range wires {
		wires[i].Tension = 1500
	}
	wires[0].Tension = 0
	pulls := loading.DeadEndPulls(wires, 90)
	if len(pulls) != 2 {
		t.Fatalf("pulls = %d", len(pulls))
	}
	if pulls[0].Height <= pulls[1].Height {
		t.Error("pulls should come back highest first")
	}
	for _, p := range pulls {
		if p.Bearing != 90 || p.Force != 1500 {
			t.Errorf("pull = %+v", p)
		}
	}
}

func TestAnglePullsSkipStraightWires(t *testing.T) {
	wires := sampleWires(t)
	for i := range wires {
		wires[i].Tension = 1500
	}
	wires[1].AngleDegrees = 30
	pulls, err := loading.AnglePulls(wires, 45)
	if err != nil {
		t.Fatal(err)
	}
	if len(pulls) != 1 {
		t.Fatalf("pulls = %d", len(pulls))
	}
	want, err := loading.AngleTension(1500, 30)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(pulls[0].Force-want) > 1e-9 {
		t.Errorf("angle pull = %v, want %v", pulls[0].Force, want)
	}
	wires[2].AngleDegrees = 400
	wires[2].Tension = 1500
	if _, err := loading.AnglePulls(wires, 45); err == nil {
		t.Error("an impossible deflection should be rejected")
	}
}

func TestWindPullsCoverEveryWire(t *testing.T) {
	l := heavyLoading(t)
	wires := sampleWires(t)
	pulls, err := loading.WindPulls(wires, l, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(pulls) != len(wires) {
		t.Fatalf("pulls = %d", len(pulls))
	}
	for _, p := range pulls {
		if p.Force <= 0 || p.Bearing != 0 {
			t.Errorf("pull = %+v", p)
		}
	}
	wires[0].Conductor.DiameterInches = 0
	if _, err := loading.WindPulls(wires, l, 0); err == nil {
		t.Error("an invalid conductor should be rejected")
	}
}
