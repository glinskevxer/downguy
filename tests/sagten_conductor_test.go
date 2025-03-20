package tests

import (
	"errors"
	"sort"
	"testing"

	"github.com/glinskevxer/downguy/internal/sagten"
)

func TestConductorLookup(t *testing.T) {
	c, err := sagten.Lookup("4/0 ACSR PENGUIN")
	if err != nil {
		t.Fatal(err)
	}
	if c.DiameterInches != 0.563 || c.RatedStrength != 8350 {
		t.Errorf("penguin = %+v", c)
	}
	if _, err := sagten.Lookup("  4/0 acsr penguin "); err != nil {
		t.Errorf("lookup should normalise: %v", err)
	}
	if _, err := sagten.Lookup("baling wire"); !errors.Is(err, sagten.ErrUnknownConductor) {
		t.Errorf("unknown conductor error = %v", err)
	}
}

func TestConductorCodesAreSorted(t *testing.T) {
	codes := sagten.Codes()
	if len(codes) < 8 {
		t.Fatalf("catalogue holds %d conductors", len(codes))
	}
	if !sort.StringsAreSorted(codes) {
		t.Errorf("codes are not sorted: %v", codes)
	}
}

func TestEveryCataloguedConductorValidates(t *testing.T) {
	for _, code := range sagten.Codes() {
		c, err := sagten.Lookup(code)
		if err != nil {
			t.Fatalf("%s: %v", code, err)
		}
		if err := c.Validate(); err != nil {
			t.Errorf("%s does not validate: %v", code, err)
		}
		if c.Code != code {
			t.Errorf("%s carries code %q", code, c.Code)
		}
		if c.Kind != "supply" && c.Kind != "communication" {
			t.Errorf("%s has kind %q", code, c.Kind)
		}
	}
}

func TestConductorValidateRejectsAZeroField(t *testing.T) {
	base, err := sagten.Lookup("2 ACSR SPARROW")
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(*sagten.Conductor){
		"no code":     func(c *sagten.Conductor) { c.Code = " " },
		"no diameter": func(c *sagten.Conductor) { c.DiameterInches = 0 },
		"no weight":   func(c *sagten.Conductor) { c.WeightPerFoot = 0 },
		"no area":     func(c *sagten.Conductor) { c.AreaSquareInches = -1 },
		"no strength": func(c *sagten.Conductor) { c.RatedStrength = 0 },
		"no modulus":  func(c *sagten.Conductor) { c.ModulusPSI = 0 },
		"no thermal":  func(c *sagten.Conductor) { c.ThermalCoefficient = 0 },
	}
	for name, mutate := range cases {
		c := base
		mutate(&c)
		if err := c.Validate(); err == nil {
			t.Errorf("%s should have been rejected", name)
		}
	}
}

func TestCommunicationSplit(t *testing.T) {
	messenger, err := sagten.Lookup("3/8 EHS MESSENGER")
	if err != nil {
		t.Fatal(err)
	}
	if !messenger.IsCommunication() {
		t.Error("a messenger is a communication facility")
	}
	supply, err := sagten.Lookup("336 ACSR MERLIN")
	if err != nil {
		t.Fatal(err)
	}
	if supply.IsCommunication() {
		t.Error("acsr is a supply conductor")
	}
}

func TestTensionLimits(t *testing.T) {
	supply, err := sagten.Lookup("2 ACSR SPARROW")
	if err != nil {
		t.Fatal(err)
	}
	if supply.TensionLimit(false) != 0.25 || supply.TensionLimit(true) != 0.33 {
		t.Errorf("supply limits = %v %v", supply.TensionLimit(false), supply.TensionLimit(true))
	}
	if got := supply.MaxTension(false); got != 5926*0.25 {
		t.Errorf("supply everyday tension = %v", got)
	}
	comm, err := sagten.Lookup("1/4 EHS MESSENGER")
	if err != nil {
		t.Fatal(err)
	}
	if comm.TensionLimit(false) != 0.35 || comm.TensionLimit(true) != 0.50 {
		t.Errorf("messenger limits = %v %v", comm.TensionLimit(false), comm.TensionLimit(true))
	}
	if comm.MaxTension(true) <= comm.MaxTension(false) {
		t.Error("the loaded limit is the higher one")
	}
}
