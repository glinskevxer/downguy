package cliapp

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/glinskevxer/downguy/internal/permit"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/report"
	"github.com/glinskevxer/downguy/internal/store"
	"github.com/glinskevxer/downguy/internal/unit"
)

func init() {
	Register(Command{
		Name:    "application",
		Summary: "open an application, move it on, and show where it has got to",
		Usage:   applicationUsage(),
		Run:     runApplication,
	})
}

// applicationActions are the verbs this command takes.
var applicationActions = []string{"open", "move", "show", "list"}

func applicationUsage() string {
	f := newApplicationFlags(nil)
	return f.Usage("application <action> [id] [options]", "application <action> [id] [options]",
		"Runs an application through its lifecycle. The actions are "+
			strings.Join(applicationActions, ", ")+".\n\n"+
			"  open   starts one in draft, with -applicant, -owner, -pole and -on\n"+
			"  move   records a transition, with -to, -on and -by\n"+
			"  show   prints one application with its whole history\n"+
			"  list   prints the applications, filtered by -state or -applicant\n\n"+
			"A transition the state machine does not allow is refused, and so is a date\n"+
			"earlier than the last event, because a history that goes backwards is worse\n"+
			"than no history.")
}

func newApplicationFlags(out io.Writer) *Flags {
	f := NewFlags("application", out).WithJSON()
	f.Set().String("applicant", "", "the attacher applying")
	f.Set().String("owner", "", "the pole owner")
	f.Set().Var(&listFlag{}, "pole", "a pole the application covers, repeatable")
	f.Set().String("on", "", "the date of the step, as yyyy-mm-dd")
	f.Set().String("by", "", "who recorded the step")
	f.Set().String("to", "", "the state to move to")
	f.Set().String("note", "", "a note against the step")
	f.Set().String("state", "", "filter a listing by state")
	return f
}

// ApplicationJSON is one application on the wire.
type ApplicationJSON struct {
	ID        string      `json:"id"`
	Applicant string      `json:"applicant"`
	Owner     string      `json:"owner"`
	State     string      `json:"state"`
	HeldBy    string      `json:"held_by"`
	Live      bool        `json:"live"`
	Opened    string      `json:"opened"`
	Poles     []string    `json:"poles"`
	History   []EventJSON `json:"history,omitempty"`
}

// EventJSON is one step of a history.
type EventJSON struct {
	From string `json:"from"`
	To   string `json:"to"`
	On   string `json:"on"`
	By   string `json:"by"`
	Note string `json:"note,omitempty"`
}

func runApplication(env *Env) int {
	f := newApplicationFlags(env.Stderr)
	if err := f.Parse(env.Args); err != nil {
		return CodeUsage
	}
	rest := f.Rest()
	if len(rest) == 0 {
		return env.Fail(CodeUsage, "application needs an action: %s", strings.Join(applicationActions, ", "))
	}
	action := strings.ToLower(rest[0])
	args := rest[1:]

	ctx := context.Background()
	db, _, err := f.Open(ctx, env)
	if err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}
	defer func() { _ = db.Close() }()
	repo := store.NewApplications(db)

	switch action {
	case "open":
		return applicationOpen(env, f, repo, ctx, args)
	case "move":
		return applicationMove(env, f, repo, ctx, args)
	case "show":
		return applicationShow(env, f, repo, ctx, args)
	case "list":
		return applicationList(env, f, repo, ctx, args)
	}
	return env.Fail(CodeUsage, "no such action %q, try one of %s", action, strings.Join(applicationActions, ", "))
}

func applicationOpen(env *Env, f *Flags, repo *store.Applications, ctx context.Context, args []string) int {
	if len(args) != 1 {
		return env.Fail(CodeUsage, "application open takes exactly one id")
	}
	on, err := requiredDateFlag(f, "on")
	if err != nil {
		return env.Fail(CodeUsage, "%v", err)
	}
	applicant := f.Set().Lookup("applicant").Value.String()
	owner := f.Set().Lookup("owner").Value.String()
	by := f.Set().Lookup("by").Value.String()
	if by == "" {
		return env.Fail(CodeUsage, "-by is required and names whoever is recording this")
	}

	var poles []plant.PoleID
	if value := f.Set().Lookup("pole"); value != nil {
		if typed, ok := value.Value.(*listFlag); ok {
			for _, tag := range typed.values {
				id, err := plant.ParsePoleID(tag)
				if err != nil {
					return env.Fail(CodeUsage, "%v", err)
				}
				poles = append(poles, id)
			}
		}
	}

	app, err := permit.New(args[0], applicant, owner, poles, on, by)
	if err != nil {
		return env.Fail(CodeUsage, "%v", err)
	}
	if err := repo.Put(ctx, app); err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}
	return printApplication(env, f, app, false)
}

