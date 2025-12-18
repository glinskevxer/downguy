package sagten

import (
	"fmt"
	"sort"

	"github.com/glinskevxer/downguy/internal/nesc"
	"github.com/glinskevxer/downguy/internal/unit"
)

// Row is one line of a sag table: a named condition with the tension the wire
// settles at and the sag that follows.
type Row struct {
	Label       string
	Temperature unit.Temperature
	LoadPerFoot float64
	Tension     float64
	SagFeet     float64
	Iced        bool
}

// Table is a sag run for one conductor on one span, worked out from one
// starting condition.
type Table struct {
	Conductor      Conductor
	DesignDistrict nesc.District
	RulingSpanFeet float64
	SpanFeet       float64
	Start          Condition
	Rows           []Row
}

// TemperatureRun is the set of temperatures a sag table is worked out at. It
// runs from the coldest the wire is checked at to the hottest it is allowed to
// operate at.
func TemperatureRun() []unit.Temperature {
	return []unit.Temperature{0, 15, 30, 60, 90, 120}
}

// BuildTable works out the sag and tension of a conductor at every temperature
// in the run under its bare load, and once more under each district loading
// case. The bare rows are what a stringing chart is read from; the loaded rows
// are what the strength check reads. The district argument is the one the line
// is designed to, which fixes the stringing tension every other row is worked
// out from; the loaded rows still cover all three, because a line does get the
// weather it was not designed for.
func BuildTable(c Conductor, d nesc.District, rulingSpanFeet, spanFeet float64) (*Table, error) {
	if spanFeet <= 0 {
		return nil, fmt.Errorf("%w: span is %v feet", ErrBadSag, spanFeet)
	}
	if rulingSpanFeet <= 0 {
		return nil, fmt.Errorf("%w: ruling span is %v feet", ErrBadSag, rulingSpanFeet)
	}
	start, err := InitialCondition(c, rulingSpanFeet, d)
	if err != nil {
		return nil, err
	}
	t := &Table{Conductor: c, DesignDistrict: d, RulingSpanFeet: rulingSpanFeet, SpanFeet: spanFeet, Start: start}
	for _, temp := range TemperatureRun() {
		sag, tension, err := SagAt(c, rulingSpanFeet, spanFeet, start, start.LoadPerFoot, temp)
		if err != nil {
			return nil, err
		}
		t.Rows = append(t.Rows, Row{
			Label:       fmt.Sprintf("bare at %s", temp),
			Temperature: temp, LoadPerFoot: start.LoadPerFoot,
			Tension: tension, SagFeet: sag,
		})
	}
	for _, d := range nesc.Districts() {
		load, temp, err := DistrictCondition(c, d)
		if err != nil {
			return nil, err
		}
		sag, tension, err := SagAt(c, rulingSpanFeet, spanFeet, start, load, temp)
		if err != nil {
			return nil, err
		}
		l, err := nesc.LoadingFor(d)
		if err != nil {
			return nil, err
		}
		t.Rows = append(t.Rows, Row{
			Label:       fmt.Sprintf("%s district", d),
			Temperature: temp, LoadPerFoot: load,
			Tension: tension, SagFeet: sag, Iced: l.RadialIce > 0,
		})
	}
	return t, nil
}

// At returns the bare row at a temperature and reports whether the run covers
// it.
func (t *Table) At(temp unit.Temperature) (Row, bool) {
	for _, r := range t.Rows {
		if r.Iced {
			continue
		}
		if r.Temperature == temp && r.LoadPerFoot == t.Start.LoadPerFoot {
			return r, true
		}
	}
	return Row{}, false
}

// DistrictRow returns the loaded row for a district and reports whether the
// table carries it.
func (t *Table) DistrictRow(d nesc.District) (Row, bool) {
	want := fmt.Sprintf("%s district", d)
	for _, r := range t.Rows {
		if r.Label == want {
			return r, true
		}
	}
	return Row{}, false
}

// MaxSag returns the row with the deepest sag anywhere in the table. It is the
// governing condition for a ground clearance check, because a clearance has to
// hold at the worst the wire ever does.
func (t *Table) MaxSag() (Row, bool) {
	if len(t.Rows) == 0 {
		return Row{}, false
	}
	worst := t.Rows[0]
	for _, r := range t.Rows[1:] {
		if r.SagFeet > worst.SagFeet {
			worst = r
		}
	}
	return worst, true
}

// MaxTension returns the row pulling hardest, which is the governing condition
// for a structure strength check.
func (t *Table) MaxTension() (Row, bool) {
	if len(t.Rows) == 0 {
		return Row{}, false
	}
	worst := t.Rows[0]
	for _, r := range t.Rows[1:] {
		if r.Tension > worst.Tension {
			worst = r
		}
	}
	return worst, true
}

// TensionTolerance is how far over its working limit a row may read before it
// counts as over tension, in pounds. The design case is reached by solving
// back to a stringing tension and then forward again, and the two passes of
// the change of state cubic do not land on the identical bit pattern, so the
// row the conductor was designed at can read a fraction of a pound over its
// own limit. Nobody strings a conductor to a tenth of a pound either.
const TensionTolerance = 1.0

// OverTension returns the rows whose tension is above the working limit for
// their condition, sorted by how far over they are. An empty result is a
// conductor strung within its limits everywhere in the run.
func (t *Table) OverTension() []Row {
	var out []Row
	for _, r := range t.Rows {
		limit := t.Conductor.MaxTension(r.Iced)
		if r.Tension > limit+TensionTolerance {
			out = append(out, r)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Tension-t.Conductor.MaxTension(out[i].Iced) >
			out[j].Tension-t.Conductor.MaxTension(out[j].Iced)
	})
	return out
}

// SagHeight returns a row's sag as a stored height.
func (r Row) SagHeight() unit.Height { return unit.Tenths(int(r.SagFeet*10 + 0.5)) }

// String prints the row the way a sag sheet lists it.
func (r Row) String() string {
	return fmt.Sprintf("%-20s %6s  %8.1f lb  %6.2f ft", r.Label, r.Temperature, r.Tension, r.SagFeet)
}
