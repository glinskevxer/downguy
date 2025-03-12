package tests

import (
	"errors"
	"testing"

	"github.com/glinskevxer/downguy/internal/nesc"
	"github.com/glinskevxer/downguy/internal/plant"
)

func TestCircuitOfFacility(t *testing.T) {
	cases := []struct {
		kind  plant.Kind
		volts int
		want  nesc.Circuit
	}{
		{plant.KindCommCable, 0, nesc.CircuitCommunication},
		{plant.KindFiber, 0, nesc.CircuitCommunication},
		{plant.KindCommDrop, 0, nesc.CircuitCommunication},
		{plant.KindNeutral, 0, nesc.CircuitNeutral},
		{plant.KindSecondary, 240, nesc.CircuitSecondary},
		{plant.KindServiceDrop, 240, nesc.CircuitSecondary},
		{plant.KindPrimary, 7620, nesc.CircuitPrimary15kV},
		{plant.KindPrimary, 12470, nesc.CircuitPrimary15kV},
		{plant.KindPrimary, 19920, nesc.CircuitPrimary35kV},
	}
	for _, c := range cases {
		got, err := nesc.CircuitOf(c.kind, c.volts)
		if err != nil || got != c.want {
			t.Errorf("CircuitOf(%s,%d) = %q %v, want %q", c.kind, c.volts, got, err, c.want)
		}
	}
}

func TestCircuitOfRejectsWhatTheRuleDoesNotReach(t *testing.T) {
	if _, err := nesc.CircuitOf(plant.KindTransformer, 7620); !errors.Is(err, nesc.ErrNoClearance) {
		t.Errorf("a transformer does not run into the span: %v", err)
	}
	if _, err := nesc.CircuitOf(plant.KindRiser, 0); err == nil {
		t.Error("a riser does not run into the span")
	}
	if _, err := nesc.CircuitOf(plant.KindPrimary, 0); err == nil {
		t.Error("a primary at no volts has no class")
	}
	if _, err := nesc.CircuitOf(plant.KindPrimary, 69000); err == nil {
		t.Error("69 kV is above what this system carries")
	}
}

func TestGroundClearanceOverARoad(t *testing.T) {
	cases := []struct {
		c      nesc.Circuit
		tenths int
	}{
		{nesc.CircuitCommunication, 155},
		{nesc.CircuitNeutral, 155},
		{nesc.CircuitSecondary, 160},
		{nesc.CircuitPrimary15kV, 185},
		{nesc.CircuitPrimary35kV, 190},
	}
	for _, c := range cases {
		got, rule, err := nesc.GroundClearance(plant.CoverRoad, c.c)
		if err != nil {
			t.Fatalf("%s: %v", c.c, err)
		}
		if got.Tenths() != c.tenths {
			t.Errorf("%s over a road = %d tenths, want %d", c.c, got.Tenths(), c.tenths)
		}
		if rule != "NESC-232A" {
			t.Errorf("%s cites %s", c.c, rule)
		}
	}
}

func TestGroundClearanceRisesWithVoltage(t *testing.T) {
	for _, cover := range []plant.Cover{plant.CoverRoad, plant.CoverField, plant.CoverPedestrian, plant.CoverRailroad} {
		prev := -1
		for _, c := range nesc.Circuits() {
			got, _, err := nesc.GroundClearance(cover, c)
			if err != nil {
				t.Fatalf("%s over %s: %v", c, cover, err)
			}
			if got.Tenths() < prev {
				t.Errorf("%s over %s drops to %d from %d", c, cover, got.Tenths(), prev)
			}
			prev = got.Tenths()
		}
	}
}

func TestRailroadAndBuildingCiteTheirOwnRules(t *testing.T) {
	_, rule, err := nesc.GroundClearance(plant.CoverRailroad, nesc.CircuitPrimary15kV)
	if err != nil || rule != "NESC-232B" {
		t.Errorf("railroad rule = %q %v", rule, err)
	}
	_, rule, err = nesc.GroundClearance(plant.CoverBuilding, nesc.CircuitCommunication)
	if err != nil || rule != "NESC-234C" {
		t.Errorf("building rule = %q %v", rule, err)
	}
	for _, id := range []string{"NESC-232A", "NESC-232B", "NESC-234C"} {
		if _, err := nesc.LookupRule(id); err != nil {
			t.Errorf("the table cites %s, which is not in the catalogue", id)
		}
	}
}

func TestPedestrianIsTheLowestBarAndRailroadTheHighest(t *testing.T) {
	ped, _, err := nesc.GroundClearance(plant.CoverPedestrian, nesc.CircuitCommunication)
	if err != nil {
		t.Fatal(err)
	}
	road, _, err := nesc.GroundClearance(plant.CoverRoad, nesc.CircuitCommunication)
	if err != nil {
		t.Fatal(err)
	}
	rail, _, err := nesc.GroundClearance(plant.CoverRailroad, nesc.CircuitCommunication)
	if err != nil {
		t.Fatal(err)
	}
	if !(ped < road && road < rail) {
		t.Errorf("expected pedestrian %s under road %s under rail %s", ped, road, rail)
	}
}

func TestGroundClearanceRejectsWhatIsNotTabulated(t *testing.T) {
	if _, _, err := nesc.GroundClearance(plant.Cover("swamp"), nesc.CircuitCommunication); !errors.Is(err, nesc.ErrNoClearance) {
		t.Errorf("unknown cover error = %v", err)
	}
	if _, _, err := nesc.GroundClearance(plant.CoverRoad, nesc.Circuit("transmission")); !errors.Is(err, nesc.ErrNoClearance) {
		t.Errorf("unknown circuit error = %v", err)
	}
}

func TestEveryCoverIsTabulatedForEveryCircuit(t *testing.T) {
	covers := []plant.Cover{
		plant.CoverRoad, plant.CoverDriveway, plant.CoverAlley, plant.CoverParking,
		plant.CoverPedestrian, plant.CoverField, plant.CoverCultivated,
		plant.CoverWaterNoSail, plant.CoverRailroad, plant.CoverBuilding,
	}
	for _, cover := range covers {
		for _, c := range nesc.Circuits() {
			got, _, err := nesc.GroundClearance(cover, c)
			if err != nil {
				t.Errorf("%s over %s is not tabulated: %v", c, cover, err)
				continue
			}
			if got.Tenths() <= 0 {
				t.Errorf("%s over %s is %s", c, cover, got)
			}
		}
	}
}
