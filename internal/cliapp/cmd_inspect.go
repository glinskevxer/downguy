package cliapp

import (
	"context"
	"io"
	"sort"

	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/report"
	"github.com/glinskevxer/downguy/internal/store"
)

func init() {
	Register(Command{
		Name:    "inspect",
		Summary: "list the poles a groundline inspection programme should visit next",
		Usage:   inspectUsage(),
		Run:     runInspect,
	})
}

func inspectUsage() string {
	f := newInspectFlags(nil)
	return f.Usage("inspect [options]", "inspect [options]",
		"Ranks the poles by how far through their planning life they are and how\n"+
			"long since anybody looked at them, which is how a programme with a budget\n"+
			"for four thousand inspections chooses them out of sixty thousand poles.\n\n"+
			"A pole whose recorded grade is better than its age suggests is flagged,\n"+
			"because that is the record most likely to be wrong and the one an\n"+
			"application will be approved against.\n\n"+
			"Exit code 1 means there are poles due.")
}

func newInspectFlags(out io.Writer) *Flags {
	f := NewFlags("inspect", out).WithJSON()
	f.Set().Int("year", 0, "the year to judge against")
	f.Set().String("district", "", "one plant district only, by tag prefix")
	f.Set().Int("limit", 0, "how many poles to list")
	f.Set().Bool("optimistic-only", false, "list only the poles graded better than their age suggests")
	return f
}

// InspectRowJSON is one pole on an inspection list.
type InspectRowJSON struct {
	Pole       string `json:"pole"`
	Species    string `json:"species"`
	Installed  int    `json:"installed_in"`
	Age        int    `json:"age"`
	Stage      string `json:"stage"`
	Interval   int    `json:"interval_years"`
	Recorded   string `json:"recorded_condition"`
	Expected   string `json:"expected_condition"`
	Optimistic bool   `json:"optimistic"`
	Due        bool   `json:"due"`
}

func runInspect(env *Env) int {
	f := newInspectFlags(env.Stderr)
	if err := f.Parse(env.Args); err != nil {
		return CodeUsage
	}
	if len(f.Rest()) > 0 {
		return env.Fail(CodeUsage, "inspect takes no arguments")
	}
	year := flagInt(f, "year", 0)
	if year < 1900 {
		return env.Fail(CodeUsage, "-year is required and takes the year to judge against")
	}
	limit := flagInt(f, "limit", 0)
	if limit < 0 {
		return env.Fail(CodeUsage, "-limit takes a whole number")
	}
	optimisticOnly := flagBool(f, "optimistic-only")

	ctx := context.Background()
	db, _, err := f.Open(ctx, env)
	if err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}
	defer func() { _ = db.Close() }()

	poles := store.NewPoles(db)
	var list []plant.Pole
	if district := stringFlag(f, "district"); district != "" {
		list, err = poles.InDistrict(ctx, district)
	} else {
		list, err = poles.All(ctx)
	}
	if err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}

	rows := make([]InspectRowJSON, 0, len(list))
	for _, pole := range list {
		stage := plant.StageOf(pole, year)
		interval, _ := plant.InspectionInterval(stage)
		// Nothing in the plant tables records the last inspection, so every
		// pole is treated as never inspected. That is the honest answer and it
		// is why the ranking is by stage rather than by overdue days.
		due, err := plant.DueForInspection(pole, 0, year)
		if err != nil {
			return env.Fail(CodeUsage, "%v", err)
		}
		row := InspectRowJSON{
			Pole: string(pole.ID), Species: string(pole.Species),
			Installed: pole.InstalledIn, Age: pole.Age(year),
			Stage: string(stage), Interval: interval,
			Recorded:   string(pole.Condition),
			Expected:   string(plant.ExpectedCondition(pole, year)),
			Optimistic: plant.GradeIsOptimistic(pole, year),
			Due:        due,
		}
		if optimisticOnly && !row.Optimistic {
			continue
		}
		rows = append(rows, row)
	}

	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.Optimistic != b.Optimistic {
			return a.Optimistic
		}
		if a.Stage != b.Stage {
			return plant.LifeStage(a.Stage).LaterThan(plant.LifeStage(b.Stage))
		}
		if a.Age != b.Age {
			return a.Age > b.Age
		}
		return a.Pole < b.Pole
	})
	if limit > 0 && limit < len(rows) {
		rows = rows[:limit]
	}

	if f.JSON() {
		if err := report.WriteJSON(env.Stdout, rows); err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
	} else {
		table := report.NewTable("pole", "species", "set", "age", "stage", "cycle", "recorded", "expected", "flag")
		table.RightAlign(2, 3, 5)
		for _, r := range rows {
			flag := ""
			if r.Optimistic {
				flag = "graded better than its age"
			}
			table.Row(r.Pole, r.Species, itoaCLI(r.Installed), itoaCLI(r.Age),
				r.Stage, itoaCLI(r.Interval), r.Recorded, r.Expected, flag)
		}
		if table.Len() == 0 {
			env.Println("nothing to inspect")
		} else if err := table.Render(env.Stdout); err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
	}
	due := 0
	for _, r := range rows {
		if r.Due {
			due++
		}
	}
	if due > 0 {
		return CodeFindings
	}
	return CodeOK
}
