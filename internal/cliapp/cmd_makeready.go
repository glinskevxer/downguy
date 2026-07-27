package cliapp

import (
	"context"
	"database/sql"
	"fmt"
	"io"

	"github.com/glinskevxer/downguy/internal/clearance"
	"github.com/glinskevxer/downguy/internal/loading"
	"github.com/glinskevxer/downguy/internal/makeready"
	"github.com/glinskevxer/downguy/internal/nesc"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/report"
	"github.com/glinskevxer/downguy/internal/store"
	"github.com/glinskevxer/downguy/internal/survey"
)

func init() {
	Register(Command{
		Name:    "makeready",
		Summary: "plan the work a pole needs before an attachment can go up",
		Usage:   makereadyUsage(),
		Run:     runMakeready,
	})
}

func makereadyUsage() string {
	f := newMakereadyFlags(nil)
	return f.Usage("makeready <tag> [options]", "makeready <tag> [options]",
		"Runs the clearance rules against the latest survey, then works every\n"+
			"finding into the work that answers it, in the order a crew does it.\n\n"+
			"With -applicant the findings are attributed by running the pole twice, once\n"+
			"with the applicant's facilities and once without, so the plan says what the\n"+
			"applicant caused rather than what happens to be wrong with the pole.\n\n"+
			"Exit code 1 means there is work to do.")
}

func newMakereadyFlags(out io.Writer) *Flags {
	f := newEngineeringFlags("makeready", out)
	f.Set().String("applicant", "", "the attacher applying")
	f.Set().String("application", "", "the application id to label the plan with")
	f.Set().Bool("every-finding", false,
		"plan work for every finding, including the ones a facility is already being moved for")
	f.Set().Var(&assignmentFlag{}, "wire", "assign a conductor, as attachment=code")
	f.Set().String("default-wire", "", "the conductor to assume for anything not assigned")
	return f
}

func runMakeready(env *Env) int {
	f := newMakereadyFlags(env.Stderr)
	if err := f.Parse(env.Args); err != nil {
		return CodeUsage
	}
	rest := f.Rest()
	if len(rest) != 1 {
		return env.Fail(CodeUsage, "makeready takes exactly one tag")
	}
	id, err := plant.ParsePoleID(rest[0])
	if err != nil {
		return env.Fail(CodeUsage, "%v", err)
	}
	engineering, err := f.Engineering(env)
	if err != nil {
		return env.Fail(CodeUsage, "%v", err)
	}
	applicant := stringFlag(f, "applicant")
	application := stringFlag(f, "application")
	if application == "" {
		application = "APP-" + string(id)
	}

	ctx := context.Background()
	db, _, err := f.Open(ctx, env)
	if err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}
	defer func() { _ = db.Close() }()

	applied, err := loadSurveyed(ctx, db, id)
	if notFound(err) {
		return env.Fail(CodeFindings, "%v", err)
	}
	if err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}

	engine := clearance.New()
	after, err := engine.CheckPole(clearance.PoleContext{
		Applied: applied, District: engineering.District, Grade: engineering.Grade,
		Applicant: applicant,
	})
	if err != nil {
		return env.Fail(CodeUsage, "%v", err)
	}

	findings := after
	if applicant != "" {
		before, err := withoutApplicant(engine, applied, engineering.District, engineering.Grade, applicant)
		if err != nil {
			return env.Fail(CodeUsage, "%v", err)
		}
		findings = clearance.Attribute(before, after)
	}

	var hidden []clearance.Suppression
	if !flagBool(f, "every-finding") {
		findings, hidden = clearance.Suppress(findings)
	}

	planner := makeready.NewPlanner(application)
	planner.Know(applied.Assembly)
	plan := planner.Plan(findings)

	// A pole that will not carry what is on it is a finding no clearance rule
	// produces, and until now the plan simply did not mention it. The
	// structural check runs only when the caller has said what the wires are,
	// because without a conductor there is nothing to work a load out from.
	structural, code := assessStructure(ctx, env, f, db, applied, engineering)
	if code != CodeOK {
		return code
	}
	if structural != nil && !structural.Passes {
		plan.Items = makeready.Order(append(plan.Items, structural.Items...))
		findings = append(findings, structural.Findings...)
	}

	if err := plan.Validate(); err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}

	if f.JSON() {
		if err := report.WriteJSON(env.Stdout, report.PlanWithFindings(plan, findings)); err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
	} else {
		env.Print("%s on %s\n\n", application, id)
		if structural != nil {
			env.Print("%s\n\n", structural.Summary())
		}
		if err := report.Work(env.Stdout, plan); err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
		if len(hidden) > 0 {
			env.Print("\n%d findings are covered by work already planned:\n", len(hidden))
			for _, h := range hidden {
				env.Print("  %s %s\n", h.Hidden.RuleID, h.Reason)
			}
		}
	}
	if f.JSON() && len(hidden) > 0 {
		// The json shape carries the plan, so the suppressions go to standard
		// error rather than being dropped: a caller piping the plan onward
		// still gets told what was folded into it.
		for _, h := range hidden {
			fmt.Fprintf(env.Stderr, "covered: %s %s\n", h.Hidden.RuleID, h.Reason)
		}
	}
	if plan.Empty() && len(plan.Unresolved) == 0 {
		return CodeOK
	}
	return CodeFindings
}

// assessStructure runs the groundline check when the caller has told the
// command what the wires are. It returns nil when they have not, rather than
// guessing at a conductor: a load worked out from a wire nobody named is a
// number that looks authoritative and is not.
func assessStructure(ctx context.Context, env *Env, f *Flags, db *sql.DB,
	applied *survey.Applied, engineering Engineering) (*makeready.Structural, int) {
	assignment := assignmentsFor(f, applied.Assembly)
	if len(assignment) == 0 {
		return nil, CodeOK
	}
	loader := store.NewPlant(db)
	line, err := loader.Line(ctx, "LN-"+string(applied.Assembly.Pole.ID), applied.Assembly.Pole.ID)
	if err != nil {
		if notFound(err) {
			return nil, CodeOK
		}
		return nil, env.Fail(CodeInternal, "%v", err)
	}
	wires, err := loading.BuildWires(applied.Assembly, line, assignment, engineering.District)
	if err != nil {
		return nil, env.Fail(CodeUsage, "%v", err)
	}
	got, err := makeready.NewPlanner("structural").AssessStructure(
		applied.Assembly.Pole, wires, wires, nil, engineering.District, engineering.Grade)
	if err != nil {
		return nil, env.Fail(CodeUsage, "%v", err)
	}
	return got, CodeOK
}

// withoutApplicant runs the same checks on the pole as it would be with the
// applicant's facilities taken off, which is the comparison the attribution
// needs.
func withoutApplicant(engine *clearance.Engine, applied *survey.Applied, district nesc.District, grade nesc.Grade, applicant string) (clearance.Findings, error) {
	kept := applied.Assembly.Where(func(att plant.Attachment) bool {
		return !clearance.SameAttacher(att.Owner, applicant)
	})
	bare := &survey.Applied{
		Assembly: plant.NewAssembly(applied.Assembly.Pole, kept),
		Sheet:    applied.Sheet, Methods: applied.Methods,
	}
	return engine.CheckPole(clearance.PoleContext{
		Applied: bare, District: district, Grade: grade, Applicant: applicant,
	})
}
