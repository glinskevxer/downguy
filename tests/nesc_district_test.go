package tests

import (
	"errors"
	"math"
	"testing"

	"github.com/glinskevxer/downguy/internal/nesc"
	"github.com/glinskevxer/downguy/internal/unit"
)

func TestLoadingForEachDistrict(t *testing.T) {
	cases := []struct {
		d      nesc.District
		ice    float64
		wind   float64
		temp   unit.Temperature
		kPerFt float64
	}{
		{nesc.Heavy, 0.50, 4.0, unit.TempHeavyDistrict, 0.30},
		{nesc.Medium, 0.25, 4.0, unit.TempMediumDistrict, 0.20},
		{nesc.Light, 0.00, 9.0, unit.TempLightDistrict, 0.05},
	}
	for _, c := range cases {
		l, err := nesc.LoadingFor(c.d)
		if err != nil {
			t.Fatalf("LoadingFor(%s): %v", c.d, err)
		}
		if math.Abs(l.IceInches()-c.ice) > 1e-9 {
			t.Errorf("%s ice = %v, want %v", c.d, l.IceInches(), c.ice)
		}
		if math.Abs(l.WindPSF()-c.wind) > 1e-9 {
			t.Errorf("%s wind = %v, want %v", c.d, l.WindPSF(), c.wind)
		}
		if l.Temperature != c.temp {
			t.Errorf("%s temp = %v, want %v", c.d, l.Temperature, c.temp)
		}
		if math.Abs(l.KPoundsPerFoot()-c.kPerFt) > 1e-9 {
			t.Errorf("%s k = %v, want %v", c.d, l.KPoundsPerFoot(), c.kPerFt)
		}
	}
	if _, err := nesc.LoadingFor(nesc.District("tropical")); !errors.Is(err, nesc.ErrUnknownDistrict) {
		t.Errorf("unknown district error = %v", err)
	}
}

func TestParseDistrict(t *testing.T) {
	for _, in := range []string{"heavy", "HEAVY", " Heavy "} {
		got, err := nesc.ParseDistrict(in)
		if err != nil || got != nesc.Heavy {
			t.Errorf("ParseDistrict(%q) = %q %v", in, got, err)
		}
	}
	if _, err := nesc.ParseDistrict("tropical"); err == nil {
		t.Error("tropical is not a district")
	}
	all := nesc.Districts()
	if len(all) != 3 || all[0] != nesc.Heavy || all[2] != nesc.Light {
		t.Errorf("districts = %v", all)
	}
}

func TestDistrictSeverityOrders(t *testing.T) {
	if nesc.Heavy.Severity() <= nesc.Medium.Severity() {
		t.Error("heavy outranks medium")
	}
	if nesc.Medium.Severity() <= nesc.Light.Severity() {
		t.Error("medium outranks light")
	}
	if nesc.District("tropical").Severity() != 0 {
		t.Error("an unknown district has no severity")
	}
	if nesc.Heavy.String() != "heavy" {
		t.Errorf("String = %q", nesc.Heavy.String())
	}
}

func TestIceSleeveWeight(t *testing.T) {
	heavy, err := nesc.LoadingFor(nesc.Heavy)
	if err != nil {
		t.Fatal(err)
	}
	// A half inch sleeve on a 0.563 inch conductor is an annulus of
	// pi * 0.5 * (0.563 + 0.5) square inches at 57 pounds per cubic foot.
	want := math.Pi * 0.5 * (0.563 + 0.5) / 144 * nesc.IceDensity
	got := heavy.IceWeightPerFoot(0.563)
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("ice weight = %v, want %v", got, want)
	}
	light, err := nesc.LoadingFor(nesc.Light)
	if err != nil {
		t.Fatal(err)
	}
	if got := light.IceWeightPerFoot(0.563); got != 0 {
		t.Errorf("the light district has no ice, got %v", got)
	}
	if got := heavy.IceWeightPerFoot(0); got != 0 {
		t.Errorf("a conductor with no diameter carries no ice, got %v", got)
	}
	medium, err := nesc.LoadingFor(nesc.Medium)
	if err != nil {
		t.Fatal(err)
	}
	if medium.IceWeightPerFoot(0.563) >= got {
		t.Error("a quarter inch of ice weighs less than a half inch")
	}
}

func TestIcedDiameterAndWindLoad(t *testing.T) {
	heavy, err := nesc.LoadingFor(nesc.Heavy)
	if err != nil {
		t.Fatal(err)
	}
	if got := heavy.IcedDiameter(0.563); math.Abs(got-1.563) > 1e-9 {
		t.Errorf("iced diameter = %v, want 1.563", got)
	}
	want := 1.563 / 12 * 4.0
	if got := heavy.WindLoadPerFoot(0.563); math.Abs(got-want) > 1e-9 {
		t.Errorf("wind load = %v, want %v", got, want)
	}
	light, err := nesc.LoadingFor(nesc.Light)
	if err != nil {
		t.Fatal(err)
	}
	if got := light.IcedDiameter(0.563); math.Abs(got-0.563) > 1e-9 {
		t.Errorf("no ice means no growth, got %v", got)
	}
	if got := light.WindLoadPerFoot(0); got != 0 {
		t.Errorf("nothing to blow on = %v", got)
	}
}
