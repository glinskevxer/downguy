package cliapp

import (
	"context"
	"errors"
	"io"

	"github.com/glinskevxer/downguy/internal/clearance"
	"github.com/glinskevxer/downguy/internal/nesc"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/report"
	"github.com/glinskevxer/downguy/internal/store"
	"github.com/glinskevxer/downguy/internal/survey"
)

func init() {
	Register(Command{
		Name:    "clear",
		Summary: "run the clearance rules against a surveyed pole",
		Usage:   clearUsage(),
		Run:     runClear,
	})
	Register(Command{
		Name:    "rules",
		Summary: "list the code rules this system checks",
		Usage:   rulesUsage(),
		Run:     runRules,
	})
}

func clearUsage() string {
	f := newClearFlags(nil)
	return f.Usage("clear <tag> [options]", "clear <tag> [options]",
		"Applies the latest survey of the pole to its plant record and runs the\n"+
			"clearance rules against what the crew actually found.\n\n"+
			"Without a survey the command refuses rather than checking the paperwork,\n"+
			"because a clearance decision made against a record nobody has been out to\n"+
			"look at is not a clearance decision.\n\n"+
			"Exit code 1 means there are findings.")
}

func newClearFlags(out io.Writer) *Flags {
	f := NewFlags("clear", out).WithJSON()
	f.Set().String("applicant", "", "the attacher applying, so its own facilities can be told apart")
	f.Set().String("grade", "", "the grade of construction, overriding the configuration")
	f.Set().String("district", "", "the loading district, overriding the configuration")
	f.Set().Bool("allow-stale", false, "run against a survey that is past its shelf life")
	f.Set().String("as-of", "", "the date to judge staleness against, defaulting to the survey date")
	return f
}

func runClear(env *Env) int {
	f := newClearFlags(env.Stderr)
	if err := f.Parse(env.Args); err != nil {
		return CodeUsage
	}
	rest := f.Rest()
	if len(rest) != 1 {
		return env.Fail(CodeUsage, "clear takes exactly one tag")
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
	asOf, err := dateFlag(f, "as-of")
	if err != nil {
		return env.Fail(CodeUsage, "%v", err)
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
	if !asOf.IsZero() && sheet.Stale(asOf) && !flagBool(f, "allow-stale") {
		return env.Fail(CodeFindings,
			"the survey of %s is %d days old, past the %d day shelf life; pass -allow-stale to use it anyway",
			id, sheet.Age(asOf), survey.StaleAfterDays)
	}

	applied, err := survey.Apply(sheet, assembly)
	if err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}
	findings, err := clearance.New().CheckPole(clearance.PoleContext{
		Applied: applied, District: district, Grade: grade,
		Applicant: f.Set().Lookup("applicant").Value.String(),
	})
	if err != nil {
		return env.Fail(CodeUsage, "%v", err)
	}

	if f.JSON() {
		if err := report.WriteJSON(env.Stdout, report.FindingsJSON(findings)); err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
	} else {
		env.Print("%s\n\n", applied.Summary())
		if err := report.Findings(env.Stdout, findings); err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
	}
	if len(findings) == 0 {
		return CodeOK
	}
	return CodeFindings
}

func rulesUsage() string {
	f := NewFlags("rules", nil).WithJSON()
	f.Set().String("section", "", "list only one section of the code")
	return f.Usage("rules [options]", "rules [options]",
		"Lists the code rules this system checks, with what each one applies to and\n"+
			"what a failure means for the work.")
}

func runRules(env *Env) int {
	f := NewFlags("rules", env.Stderr).WithJSON()
	f.Set().String("section", "", "list only one section of the code")
	if err := f.Parse(env.Args); err != nil {
		return CodeUsage
	}
	if len(f.Rest()) > 0 {
		return env.Fail(CodeUsage, "rules takes no arguments")
	}
	section := f.Set().Lookup("section").Value.String()

	var rules []nesc.Rule
	if section != "" {
		rules = nesc.RulesInSection(section)
		if len(rules) == 0 {
			return env.Fail(CodeUsage, "no rules in section %q", section)
		}
	} else {
		for _, id := range nesc.RuleIDs() {
			rule, err := nesc.LookupRule(id)
			if err != nil {
				return env.Fail(CodeInternal, "%v", err)
			}
			rules = append(rules, rule)
		}
	}

	if f.JSON() {
		type row struct {
			ID       string `json:"id"`
			Section  string `json:"section"`
			Title    string `json:"title"`
			Severity string `json:"severity"`
			Applies  string `json:"applies_to"`
		}
		out := make([]row, 0, len(rules))
		for _, r := range rules {
			out = append(out, row{ID: r.ID, Section: r.Section, Title: r.Title,
				Severity: string(r.Severity), Applies: r.Applies})
		}
		if err := report.WriteJSON(env.Stdout, out); err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
		return CodeOK
	}
	table := report.NewTable("rule", "section", "severity", "title", "applies to")
	for _, r := range rules {
		table.Row(r.ID, r.Section, string(r.Severity), r.Title, r.Applies)
	}
	return renderOrFail(env, table)
}
