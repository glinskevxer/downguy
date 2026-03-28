package cliapp

import (
	"fmt"
	"io"
	"strings"

	"github.com/glinskevxer/downguy/internal/report"
	"github.com/glinskevxer/downguy/internal/sagten"
)

func init() {
	Register(Command{
		Name:    "sag",
		Summary: "work out a sag and tension run for a conductor",
		Usage:   sagUsage(),
		Run:     runSag,
	})
	Register(Command{
		Name:    "conductors",
		Summary: "list the catalogued conductors",
		Usage:   conductorsUsage(),
		Run:     runConductors,
	})
}

func sagUsage() string {
	f := newSagFlags(nil)
	return f.Usage("sag <conductor> [options]", "sag <conductor> [options]",
		"Works the conductor from its district design case back to a stringing\n"+
			"tension, then sags it at every temperature in the run and under each\n"+
			"district loading case.\n\n"+
			"Run downguy conductors for the catalogue.")
}

func newSagFlags(out io.Writer) *Flags {
	f := NewFlags("sag", out).WithJSON()
	f.Set().Int("span", 225, "the span to sag over, in feet")
	f.Set().Int("ruling-span", 0, "the ruling span of the section, defaulting to the span")
	return f.WithEngineering()
}

// SagRowJSON is one line of a sag table on the wire.
type SagRowJSON struct {
	Label       string  `json:"label"`
	Temperature int     `json:"temperature_f"`
	LoadPerFoot float64 `json:"load_lb_per_ft"`
	Tension     float64 `json:"tension_lb"`
	Sag         float64 `json:"sag_ft"`
	Iced        bool    `json:"iced"`
	OverTension bool    `json:"over_tension"`
}

// SagTableJSON is a whole run on the wire.
type SagTableJSON struct {
	Conductor  string       `json:"conductor"`
	District   string       `json:"design_district"`
	Span       float64      `json:"span_ft"`
	RulingSpan float64      `json:"ruling_span_ft"`
	Rows       []SagRowJSON `json:"rows"`
}

func runSag(env *Env) int {
	f := newSagFlags(env.Stderr)
	if err := f.Parse(env.Args); err != nil {
		return CodeUsage
	}
	rest := f.Rest()
	if len(rest) != 1 {
		return env.Fail(CodeUsage, "sag takes exactly one conductor code")
	}
	conductor, err := sagten.Lookup(rest[0])
	if err != nil {
		return env.Fail(CodeUsage, "%v", err)
	}
	engineering, err := f.Engineering(env)
	if err != nil {
		return env.Fail(CodeUsage, "%v", err)
	}
	district := engineering.District

	span := flagInt(f, "span", 225)
	ruling := flagInt(f, "ruling-span", 0)
	if ruling == 0 {
		ruling = span
	}
	table, err := sagten.BuildTable(conductor, district, float64(ruling), float64(span))
	if err != nil {
		return env.Fail(CodeUsage, "%v", err)
	}

	over := map[string]bool{}
	for _, row := range table.OverTension() {
		over[row.Label] = true
	}

	if f.JSON() {
		wire := SagTableJSON{
			Conductor: table.Conductor.Code, District: string(table.DesignDistrict),
			Span: table.SpanFeet, RulingSpan: table.RulingSpanFeet,
			Rows: make([]SagRowJSON, 0, len(table.Rows)),
		}
		for _, row := range table.Rows {
			wire.Rows = append(wire.Rows, SagRowJSON{
				Label: row.Label, Temperature: row.Temperature.Degrees(),
				LoadPerFoot: row.LoadPerFoot, Tension: row.Tension, Sag: row.SagFeet,
				Iced: row.Iced, OverTension: over[row.Label],
			})
		}
		if err := report.WriteJSON(env.Stdout, wire); err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
		if len(over) > 0 {
			return CodeFindings
		}
		return CodeOK
	}

	env.Print("%s designed for the %s district over %d ft, ruling span %d ft\n",
		table.Conductor.Code, table.DesignDistrict, span, ruling)
	env.Println()
	out := report.NewTable("condition", "temp F", "load lb/ft", "tension lb", "sag ft", "note")
	out.RightAlign(1, 2, 3, 4)
	for _, row := range table.Rows {
		note := ""
		if over[row.Label] {
			note = "over the working limit"
		}
		out.Row(row.Label, itoaCLI(row.Temperature.Degrees()),
			fmt.Sprintf("%.4f", row.LoadPerFoot),
			fmt.Sprintf("%.0f", row.Tension),
			fmt.Sprintf("%.2f", row.SagFeet), note)
	}
	if err := out.Render(env.Stdout); err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}
	if worst, ok := table.MaxSag(); ok {
		env.Print("\ndeepest sag %.2f ft at %s\n", worst.SagFeet, worst.Label)
	}
	if len(over) > 0 {
		return CodeFindings
	}
	return CodeOK
}

func conductorsUsage() string {
	f := NewFlags("conductors", nil).WithJSON()
	return f.Usage("conductors [options]", "conductors [options]",
		"Lists the conductors this system carries properties for, with the diameter,\n"+
			"weight and rated strength the sag arithmetic uses.")
}

func runConductors(env *Env) int {
	f := NewFlags("conductors", env.Stderr).WithJSON()
	if err := f.Parse(env.Args); err != nil {
		return CodeUsage
	}
	if len(f.Rest()) > 0 {
		return env.Fail(CodeUsage, "conductors takes no arguments")
	}
	codes := sagten.Codes()
	if f.JSON() {
		type row struct {
			Code     string  `json:"code"`
			Kind     string  `json:"kind"`
			Diameter float64 `json:"diameter_in"`
			Weight   float64 `json:"weight_lb_per_ft"`
			Rated    float64 `json:"rated_strength_lb"`
		}
		out := make([]row, 0, len(codes))
		for _, code := range codes {
			c, err := sagten.Lookup(code)
			if err != nil {
				return env.Fail(CodeInternal, "%v", err)
			}
			out = append(out, row{
				Code: c.Code, Kind: c.Kind, Diameter: c.DiameterInches,
				Weight: c.WeightPerFoot, Rated: c.RatedStrength,
			})
		}
		if err := report.WriteJSON(env.Stdout, out); err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
		return CodeOK
	}
	table := report.NewTable("code", "kind", "diameter in", "lb per ft", "rated lb")
	table.RightAlign(2, 3, 4)
	for _, code := range codes {
		c, err := sagten.Lookup(code)
		if err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
		table.Row(c.Code, c.Kind,
			fmt.Sprintf("%.3f", c.DiameterInches),
			fmt.Sprintf("%.4f", c.WeightPerFoot),
			fmt.Sprintf("%.0f", c.RatedStrength))
	}
	return renderOrFail(env, table)
}

// flagInt reads a registered integer flag, falling back to a default when it
// cannot be read at all.
func flagInt(f *Flags, name string, fallback int) int {
	value := f.Set().Lookup(name)
	if value == nil {
		return fallback
	}
	n := fallback
	if _, err := fmt.Sscanf(value.Value.String(), "%d", &n); err != nil {
		return fallback
	}
	return n
}

// flagBool reads a registered boolean flag.
func flagBool(f *Flags, name string) bool {
	value := f.Set().Lookup(name)
	return value != nil && strings.EqualFold(value.Value.String(), "true")
}

// renderOrFail writes a table and turns a write failure into an exit code.
func renderOrFail(env *Env, table *report.Table) int {
	if err := table.Render(env.Stdout); err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}
	return CodeOK
}
