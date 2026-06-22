package cliapp

import (
	"context"
	"io"
	"strings"

	"github.com/glinskevxer/downguy/internal/report"
	"github.com/glinskevxer/downguy/internal/store"
)

func init() {
	Register(Command{
		Name:    "history",
		Summary: "read the change log for a record, a person or a date",
		Usage:   historyUsage(),
		Run:     runHistory,
	})
}

func historyUsage() string {
	f := newHistoryFlags(nil)
	return f.Usage("history [options]", "history [options]",
		"Reads the append only change log. Ask for one record with -subject and -id,\n"+
			"for one person's work with -actor, or for everything since a day with\n"+
			"-since.\n\n"+
			"Exit code 1 means there is nothing recorded for what was asked about,\n"+
			"which on a record somebody expected to see is itself the answer.")
}

func newHistoryFlags(out io.Writer) *Flags {
	f := NewFlags("history", out).WithJSON()
	f.Set().String("subject", "", "the kind of record, one of "+subjectList())
	f.Set().String("id", "", "the record to read the history of")
	f.Set().String("actor", "", "one person's work")
	f.Set().String("since", "", "everything on or after a day, as yyyy-mm-dd")
	return f
}

func subjectList() string {
	names := make([]string, 0, len(store.Subjects()))
	for _, s := range store.Subjects() {
		names = append(names, string(s))
	}
	return strings.Join(names, ", ")
}

// ChangeJSON is one change log entry on the wire.
type ChangeJSON struct {
	Seq     int64  `json:"seq"`
	At      string `json:"at"`
	Subject string `json:"subject"`
	ID      string `json:"id"`
	Action  string `json:"action"`
	Actor   string `json:"actor"`
	Detail  string `json:"detail,omitempty"`
}

func runHistory(env *Env) int {
	f := newHistoryFlags(env.Stderr)
	if err := f.Parse(env.Args); err != nil {
		return CodeUsage
	}
	if len(f.Rest()) > 0 {
		return env.Fail(CodeUsage, "history takes no arguments")
	}
	subject := stringFlag(f, "subject")
	id := stringFlag(f, "id")
	actor := stringFlag(f, "actor")
	since, err := dateFlag(f, "since")
	if err != nil {
		return env.Fail(CodeUsage, "%v", err)
	}

	chosen := 0
	if subject != "" || id != "" {
		chosen++
	}
	if actor != "" {
		chosen++
	}
	if !since.IsZero() {
		chosen++
	}
	switch {
	case chosen == 0:
		return env.Fail(CodeUsage, "history needs one of -subject with -id, -actor, or -since")
	case chosen > 1:
		return env.Fail(CodeUsage, "history takes one of -subject with -id, -actor, or -since")
	}
	if (subject == "") != (id == "") {
		return env.Fail(CodeUsage, "-subject and -id go together")
	}
	if subject != "" && !store.KnownSubject(store.Subject(subject)) {
		return env.Fail(CodeUsage, "no such subject %q, try one of %s", subject, subjectList())
	}

	ctx := context.Background()
	db, _, err := f.Open(ctx, env)
	if err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}
	defer func() { _ = db.Close() }()
	log := store.NewChangeLog(db)

	var changes []store.Change
	switch {
	case subject != "":
		changes, err = log.For(ctx, store.Subject(subject), id)
	case actor != "":
		changes, err = log.ByActor(ctx, actor)
	default:
		changes, err = log.Since(ctx, since)
	}
	if err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}

	if f.JSON() {
		wire := make([]ChangeJSON, 0, len(changes))
		for _, c := range changes {
			wire = append(wire, ChangeJSON{
				Seq: c.Seq, At: c.At.String(), Subject: string(c.Subject),
				ID: c.ID, Action: string(c.Action), Actor: c.Actor, Detail: c.Detail,
			})
		}
		if err := report.WriteJSON(env.Stdout, wire); err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
	} else if len(changes) == 0 {
		env.Println("nothing recorded")
	} else {
		table := report.NewTable("seq", "on", "subject", "record", "action", "by", "detail")
		table.RightAlign(0)
		for _, c := range changes {
			table.Row(itoaCLI(int(c.Seq)), c.At.String(), string(c.Subject),
				c.ID, string(c.Action), c.Actor, c.Detail)
		}
		if err := table.Render(env.Stdout); err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
	}
	if len(changes) == 0 {
		return CodeFindings
	}
	return CodeOK
}
