package tests

import (
	"strings"
	"testing"

	"github.com/glinskevxer/downguy/internal/nesc"
	"github.com/glinskevxer/downguy/internal/sagten"
	"github.com/glinskevxer/downguy/internal/unit"
)

func sampleTable(t *testing.T) *sagten.Table {
	t.Helper()
	tbl, err := sagten.BuildTable(sparrow(t), nesc.Heavy, 225, 225)
	if err != nil {
		t.Fatal(err)
	}
	return tbl
}

func TestTemperatureRunCoversTheCheckedRange(t *testing.T) {
	run := sagten.TemperatureRun()
	if len(run) < 5 {
		t.Fatalf("run = %v", run)
	}
	if run[0] != 0 || run[len(run)-1] != unit.TempMaxOperating {
		t.Errorf("run should span 0 to the max operating temperature, got %v", run)
	}
	for i := 1; i < len(run); i++ {
		if run[i] <= run[i-1] {
			t.Fatalf("run is not increasing: %v", run)
		}
	}
}

func TestBuildTableHasARowPerConditionChecked(t *testing.T) {
	tbl := sampleTable(t)
	if len(tbl.Rows) != len(sagten.TemperatureRun())+len(nesc.Districts()) {
		t.Fatalf("rows = %d", len(tbl.Rows))
	}
	for _, r := range tbl.Rows {
		if r.Tension <= 0 || r.SagFeet <= 0 {
			t.Errorf("row %q = %+v", r.Label, r)
		}
	}
}

func TestBuildTableRejectsNonsense(t *testing.T) {
	c := sparrow(t)
	if _, err := sagten.BuildTable(c, nesc.Heavy, 225, 0); err == nil {
		t.Error("a zero span should be rejected")
	}
	if _, err := sagten.BuildTable(c, nesc.District("tropical"), 225, 225); err == nil {
		t.Error("an unknown district should be rejected")
	}
	if _, err := sagten.BuildTable(c, nesc.Heavy, 0, 225); err == nil {
		t.Error("a zero ruling span should be rejected")
	}
	bad := c
	bad.RatedStrength = 0
	if _, err := sagten.BuildTable(bad, nesc.Heavy, 225, 225); err == nil {
		t.Error("an invalid conductor should be rejected")
	}
}

func TestBareSagRisesWithTemperature(t *testing.T) {
	tbl := sampleTable(t)
	prev := 0.0
	for _, temp := range sagten.TemperatureRun() {
		r, ok := tbl.At(temp)
		if !ok {
			t.Fatalf("no bare row at %s", temp)
		}
		if r.SagFeet <= prev {
			t.Errorf("sag at %s is %v, not above the previous %v", temp, r.SagFeet, prev)
		}
		prev = r.SagFeet
	}
	if _, ok := tbl.At(unit.Temperature(45)); ok {
		t.Error("the run does not cover 45 degrees")
	}
}

func TestDistrictRowsCarryTheirIceFlag(t *testing.T) {
	tbl := sampleTable(t)
	heavy, ok := tbl.DistrictRow(nesc.Heavy)
	if !ok {
		t.Fatal("no heavy district row")
	}
	if !heavy.Iced {
		t.Error("the heavy district row is iced")
	}
	light, ok := tbl.DistrictRow(nesc.Light)
	if !ok {
		t.Fatal("no light district row")
	}
	if light.Iced {
		t.Error("the light district carries no ice")
	}
	if heavy.LoadPerFoot <= light.LoadPerFoot {
		t.Error("the heavy district loads the wire harder")
	}
	if _, ok := tbl.DistrictRow(nesc.District("tropical")); ok {
		t.Error("there is no tropical district row")
	}
}

func TestGoverningConditions(t *testing.T) {
	tbl := sampleTable(t)
	sagRow, ok := tbl.MaxSag()
	if !ok {
		t.Fatal("no max sag row")
	}
	for _, r := range tbl.Rows {
		if r.SagFeet > sagRow.SagFeet {
			t.Errorf("%q sags more than the reported worst", r.Label)
		}
	}
	tensionRow, ok := tbl.MaxTension()
	if !ok {
		t.Fatal("no max tension row")
	}
	for _, r := range tbl.Rows {
		if r.Tension > tensionRow.Tension {
			t.Errorf("%q pulls harder than the reported worst", r.Label)
		}
	}
	if !strings.Contains(tensionRow.Label, "district") {
		t.Errorf("the hardest pull should be a loaded case, got %q", tensionRow.Label)
	}
	empty := &sagten.Table{}
	if _, ok := empty.MaxSag(); ok {
		t.Error("an empty table has no worst row")
	}
	if _, ok := empty.MaxTension(); ok {
		t.Error("an empty table has no worst row")
	}
}

func TestOverTensionIsCleanForALineDesignedToItsOwnDistrict(t *testing.T) {
	tbl := sampleTable(t)
	for _, r := range tbl.OverTension() {
		t.Errorf("over tension: %s against a limit of %.1f", r, tbl.Conductor.MaxTension(r.Iced))
	}
	if tbl.DesignDistrict != nesc.Heavy {
		t.Errorf("table district = %s", tbl.DesignDistrict)
	}
}

func TestOverTensionCatchesWeatherTheLineWasNotDesignedFor(t *testing.T) {
	c := sparrow(t)
	tbl, err := sagten.BuildTable(c, nesc.Light, 225, 225)
	if err != nil {
		t.Fatal(err)
	}
	rows := tbl.OverTension()
	if len(rows) == 0 {
		t.Fatal("a line designed for the light district should run over tension under ice")
	}
	found := false
	for _, r := range rows {
		if r.Label == "heavy district" {
			found = true
		}
	}
	if !found {
		t.Errorf("the heavy district row should be over tension, got %v", rows)
	}
	for i := 1; i < len(rows); i++ {
		a := rows[i-1].Tension - c.MaxTension(rows[i-1].Iced)
		b := rows[i].Tension - c.MaxTension(rows[i].Iced)
		if b > a {
			t.Fatal("rows should come back worst first")
		}
	}
}

func TestRowFormatting(t *testing.T) {
	tbl := sampleTable(t)
	r, ok := tbl.At(unit.TempStringing)
	if !ok {
		t.Fatal("no row at stringing temperature")
	}
	if r.SagHeight().Tenths() <= 0 {
		t.Errorf("sag height = %s", r.SagHeight())
	}
	line := r.String()
	if !strings.Contains(line, "bare at 60 F") || !strings.Contains(line, "lb") {
		t.Errorf("row line = %q", line)
	}
}

func TestTheDesignRowIsNotItsOwnOverTension(t *testing.T) {
	c := sparrow(t)
	// The design case is solved back to a stringing tension and forward again,
	// so it lands a fraction of a pound either side of its own limit. Neither
	// side should be reported.
	for _, span := range []float64{150, 225, 320, 400} {
		tbl, err := sagten.BuildTable(c, nesc.Heavy, span, span)
		if err != nil {
			t.Fatalf("%v ft: %v", span, err)
		}
		row, ok := tbl.DistrictRow(nesc.Heavy)
		if !ok {
			t.Fatalf("%v ft: no heavy district row", span)
		}
		limit := c.MaxTension(true)
		if diff := row.Tension - limit; diff > sagten.TensionTolerance || diff < -sagten.TensionTolerance {
			t.Errorf("%v ft: the design row reads %.4f against a limit of %.4f", span, row.Tension, limit)
		}
		for _, over := range tbl.OverTension() {
			if over.Label == row.Label {
				t.Errorf("%v ft: the design row was reported over its own limit", span)
			}
		}
	}
}
