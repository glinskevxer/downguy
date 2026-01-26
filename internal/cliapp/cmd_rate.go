package cliapp

import (
	"fmt"
	"io"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/rental"
	"github.com/glinskevxer/downguy/internal/report"
)

func init() {
	Register(Command{
		Name:    "rate",
		Summary: "work out the annual attachment rate for a year",
		Usage:   rateUsage(),
		Run:     runRate,
	})
}

func rateUsage() string {
	f := newRateFlags(nil)
	return f.Usage("rate <year> [options]", "rate <year> [options]",
		"Works out what one attachment costs for a rate year under both formulas,\n"+
			"from the plant account and carrying charge filed for that year.\n\n"+
			"With -explain the working is printed line by line, which is what goes into\n"+
			"a rate filing and what an attacher asks for when it disputes a bill.")
}

func newRateFlags(out io.Writer) *Flags {
	f := NewFlags("rate", out).WithJSON()
	f.Set().Int("attachers", 0, "attaching entities per pole, overriding the configuration")
	f.Set().Bool("explain", false, "print the working behind the rate")
	f.Set().String("formula", "", "print only one formula")
	return f
}

// RateJSON is one worked rate on the wire.
type RateJSON struct {
	Year       int      `json:"year"`
	Formula    string   `json:"formula"`
	Attachers  int      `json:"attachers"`
	SpaceShare string   `json:"space_factor"`
	PoleCost   string   `json:"annual_pole_cost"`
	Annual     string   `json:"annual_rate"`
	Monthly    string   `json:"monthly_rate"`
	Working    []string `json:"working,omitempty"`
}

func runRate(env *Env) int {
	f := newRateFlags(env.Stderr)
	if err := f.Parse(env.Args); err != nil {
		return CodeUsage
	}
	rest := f.Rest()
	if len(rest) != 1 {
		return env.Fail(CodeUsage, "rate takes exactly one year")
	}
	year := 0
	if _, err := fmt.Sscanf(rest[0], "%d", &year); err != nil || year < 1900 {
		return env.Fail(CodeUsage, "%q is not a rate year", rest[0])
	}
	cfg, err := f.Config(env)
	if err != nil {
		return env.Fail(CodeUsage, "%v", err)
	}
	attachers := cfg.Attachers
	if override := flagInt(f, "attachers", 0); override != 0 {
		// The override goes through the same range check the configuration
		// does, rather than round it, because the telecom formula divides by
		// this number and nobody wants to find out what forty attachers on one
		// pole does to a rate filing.
		cfg.Attachers = override
		if err := cfg.Validate(); err != nil {
			return env.Fail(CodeUsage, "%v", err)
		}
		attachers = override
	}

	formulas := rental.Formulas()
	if only := f.Set().Lookup("formula").Value.String(); only != "" {
		formula, err := rental.ParseFormula(only)
		if err != nil {
			return env.Fail(CodeUsage, "%v", err)
		}
		formulas = []rental.Formula{formula}
	}
	explain := flagBool(f, "explain")

	var wire []RateJSON
	for _, formula := range formulas {
		rate, err := rental.Standard(calendar.RateYear(year), formula, attachers)
		if err != nil {
			return env.Fail(CodeUsage, "%v", err)
		}
		row := RateJSON{
			Year: year, Formula: string(formula), Attachers: attachers,
			SpaceShare: rate.Inputs.SpaceShare.String(),
			PoleCost:   rate.Inputs.PoleCost.String(),
			Annual:     rate.Annual.String(), Monthly: rate.Monthly().String(),
		}
		if explain {
			row.Working = rate.Explain()
		}
		wire = append(wire, row)
	}

	if f.JSON() {
		if err := report.WriteJSON(env.Stdout, wire); err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
		return CodeOK
	}

	table := report.NewTable("year", "formula", "attachers", "space factor", "pole cost", "annual", "monthly")
	table.RightAlign(2, 3, 4, 5, 6)
	for _, r := range wire {
		table.Row(itoaCLI(r.Year), r.Formula, itoaCLI(r.Attachers),
			r.SpaceShare, r.PoleCost, r.Annual, r.Monthly)
	}
	if err := table.Render(env.Stdout); err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}
	if !explain {
		return CodeOK
	}
	for _, r := range wire {
		env.Println()
		if err := report.Heading(env.Stdout, r.Formula+" formula"); err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
		for _, line := range r.Working {
			env.Print("  %s\n", line)
		}
	}
	return CodeOK
}
