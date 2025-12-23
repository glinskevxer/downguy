package cliapp

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/glinskevxer/downguy/internal/loading"
	"github.com/glinskevxer/downguy/internal/nesc"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/report"
	"github.com/glinskevxer/downguy/internal/store"
)

func init() {
	Register(Command{
		Name:    "load",
		Summary: "run the groundline loading check on a pole",
		Usage:   loadUsage(),
		Run:     runLoad,
	})
}

func loadUsage() string {
	f := newLoadFlags(nil)
	return f.Usage("load <tag> [options]", "load <tag> [options]",
		"Builds the wire list from the plant records, works out the groundline\n"+
			"moment under the district loading case, and compares it against the pole's\n"+
			"factored capacity.\n\n"+
			"Every span carrying facility needs a conductor assigned with -wire, which\n"+
			"takes an attachment id and a catalogue code. Repeat it once per facility,\n"+
			"or give a default for everything with -default-wire.\n\n"+
			"Exit code 1 means the pole does not carry what is on it.")
}

func newLoadFlags(out io.Writer) *Flags {
	f := NewFlags("load", out).WithJSON()
	f.Set().Var(&assignmentFlag{}, "wire", "assign a conductor, as attachment=code")
	f.Set().String("default-wire", "", "the conductor to assume for anything not assigned")
	f.Set().String("grade", "", "the grade of construction, overriding the configuration")
	f.Set().String("district", "", "the loading district, overriding the configuration")
	f.Set().String("line", "", "the line section id to report against")
	return f
}

// assignmentFlag collects repeated conductor assignments off the command line.
type assignmentFlag struct {
	pairs []string
}

func (a *assignmentFlag) String() string { return strings.Join(a.pairs, ",") }

func (a *assignmentFlag) Set(value string) error {
	if !strings.Contains(value, "=") {
		return errors.New("a wire assignment looks like attachment=code")
	}
	a.pairs = append(a.pairs, value)
	return nil
}

// Assignment turns the collected pairs into what the loading package wants.
func (a *assignmentFlag) Assignment() loading.Assignment {
	out := loading.Assignment{}
	for _, pair := range a.pairs {
		at := strings.Index(pair, "=")
		out[strings.TrimSpace(pair[:at])] = strings.TrimSpace(pair[at+1:])
	}
	return out
}

func runLoad(env *Env) int {
	f := newLoadFlags(env.Stderr)
	if err := f.Parse(env.Args); err != nil {
		return CodeUsage
	}
	rest := f.Rest()
	if len(rest) != 1 {
		return env.Fail(CodeUsage, "load takes exactly one tag")
	}
	id, err := plant.ParsePoleID(rest[0])
	if err != nil {
		return env.Fail(CodeUsage, "%v", err)
	}
	cfg, err := f.Config(env)
	if err != nil {
		return env.Fail(CodeUsage, "%v", err)
	}
	district, err := cfg.LoadingDistrict()
	if err != nil {
		return env.Fail(CodeUsage, "%v", err)
	}
	if override := f.Set().Lookup("district").Value.String(); override != "" {
		if district, err = nesc.ParseDistrict(override); err != nil {
			return env.Fail(CodeUsage, "%v", err)
		}
	}
	grade, err := cfg.ConstructionGrade()
	if err != nil {
		return env.Fail(CodeUsage, "%v", err)
	}
	if override := f.Set().Lookup("grade").Value.String(); override != "" {
		if grade, err = nesc.ParseGrade(override); err != nil {
			return env.Fail(CodeUsage, "%v", err)
		}
	}

	ctx := context.Background()
	db, _, err := f.Open(ctx, env)
	if err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}
	defer func() { _ = db.Close() }()

	loader := store.NewPlant(db)
	assembly, err := loader.Assembly(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		return env.Fail(CodeFindings, "%v", err)
	}
	if err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}

	lineID := f.Set().Lookup("line").Value.String()
	if lineID == "" {
		lineID = "LN-" + string(id)
	}
	line, err := loader.Line(ctx, lineID, id)
	if err != nil {
		return env.Fail(CodeFindings, "%v", err)
	}

	assignment := assignmentsFor(f, assembly)
	analysis, err := loading.AnalyseAssembly(assembly, line, assignment, district, grade)
	if err != nil {
		return env.Fail(CodeUsage, "%v", err)
	}

	if f.JSON() {
		if err := report.WriteJSON(env.Stdout, report.AnalysisToJSON(analysis)); err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
	} else if err := report.Analysis(env.Stdout, analysis); err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}
	if analysis.Passes() {
		return CodeOK
	}
	return CodeFindings
}

// assignmentsFor builds the conductor assignment from the flags, filling in
// anything unassigned from the default when one was given.
func assignmentsFor(f *Flags, a *plant.Assembly) loading.Assignment {
	var collected *assignmentFlag
	if value := f.Set().Lookup("wire"); value != nil {
		if typed, ok := value.Value.(*assignmentFlag); ok {
			collected = typed
		}
	}
	out := loading.Assignment{}
	if collected != nil {
		out = collected.Assignment()
	}
	fallback := ""
	if value := f.Set().Lookup("default-wire"); value != nil {
		fallback = value.Value.String()
	}
	if fallback == "" {
		return out
	}
	for _, att := range a.Attachments {
		if !att.Kind.IsSpanCarrying() {
			continue
		}
		if _, ok := out[att.ID]; !ok {
			out[att.ID] = fallback
		}
	}
	return out
}
