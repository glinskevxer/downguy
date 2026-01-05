package cliapp

import (
	"context"
	"errors"
	"io"

	"github.com/glinskevxer/downguy/internal/clearance"
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
	f := NewFlags("makeready", out).WithJSON()
	f.Set().String("applicant", "", "the attacher applying")
	f.Set().String("application", "", "the application id to label the plan with")
	f.Set().String("grade", "", "the grade of construction, overriding the configuration")
	f.Set().String("district", "", "the loading district, overriding the configuration")
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
	applicant := f.Set().Lookup("applicant").Value.String()
	application := f.Set().Lookup("application").Value.String()
	if application == "" {
		application = "APP-" + string(id)
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
	sheet, err := store.NewSurveys(db).Latest(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		return env.Fail(CodeFindings, "%v", err)
	}
	if err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}
	applied, err := survey.Apply(sheet, assembly)
	if err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}

	engine := clearance.New()
	after, err := engine.CheckPole(clearance.PoleContext{
		Applied: applied, District: district, Grade: grade, Applicant: applicant,
	})
	if err != nil {
		return env.Fail(CodeUsage, "%v", err)
	}

	findings := after
	if applicant != "" {
		before, err := withoutApplicant(engine, applied, district, grade, applicant)
		if err != nil {
			return env.Fail(CodeUsage, "%v", err)
		}
		findings = clearance.Attribute(before, after)
	}

	planner := makeready.NewPlanner(application)
	planner.Know(applied.Assembly)
	plan := planner.Plan(findings)
	if err := plan.Validate(); err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}

	if f.JSON() {
		if err := report.WriteJSON(env.Stdout, report.PlanToJSON(plan)); err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
	} else {
		env.Print("%s on %s\n\n", application, id)
		if err := report.Work(env.Stdout, plan); err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
	}
	if plan.Empty() && len(plan.Unresolved) == 0 {
		return CodeOK
	}
	return CodeFindings
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
