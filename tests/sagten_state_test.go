package tests

import (
	"errors"
	"math"
	"testing"

	"github.com/glinskevxer/downguy/internal/nesc"
	"github.com/glinskevxer/downguy/internal/sagten"
	"github.com/glinskevxer/downguy/internal/unit"
)

func sparrow(t *testing.T) sagten.Conductor {
	t.Helper()
	c, err := sagten.Lookup("2 ACSR SPARROW")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestResultantLoadGrowsWithTheDistrict(t *testing.T) {
	c := sparrow(t)
	var last float64
	for _, d := range []nesc.District{nesc.Light, nesc.Medium, nesc.Heavy} {
		l, err := nesc.LoadingFor(d)
		if err != nil {
			t.Fatal(err)
		}
		got, err := sagten.ResultantLoad(c, l)
		if err != nil {
			t.Fatal(err)
		}
		if got <= c.WeightPerFoot {
			t.Errorf("%s load %v should exceed the bare weight %v", d, got, c.WeightPerFoot)
		}
		if got <= last {
			t.Errorf("%s load %v should exceed the lighter district's %v", d, got, last)
		}
		last = got
	}
}

func TestResultantLoadIsTheHypotenusePlusK(t *testing.T) {
	c := sparrow(t)
	l, err := nesc.LoadingFor(nesc.Heavy)
	if err != nil {
		t.Fatal(err)
	}
	vertical := c.WeightPerFoot + l.IceWeightPerFoot(c.DiameterInches)
	horizontal := l.WindLoadPerFoot(c.DiameterInches)
	want := math.Hypot(vertical, horizontal) + l.KPoundsPerFoot()
	got, err := sagten.ResultantLoad(c, l)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(got-want) > 1e-12 {
		t.Errorf("resultant = %v, want %v", got, want)
	}
	bad := c
	bad.WeightPerFoot = 0
	if _, err := sagten.ResultantLoad(bad, l); err == nil {
		t.Error("a weightless conductor should be rejected")
	}
	if sagten.BareLoad(c) != c.WeightPerFoot {
		t.Error("the bare load is the bare weight")
	}
}

func TestDesignConditionSitsAtTheLoadedLimit(t *testing.T) {
	c := sparrow(t)
	got, err := sagten.DesignCondition(c, nesc.Heavy)
	if err != nil {
		t.Fatal(err)
	}
	if got.Tension != c.RatedStrength*0.33 {
		t.Errorf("design tension = %v, want a third of rating", got.Tension)
	}
	if got.Temperature != unit.TempHeavyDistrict {
		t.Errorf("design temperature = %v", got.Temperature)
	}
	if got.LoadPerFoot <= c.WeightPerFoot {
		t.Errorf("the design load should carry ice and wind, got %v", got.LoadPerFoot)
	}
	if _, err := sagten.DesignCondition(c, nesc.District("tropical")); err == nil {
		t.Error("an unknown district should be rejected")
	}
	bad := c
	bad.AreaSquareInches = 0
	if _, err := sagten.DesignCondition(bad, nesc.Heavy); err == nil {
		t.Error("an invalid conductor should be rejected")
	}
}

func TestInitialConditionIsWorkedBackFromTheDesignCase(t *testing.T) {
	c := sparrow(t)
	got, err := sagten.InitialCondition(c, 225, nesc.Heavy)
	if err != nil {
		t.Fatal(err)
	}
	if got.Temperature != unit.TempStringing {
		t.Errorf("stringing temperature = %v", got.Temperature)
	}
	if got.LoadPerFoot != c.WeightPerFoot {
		t.Errorf("stringing load = %v", got.LoadPerFoot)
	}
	if got.Tension >= c.MaxTension(true) {
		t.Errorf("stringing tension %v should be under the loaded limit %v", got.Tension, c.MaxTension(true))
	}
	if got.Tension <= 0 {
		t.Errorf("stringing tension = %v", got.Tension)
	}
	light, err := sagten.InitialCondition(c, 225, nesc.Light)
	if err != nil {
		t.Fatal(err)
	}
	// Both districts design to the same third of rating, but the heavy case
	// reaches it under half an inch of ice at zero degrees. Backing that off
	// to a bare wire on a mild day leaves far less tension than backing off
	// the light case does, which is why a heavy district line is strung
	// slacker than the same wire in a light one.
	if light.Tension <= got.Tension {
		t.Errorf("the light district should string tighter: light %v, heavy %v", light.Tension, got.Tension)
	}
	if _, err := sagten.InitialCondition(c, 0, nesc.Heavy); err == nil {
		t.Error("a zero ruling span should be rejected")
	}
	if _, err := sagten.InitialCondition(c, 225, nesc.District("tropical")); err == nil {
		t.Error("an unknown district should be rejected")
	}
}

func TestTensionFallsAsItGetsHotter(t *testing.T) {
	c := sparrow(t)
	from, err := sagten.InitialCondition(c, 225, nesc.Heavy)
	if err != nil {
		t.Fatal(err)
	}
	cold, err := sagten.SolveTension(c, 225, from, from.LoadPerFoot, unit.Temperature(0))
	if err != nil {
		t.Fatal(err)
	}
	hot, err := sagten.SolveTension(c, 225, from, from.LoadPerFoot, unit.TempMaxOperating)
	if err != nil {
		t.Fatal(err)
	}
	if hot >= cold {
		t.Errorf("a hot wire hangs slacker: hot %v, cold %v", hot, cold)
	}
	same, err := sagten.SolveTension(c, 225, from, from.LoadPerFoot, from.Temperature)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(same-from.Tension) > 0.5 {
		t.Errorf("no change of condition should return the starting tension, got %v against %v", same, from.Tension)
	}
}

func TestTheDesignCaseIsTheHardestPull(t *testing.T) {
	c := sparrow(t)
	from, err := sagten.InitialCondition(c, 225, nesc.Heavy)
	if err != nil {
		t.Fatal(err)
	}
	load, temp, err := sagten.DistrictCondition(c, nesc.Heavy)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := sagten.SolveTension(c, 225, from, load, temp)
	if err != nil {
		t.Fatal(err)
	}
	if loaded <= from.Tension {
		t.Errorf("the heavy case should pull harder than stringing: %v against %v", loaded, from.Tension)
	}
	if math.Abs(loaded-c.MaxTension(true)) > 1 {
		t.Errorf("the heavy case should land back on the design limit, got %v against %v", loaded, c.MaxTension(true))
	}
	cold, err := sagten.SolveTension(c, 225, from, from.LoadPerFoot, unit.Temperature(0))
	if err != nil {
		t.Fatal(err)
	}
	if cold > c.MaxTension(false)*1.6 {
		t.Errorf("a bare cold night should not run away from the everyday limit, got %v", cold)
	}
	if _, _, err := sagten.DistrictCondition(c, nesc.District("tropical")); err == nil {
		t.Error("an unknown district should be rejected")
	}
}

func TestSolveTensionRejectsNonsense(t *testing.T) {
	c := sparrow(t)
	from, err := sagten.InitialCondition(c, 225, nesc.Heavy)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sagten.SolveTension(c, 0, from, from.LoadPerFoot, unit.TempStringing); err == nil {
		t.Error("a zero ruling span should be rejected")
	}
	dead := from
	dead.Tension = 0
	if _, err := sagten.SolveTension(c, 225, dead, from.LoadPerFoot, unit.TempStringing); err == nil {
		t.Error("a zero starting tension should be rejected")
	}
	if _, err := sagten.SolveTension(c, 225, from, 0, unit.TempStringing); err == nil {
		t.Error("a zero new load should be rejected")
	}
	bad := c
	bad.ModulusPSI = 0
	if _, err := sagten.SolveTension(bad, 225, from, from.LoadPerFoot, unit.TempStringing); err == nil {
		t.Error("an invalid conductor should be rejected")
	}
}

func TestSagAtSolvesThenSags(t *testing.T) {
	c := sparrow(t)
	from, err := sagten.InitialCondition(c, 225, nesc.Heavy)
	if err != nil {
		t.Fatal(err)
	}
	sag, tension, err := sagten.SagAt(c, 225, 225, from, from.LoadPerFoot, unit.TempMaxOperating)
	if err != nil {
		t.Fatal(err)
	}
	want, err := sagten.ParabolicSag(from.LoadPerFoot, 225, tension)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(sag-want) > 1e-12 {
		t.Errorf("sag = %v, want %v", sag, want)
	}
	cold, _, err := sagten.SagAt(c, 225, 225, from, from.LoadPerFoot, unit.Temperature(0))
	if err != nil {
		t.Fatal(err)
	}
	if cold >= sag {
		t.Error("a cold wire sags less")
	}
	if _, _, err := sagten.SagAt(c, 225, 0, from, from.LoadPerFoot, unit.TempStringing); err == nil {
		t.Error("a zero span should be rejected")
	}
	if _, _, err := sagten.SagAt(c, 0, 225, from, from.LoadPerFoot, unit.TempStringing); !errors.Is(err, sagten.ErrBadSag) {
		t.Errorf("a zero ruling span error = %v", err)
	}
}
