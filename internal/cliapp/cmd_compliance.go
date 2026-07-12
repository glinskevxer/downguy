package cliapp

import (
	"context"
	"errors"
	"io"

	"github.com/glinskevxer/downguy/internal/permit"
	"github.com/glinskevxer/downguy/internal/report"
	"github.com/glinskevxer/downguy/internal/store"
	"github.com/glinskevxer/downguy/internal/unit"
)

func init() {
	Register(Command{
		Name:    "compliance",
		Summary: "say where the time went on an application and whose delay it was",
		Usage:   complianceUsage(),
		Run:     runCompliance,
	})
}

func complianceUsage() string {
	f := newComplianceFlags(nil)
	return f.Usage("compliance [id] [options]", "compliance [id] [options]",
		"Walks an application's history against the regulated clocks and says how\n"+
			"long it sat at each step, which side was holding it, and where a clock ran\n"+
			"out.\n\n"+
			"With no id every live application is assessed and only the ones running\n"+
			"late are printed, which is the report somebody actually reads on a Monday.\n\n"+
			"Exit code 1 means a clock was missed.")
}

func newComplianceFlags(out io.Writer) *Flags {
	f := NewFlags("compliance", out).WithJSON()
	f.Set().String("as-of", "", "the date to assess against, as yyyy-mm-dd")
	f.Set().Bool("all", false, "print every interval rather than only the late ones")
	return f
}

// IntervalJSON is one stretch an application sat at one state.
type IntervalJSON struct {
	State    string `json:"state"`
	Holder   string `json:"held_by"`
	Entered  string `json:"entered"`
	Left     string `json:"left"`
	Open     bool   `json:"open"`
	Timed    bool   `json:"timed"`
	Allowed  int    `json:"allowed_days,omitempty"`
	Business bool   `json:"business_days,omitempty"`
	Due      string `json:"due,omitempty"`
	Used     int    `json:"used_days"`
	Overrun  int    `json:"overrun_days,omitempty"`
	Cite     string `json:"cite,omitempty"`
}

// ComplianceJSON is one application's timing picture.
type ComplianceJSON struct {
	Application  string         `json:"application"`
	AsOf         string         `json:"as_of"`
	OnTime       bool           `json:"on_time"`
	UtilityDays  int            `json:"days_with_utility"`
	AttacherDays int            `json:"days_with_attacher"`
	Intervals    []IntervalJSON `json:"intervals,omitempty"`
	Late         []IntervalJSON `json:"late,omitempty"`
}

func runCompliance(env *Env) int {
	f := newComplianceFlags(env.Stderr)
	if err := f.Parse(env.Args); err != nil {
		return CodeUsage
	}
	rest := f.Rest()
	if len(rest) > 1 {
		return env.Fail(CodeUsage, "compliance takes at most one id")
	}
	asOf, err := requiredDateFlag(f, "as-of")
	if err != nil {
		return env.Fail(CodeUsage, "%v", err)
	}
	holidays, ok := permit.StandardHolidays(asOf.Year())
	if !ok {
		return env.Fail(CodeUsage, "there is no holiday list for %d, so a business day clock cannot be counted", asOf.Year())
	}

	ctx := context.Background()
	db, _, err := f.Open(ctx, env)
	if err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}
	defer func() { _ = db.Close() }()
	repo := store.NewApplications(db)

	var apps []*permit.Application
	if len(rest) == 1 {
		app, err := repo.Get(ctx, rest[0])
		if errors.Is(err, store.ErrNotFound) {
			return env.Fail(CodeFindings, "%v", err)
		}
		if err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
		apps = []*permit.Application{app}
	} else {
		apps, err = repo.Live(ctx)
		if err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
	}

	everything := flagBool(f, "all") || len(rest) == 1
	var wire []ComplianceJSON
	late := 0
	for _, app := range apps {
		assessment, err := permit.Assess(app, asOf, holidays)
		if err != nil {
			return env.Fail(CodeUsage, "%v", err)
		}
		if !assessment.OnTime() {
			late++
		}
		if !everything && assessment.OnTime() {
			continue
		}
		row := ComplianceJSON{
			Application: assessment.Application, AsOf: assessment.AsOf.String(),
			OnTime:       assessment.OnTime(),
			UtilityDays:  assessment.DaysWith(permit.HolderUtility),
			AttacherDays: assessment.DaysWith(permit.HolderAttacher),
			Late:         intervalsToJSON(assessment.Late),
		}
		if everything {
			row.Intervals = intervalsToJSON(assessment.Intervals)
		}
		wire = append(wire, row)
	}

	if f.JSON() {
		if wire == nil {
			wire = []ComplianceJSON{}
		}
		if err := report.WriteJSON(env.Stdout, wire); err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
	} else if err := printCompliance(env, wire, everything); err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}
	if late > 0 {
		return CodeFindings
	}
	return CodeOK
}

func intervalsToJSON(intervals []permit.Interval) []IntervalJSON {
	out := make([]IntervalJSON, 0, len(intervals))
	for _, in := range intervals {
		row := IntervalJSON{
			State: string(in.State), Holder: string(in.Holder),
			Entered: in.Entered.String(), Left: in.Left.String(),
			Open: in.Open, Timed: in.Timed, Used: in.Used, Overrun: in.Overrun,
		}
		if in.Timed {
			row.Allowed = in.Deadline.Days
			row.Business = in.Deadline.Business
			row.Due = in.Due.String()
			row.Cite = in.Deadline.Cite
		}
		out = append(out, row)
	}
	return out
}

func printCompliance(env *Env, rows []ComplianceJSON, everything bool) error {
	if len(rows) == 0 {
		env.Println("every live application is inside its clocks")
		return nil
	}
	for i, row := range rows {
		if i > 0 {
			env.Println()
		}
		verdict := "on time"
		if !row.OnTime {
			verdict = "late"
		}
		env.Print("%s as of %s: %s, %d days with the utility and %d with the attacher\n",
			row.Application, row.AsOf, verdict, row.UtilityDays, row.AttacherDays)

		shown := row.Late
		if everything {
			shown = row.Intervals
		}
		table := report.NewTable("state", "held by", "entered", "left", "used", "allowed", "due", "over", "cite")
		table.RightAlign(4, 5, 7)
		for _, in := range shown {
			allowed, due, over := "", "", ""
			if in.Timed {
				allowed = unit.Whole(in.Allowed)
				if in.Business {
					allowed += " bd"
				}
				due = in.Due
			}
			if in.Overrun > 0 {
				over = unit.Whole(in.Overrun)
			}
			left := in.Left
			if in.Open {
				left = "open"
			}
			table.Row(in.State, in.Holder, in.Entered, left, unit.Whole(in.Used), allowed, due, over, in.Cite)
		}
		if err := table.Render(env.Stdout); err != nil {
			return err
		}
	}
	return nil
}
