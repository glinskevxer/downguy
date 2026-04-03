package cliapp

import (
	"context"
	"errors"
	"io"
	"sort"

	"github.com/glinskevxer/downguy/internal/clearance"
	"github.com/glinskevxer/downguy/internal/ingest"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/report"
	"github.com/glinskevxer/downguy/internal/store"
	"github.com/glinskevxer/downguy/internal/survey"
)

func init() {
	Register(Command{
		Name:    "sweep",
		Summary: "run the clearance rules across every surveyed pole",
		Usage:   sweepUsage(),
		Run:     runSweep,
	})
}

func sweepUsage() string {
	f := newSweepFlags(nil)
	return f.Usage("sweep [options]", "sweep [options]",
		"Runs the clearance rules against every pole with a survey on file, and\n"+
			"reports what it found by rule, by attacher and by pole.\n\n"+
			"A pole with no survey is skipped and counted rather than passed, because a\n"+
			"sweep that reports nothing wrong on a pole nobody has been out to is the\n"+
			"most dangerous output this system can produce.\n\n"+
			"Exit code 1 means there are findings.")
}

func newSweepFlags(out io.Writer) *Flags {
	f := newEngineeringFlags("sweep", out)
	f.Set().String("only-district", "", "sweep one plant district only, by tag prefix")
	f.Set().String("as-of", "", "the date to judge survey staleness against")
	f.Set().Bool("allow-stale", false, "include poles whose survey is past its shelf life")
	f.Set().Bool("csv", false, "write the findings as csv")
	return f
}

// SweepJSON is a whole sweep on the wire.
type SweepJSON struct {
	Poles    int                  `json:"poles_checked"`
	Skipped  []SkipJSON           `json:"skipped,omitempty"`
	ByRule   map[string]int       `json:"by_rule,omitempty"`
	ByOwner  map[string]int       `json:"by_owner,omitempty"`
	Findings []report.FindingJSON `json:"findings"`
}

// SkipJSON is one pole the sweep could not check.
type SkipJSON struct {
	Pole   string `json:"pole"`
	Reason string `json:"reason"`
}

func runSweep(env *Env) int {
	f := newSweepFlags(env.Stderr)
	if err := f.Parse(env.Args); err != nil {
		return CodeUsage
	}
	if len(f.Rest()) > 0 {
		return env.Fail(CodeUsage, "sweep takes no arguments")
	}
	engineering, err := f.Engineering(env)
	if err != nil {
		return env.Fail(CodeUsage, "%v", err)
	}
	asOf, err := dateFlag(f, "as-of")
	if err != nil {
		return env.Fail(CodeUsage, "%v", err)
	}
	allowStale := flagBool(f, "allow-stale")

	ctx := context.Background()
	db, _, err := f.Open(ctx, env)
	if err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}
	defer func() { _ = db.Close() }()

	loader := store.NewPlant(db)
	surveys := store.NewSurveys(db)

	var poles []plant.Pole
	if only := stringFlag(f, "only-district"); only != "" {
		poles, err = loader.Poles().InDistrict(ctx, only)
	} else {
		poles, err = loader.Poles().All(ctx)
	}
	if err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}

	engine := clearance.New()
	out := SweepJSON{ByRule: map[string]int{}, ByOwner: map[string]int{}}
	var findings clearance.Findings

	for _, pole := range poles {
		sheet, err := surveys.Latest(ctx, pole.ID)
		if errors.Is(err, store.ErrNotFound) {
			out.Skipped = append(out.Skipped, SkipJSON{Pole: string(pole.ID), Reason: "no survey on file"})
			continue
		}
		if err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
		if !asOf.IsZero() && sheet.Stale(asOf) && !allowStale {
			out.Skipped = append(out.Skipped, SkipJSON{
				Pole: string(pole.ID), Reason: "the survey is past its shelf life",
			})
			continue
		}
		assembly, err := loader.Assembly(ctx, pole.ID)
		if err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
		applied, err := survey.Apply(sheet, assembly)
		if err != nil {
			out.Skipped = append(out.Skipped, SkipJSON{Pole: string(pole.ID), Reason: err.Error()})
			continue
		}
		found, err := engine.CheckPole(clearance.PoleContext{
			Applied: applied, District: engineering.District, Grade: engineering.Grade,
		})
		if err != nil {
			out.Skipped = append(out.Skipped, SkipJSON{Pole: string(pole.ID), Reason: err.Error()})
			continue
		}
		out.Poles++
		findings = append(findings, found...)
	}

	findings.Sort()
	for _, finding := range findings {
		out.ByRule[finding.RuleID]++
		if finding.Owner != "" {
			out.ByOwner[finding.Owner]++
		}
	}
	out.Findings = report.FindingsJSON(findings)
	sort.SliceStable(out.Skipped, func(i, j int) bool { return out.Skipped[i].Pole < out.Skipped[j].Pole })

	switch {
	case flagBool(f, "csv"):
		if err := ingest.WriteFindings(env.Stdout, findings); err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
	case f.JSON():
		if err := report.WriteJSON(env.Stdout, out); err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
	default:
		if err := printSweep(env, out, findings); err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
	}
	if len(findings) > 0 {
		return CodeFindings
	}
	return CodeOK
}

func printSweep(env *Env, out SweepJSON, findings clearance.Findings) error {
	env.Print("%d poles checked, %d skipped, %d findings\n\n",
		out.Poles, len(out.Skipped), len(findings))

	if len(findings) > 0 {
		if err := report.Heading(env.Stdout, "by rule"); err != nil {
			return err
		}
		byRule := report.NewTable("rule", "findings")
		byRule.RightAlign(1)
		for _, id := range sortedKeys(out.ByRule) {
			byRule.Row(id, itoaCLI(out.ByRule[id]))
		}
		if err := byRule.Render(env.Stdout); err != nil {
			return err
		}

		env.Println()
		byOwner := map[string]string{}
		for owner, count := range out.ByOwner {
			byOwner[owner] = itoaCLI(count)
		}
		if err := report.OwnerTotals(env.Stdout, "by attacher", byOwner); err != nil {
			return err
		}

		env.Println()
		if err := report.Heading(env.Stdout, "findings"); err != nil {
			return err
		}
		if err := report.Findings(env.Stdout, findings); err != nil {
			return err
		}
	}

	if len(out.Skipped) == 0 {
		return nil
	}
	env.Println()
	if err := report.Heading(env.Stdout, "skipped"); err != nil {
		return err
	}
	skipped := report.NewTable("pole", "why")
	for _, s := range out.Skipped {
		skipped.Row(s.Pole, s.Reason)
	}
	return skipped.Render(env.Stdout)
}

func sortedKeys(counts map[string]int) []string {
	out := make([]string, 0, len(counts))
	for key := range counts {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}