func applicationMove(env *Env, f *Flags, repo *store.Applications, ctx context.Context, args []string) int {
	if len(args) != 1 {
		return env.Fail(CodeUsage, "application move takes exactly one id")
	}
	to, err := permit.ParseState(f.Set().Lookup("to").Value.String())
	if err != nil {
		return env.Fail(CodeUsage, "%v", err)
	}
	on, err := requiredDateFlag(f, "on")
	if err != nil {
		return env.Fail(CodeUsage, "%v", err)
	}
	by := f.Set().Lookup("by").Value.String()
	if by == "" {
		return env.Fail(CodeUsage, "-by is required and names whoever is recording this")
	}
	app, err := repo.Get(ctx, args[0])
	if errors.Is(err, store.ErrNotFound) {
		return env.Fail(CodeFindings, "%v", err)
	}
	if err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}
	if err := app.MoveTo(to, on, by, f.Set().Lookup("note").Value.String()); err != nil {
		return env.Fail(CodeUsage, "%v", err)
	}
	if err := repo.Put(ctx, app); err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}
	return printApplication(env, f, app, false)
}

func applicationShow(env *Env, f *Flags, repo *store.Applications, ctx context.Context, args []string) int {
	if len(args) != 1 {
		return env.Fail(CodeUsage, "application show takes exactly one id")
	}
	app, err := repo.Get(ctx, args[0])
	if errors.Is(err, store.ErrNotFound) {
		return env.Fail(CodeFindings, "%v", err)
	}
	if err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}
	return printApplication(env, f, app, true)
}

func applicationList(env *Env, f *Flags, repo *store.Applications, ctx context.Context, args []string) int {
	if len(args) != 0 {
		return env.Fail(CodeUsage, "application list takes no id")
	}
	var (
		apps []*permit.Application
		err  error
	)
	switch {
	case f.Set().Lookup("state").Value.String() != "":
		state, parseErr := permit.ParseState(f.Set().Lookup("state").Value.String())
		if parseErr != nil {
			return env.Fail(CodeUsage, "%v", parseErr)
		}
		apps, err = repo.InState(ctx, state)
	case f.Set().Lookup("applicant").Value.String() != "":
		apps, err = repo.ForApplicant(ctx, f.Set().Lookup("applicant").Value.String())
	default:
		apps, err = repo.Live(ctx)
	}
	if err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}

	if f.JSON() {
		wire := make([]ApplicationJSON, 0, len(apps))
		for _, app := range apps {
			wire = append(wire, applicationToJSON(app, false))
		}
		if err := report.WriteJSON(env.Stdout, wire); err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
		if len(apps) == 0 {
			return CodeFindings
		}
		return CodeOK
	}
	table := report.NewTable("id", "applicant", "state", "held by", "poles", "opened")
	table.RightAlign(4)
	for _, app := range apps {
		table.Row(app.ID, app.Applicant, string(app.State), string(app.HeldBy()),
			unit.Whole(app.PoleCount()), app.Opened().String())
	}
	if table.Len() == 0 {
		env.Println("no applications")
		return CodeFindings
	}
	return renderOrFail(env, table)
}

func applicationToJSON(app *permit.Application, withHistory bool) ApplicationJSON {
	out := ApplicationJSON{
		ID: app.ID, Applicant: app.Applicant, Owner: app.Owner,
		State: string(app.State), HeldBy: string(app.HeldBy()),
		Live: app.Live(), Opened: app.Opened().String(),
	}
	for _, p := range app.Poles {
		out.Poles = append(out.Poles, string(p))
	}
	if withHistory {
		for _, e := range app.History {
			out.History = append(out.History, EventJSON{
				From: string(e.From), To: string(e.To),
				On: e.On.String(), By: e.By, Note: e.Note,
			})
		}
	}
	return out
}

func printApplication(env *Env, f *Flags, app *permit.Application, withHistory bool) int {
	if f.JSON() {
		if err := report.WriteJSON(env.Stdout, applicationToJSON(app, withHistory)); err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
		return CodeOK
	}
	env.Println(app)
	env.Print("opened       %s\n", app.Opened())
	env.Print("poles        %s\n", strings.Join(applicationToJSON(app, false).Poles, ", "))
	if !withHistory {
		return CodeOK
	}
	env.Println()
	table := report.NewTable("from", "to", "on", "by", "note")
	for _, e := range app.History {
		table.Row(string(e.From), string(e.To), e.On.String(), e.By, e.Note)
	}
	return renderOrFail(env, table)
}
