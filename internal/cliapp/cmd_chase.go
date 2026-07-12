package cliapp

import (
	"context"
	"io"

	"github.com/glinskevxer/downguy/internal/permit"
	"github.com/glinskevxer/downguy/internal/report"
	"github.com/glinskevxer/downguy/internal/store"
	"github.com/glinskevxer/downguy/internal/unit"
)

func init() {
	Register(Command{
		Name:    "chase",
		Summary: "list the clocks about to run out and who to chase about them",
		Usage:   chaseUsage(),
		Run:     runChase,
	})
}

func chaseUsage() string {
	f := newChaseFlags(nil)
	return f.Usage("chase [options]", "chase [options]",
		"Walks every live application, works out how many days are left on the clock\n"+
			"it is sitting under, and lists them worst first.\n\n"+
			"With -now only the ones already blown or due today are listed, which is the\n"+
			"list somebody works through rather than reads.\n\n"+
			"Exit code 1 means something needs chasing today.")
}

func newChaseFlags(out io.Writer) *Flags {
	f := NewFlags("chase", out).WithJSON()
	f.Set().String("as-of", "", "the date to count against, as yyyy-mm-dd")
	f.Set().String("party", "", "one company only")
	f.Set().Bool("now", false, "only what is overdue or due today")
	return f
}

// ReminderJSON is one thing to chase, on the wire.
type ReminderJSON struct {
	Application string `json:"application"`
	State       string `json:"state"`
	HeldBy      string `json:"held_by"`
	Party       string `json:"party"`
	Due         string `json:"due"`
	Remaining   int    `json:"days_remaining"`
	Urgency     string `json:"urgency"`
	Cite        string `json:"cite"`
}

func runChase(env *Env) int {
	f := newChaseFlags(env.Stderr)
	if err := f.Parse(env.Args); err != nil {
		return CodeUsage
	}
	if len(f.Rest()) > 0 {
		return env.Fail(CodeUsage, "chase takes no arguments")
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

	apps, err := store.NewApplications(db).Live(ctx)
	if err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}
	reminders, err := permit.Remind(apps, asOf, holidays)
	if err != nil {
		return env.Fail(CodeUsage, "%v", err)
	}
	if party := stringFlag(f, "party"); party != "" {
		reminders = permit.ForParty(reminders, party)
	}
	urgent := permit.Chasing(reminders)
	if flagBool(f, "now") {
		reminders = urgent
	}

	if f.JSON() {
		wire := make([]ReminderJSON, 0, len(reminders))
		for _, r := range reminders {
			wire = append(wire, ReminderJSON{
				Application: r.Application, State: string(r.State), HeldBy: string(r.Holder),
				Party: r.Party, Due: r.Due.String(), Remaining: r.Remaining,
				Urgency: string(r.Urgency), Cite: r.Cite,
			})
		}
		if err := report.WriteJSON(env.Stdout, wire); err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
	} else if len(reminders) == 0 {
		env.Println("nothing to chase")
	} else {
		table := report.NewTable("application", "state", "held by", "party", "due", "days", "urgency", "cite")
		table.RightAlign(5)
		for _, r := range reminders {
			table.Row(r.Application, string(r.State), string(r.Holder), r.Party,
				r.Due.String(), unit.Whole(r.Remaining), string(r.Urgency), r.Cite)
		}
		if err := table.Render(env.Stdout); err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
		env.Print("\n%d of %d need chasing today\n", len(urgent), len(reminders))
	}
	if len(urgent) > 0 {
		return CodeFindings
	}
	return CodeOK
}
