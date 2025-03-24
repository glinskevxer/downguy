package tests

import (
	"math"
	"testing"

	"github.com/glinskevxer/downguy/internal/sagten"
	"github.com/glinskevxer/downguy/internal/unit"
)

func TestParabolicSag(t *testing.T) {
	got, err := sagten.ParabolicSag(0.2906, 225, 1000)
	if err != nil {
		t.Fatal(err)
	}
	want := 0.2906 * 225 * 225 / 8000
	if math.Abs(got-want) > 1e-12 {
		t.Errorf("sag = %v, want %v", got, want)
	}
	tighter, err := sagten.ParabolicSag(0.2906, 225, 2000)
	if err != nil {
		t.Fatal(err)
	}
	if tighter >= got {
		t.Error("more tension gives less sag")
	}
	longer, err := sagten.ParabolicSag(0.2906, 300, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if longer <= got {
		t.Error("a longer span sags more")
	}
}

func TestParabolicSagRejectsNonsense(t *testing.T) {
	for _, c := range [][3]float64{{0, 225, 1000}, {0.29, 0, 1000}, {0.29, 225, 0}, {-1, 225, 1000}} {
		if _, err := sagten.ParabolicSag(c[0], c[1], c[2]); err == nil {
			t.Errorf("ParabolicSag%v should have failed", c)
		}
	}
}

func TestCatenaryAgreesWithTheParabola(t *testing.T) {
	p, err := sagten.ParabolicSag(0.2906, 225, 1500)
	if err != nil {
		t.Fatal(err)
	}
	c, err := sagten.CatenarySag(0.2906, 225, 1500)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(c-p) > 0.01 {
		t.Errorf("catenary %v and parabola %v differ by more than a tenth of an inch", c, p)
	}
	if c < p {
		t.Error("the catenary is the deeper of the two")
	}
	if _, err := sagten.CatenarySag(0, 225, 1500); err == nil {
		t.Error("a zero load should be rejected")
	}
}

func TestTensionForSagInvertsTheParabola(t *testing.T) {
	sag, err := sagten.ParabolicSag(0.2906, 225, 1500)
	if err != nil {
		t.Fatal(err)
	}
	back, err := sagten.TensionForSag(0.2906, 225, sag)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(back-1500) > 1e-9 {
		t.Errorf("round trip tension = %v, want 1500", back)
	}
	for _, c := range [][3]float64{{0, 225, 3}, {0.29, 0, 3}, {0.29, 225, 0}} {
		if _, err := sagten.TensionForSag(c[0], c[1], c[2]); err == nil {
			t.Errorf("TensionForSag%v should have failed", c)
		}
	}
}

func TestArcLengthExceedsTheSpan(t *testing.T) {
	if got := sagten.ArcLength(225, 3); got <= 225 {
		t.Errorf("arc length = %v, should exceed the span", got)
	}
	if got := sagten.ArcLength(225, 0); got != 225 {
		t.Errorf("a taut wire is the span long, got %v", got)
	}
	if got := sagten.ArcLength(0, 3); got != 0 {
		t.Errorf("no span, no wire, got %v", got)
	}
	deeper := sagten.ArcLength(225, 6)
	shallower := sagten.ArcLength(225, 3)
	if deeper <= shallower {
		t.Error("more sag takes more wire")
	}
}

func TestLevelSpanLowPointIsInTheMiddle(t *testing.T) {
	p, err := sagten.NewProfile(200, 4, unit.Feet(25), unit.Feet(25), 0)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(p.LowPointFeet-100) > 1e-9 {
		t.Errorf("low point = %v, want 100", p.LowPointFeet)
	}
	if math.Abs(p.HeightAt(100)-21) > 1e-9 {
		t.Errorf("mid height = %v, want 21", p.HeightAt(100))
	}
	if math.Abs(p.HeightAt(0)-25) > 1e-9 || math.Abs(p.HeightAt(200)-25) > 1e-9 {
		t.Error("the ends sit at the attachment heights")
	}
	if p.MidspanHeight().Tenths() != 210 {
		t.Errorf("midspan height = %s", p.MidspanHeight())
	}
}

func TestInclinedSpanLowPointShiftsDownhill(t *testing.T) {
	p, err := sagten.NewProfile(200, 4, unit.Feet(30), unit.Feet(20), 0)
	if err != nil {
		t.Fatal(err)
	}
	if p.LowPointFeet <= 100 {
		t.Errorf("the low point should move toward the low end, got %v", p.LowPointFeet)
	}
	up, err := sagten.NewProfile(200, 4, unit.Feet(20), unit.Feet(30), 0)
	if err != nil {
		t.Fatal(err)
	}
	if up.LowPointFeet >= 100 {
		t.Errorf("the low point should move toward the near end, got %v", up.LowPointFeet)
	}
	steep, err := sagten.NewProfile(200, 1, unit.Feet(40), unit.Feet(15), 0)
	if err != nil {
		t.Fatal(err)
	}
	if steep.LowPointFeet != 200 {
		t.Errorf("on a steep span the low end is the low point, got %v", steep.LowPointFeet)
	}
}

func TestGroundDropIsCarriedThrough(t *testing.T) {
	p, err := sagten.NewProfile(200, 4, unit.Feet(25), unit.Feet(25), unit.Feet(6))
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(p.GroundAt(200)-(-6)) > 1e-9 {
		t.Errorf("far ground = %v, want -6", p.GroundAt(200))
	}
	if math.Abs(p.GroundAt(0)) > 1e-9 {
		t.Errorf("near ground = %v, want 0", p.GroundAt(0))
	}
	if math.Abs(p.HeightAt(200)-19) > 1e-9 {
		t.Errorf("the far attachment sits 19 feet over the near datum, got %v", p.HeightAt(200))
	}
	if math.Abs(p.ClearanceAt(200)-25) > 1e-9 {
		t.Errorf("far clearance = %v, want 25", p.ClearanceAt(200))
	}
	level, err := sagten.NewProfile(200, 4, unit.Feet(25), unit.Feet(25), 0)
	if err != nil {
		t.Fatal(err)
	}
	dropped, _ := p.MinimumClearance()
	flat, _ := level.MinimumClearance()
	if dropped != flat {
		t.Errorf("a uniform grade under equal attachment heights changes nothing: %s against %s", dropped, flat)
	}
	// Now let the far pole hold the wire lower over its own ground, and the
	// grade stops cancelling.
	lower, err := sagten.NewProfile(200, 4, unit.Feet(25), unit.Feet(19), unit.Feet(6))
	if err != nil {
		t.Fatal(err)
	}
	tight, _ := lower.MinimumClearance()
	if tight >= flat {
		t.Errorf("a lower far attachment should pull the tightest clearance down, got %s against %s", tight, flat)
	}
}

func TestMinimumClearanceOnALevelSpan(t *testing.T) {
	p, err := sagten.NewProfile(200, 4, unit.Feet(25), unit.Feet(25), 0)
	if err != nil {
		t.Fatal(err)
	}
	got, at := p.MinimumClearance()
	if got.Tenths() != 210 {
		t.Errorf("minimum clearance = %s, want 21.0 ft", got)
	}
	if math.Abs(at-100) > 1 {
		t.Errorf("tightest at %v, want the middle", at)
	}
}

func TestProfileClampsOutOfRangeStations(t *testing.T) {
	p, err := sagten.NewProfile(200, 4, unit.Feet(25), unit.Feet(25), 0)
	if err != nil {
		t.Fatal(err)
	}
	if p.HeightAt(-50) != p.HeightAt(0) {
		t.Error("before the near pole should read as the near pole")
	}
	if p.HeightAt(500) != p.HeightAt(200) {
		t.Error("past the far pole should read as the far pole")
	}
	if p.GroundAt(-50) != p.GroundAt(0) || p.GroundAt(500) != p.GroundAt(200) {
		t.Error("ground should clamp too")
	}
	if _, err := sagten.NewProfile(0, 4, unit.Feet(25), unit.Feet(25), 0); err == nil {
		t.Error("a zero span should be rejected")
	}
	if _, err := sagten.NewProfile(200, -1, unit.Feet(25), unit.Feet(25), 0); err == nil {
		t.Error("a negative sag should be rejected")
	}
}
