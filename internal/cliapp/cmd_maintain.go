package cliapp

import (
	"context"
	"io"

	"github.com/glinskevxer/downguy/internal/report"
	"github.com/glinskevxer/downguy/internal/store"
	"github.com/glinskevxer/downguy/internal/unit"
)

func init() {
	Register(Command{
		Name:    "maintain",
		Summary: "count what is on file, check it hangs together, and compact it",
		Usage:   maintainUsage(),
		Run:     runMaintain,
	})
}

func maintainUsage() string {
	f := newMaintainFlags(nil)
	return f.Usage("maintain [options]", "maintain [options]",
		"Counts every table, runs the integrity checks that a foreign key cannot\n"+
			"express, and with -compact reclaims the space a long run of deletions left\n"+
			"behind.\n\n"+
			"Exit code 1 means a check found something.")
}

func newMaintainFlags(out io.Writer) *Flags {
	f := NewFlags("maintain", out).WithJSON()
	f.Set().Bool("compact", false, "vacuum and re-analyse the database")
	return f
}

// MaintainJSON is what a maintenance run found.
type MaintainJSON struct {
	Schema      int             `json:"schema_version"`
	Pages       int             `json:"pages"`
	PagesBefore int             `json:"pages_before,omitempty"`
	Counts      store.Counts    `json:"counts"`
	Problems    []store.Problem `json:"problems,omitempty"`
}

func runMaintain(env *Env) int {
	f := newMaintainFlags(env.Stderr)
	if err := f.Parse(env.Args); err != nil {
		return CodeUsage
	}
	if len(f.Rest()) > 0 {
		return env.Fail(CodeUsage, "maintain takes no arguments")
	}
	ctx := context.Background()
	db, _, err := f.Open(ctx, env)
	if err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}
	defer func() { _ = db.Close() }()

	schema, err := store.CurrentVersion(ctx, db)
	if err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}
	counts, err := store.Tally(ctx, db)
	if err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}
	problems, err := store.Check(ctx, db)
	if err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}
	pages, err := store.PageCount(ctx, db)
	if err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}

	out := MaintainJSON{Schema: schema, Pages: pages, Counts: counts, Problems: problems}
	if flagBool(f, "compact") {
		out.PagesBefore = pages
		if err := store.Compact(ctx, db); err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
		if out.Pages, err = store.PageCount(ctx, db); err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
	}

	if f.JSON() {
		if err := report.WriteJSON(env.Stdout, out); err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
	} else {
		env.Print("schema version %d, %d pages", out.Schema, out.Pages)
		if out.PagesBefore > 0 {
			env.Print(" (was %d)", out.PagesBefore)
		}
		env.Println()
		env.Println()
		table := report.NewTable("table", "rows")
		table.RightAlign(1)
		for _, row := range [][2]any{
			{"poles", counts.Poles}, {"attachments", counts.Attachments},
			{"spans", counts.Spans}, {"anchors", counts.Anchors}, {"guys", counts.Guys},
			{"applications", counts.Applications}, {"surveys", counts.Surveys},
			{"rental bills", counts.Bills},
			{"change log", counts.Changes},
		} {
			table.Row(row[0].(string), unit.Whole(row[1].(int)))
		}
		if err := table.Render(env.Stdout); err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
		env.Println()
		if len(problems) == 0 {
			env.Println("the integrity checks found nothing")
		} else {
			found := report.NewTable("check", "detail")
			for _, p := range problems {
				found.Row(p.Check, p.Detail)
			}
			if err := found.Render(env.Stdout); err != nil {
				return env.Fail(CodeInternal, "%v", err)
			}
		}
	}
	if len(problems) > 0 {
		return CodeFindings
	}
	return CodeOK
}
