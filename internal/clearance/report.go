package clearance

import (
	"fmt"
	"sort"
	"strings"

	"github.com/glinskevxer/downguy/internal/nesc"
)

// Report is the finished clearance check across every pole and span in an
// application. It is what the make ready planner reads and what goes back to
// the attacher.
type Report struct {
	Application string
	Applicant   string
	District    nesc.District
	Grade       nesc.Grade
	Poles       []string
	Spans       []string
	Attribution Attribution
	Skipped     []Skip
}

// Skip records a pole or span the engine could not check, and why. A silent
// omission is worse than a finding: an attacher who is told nothing about a
// pole assumes it passed.
type Skip struct {
	Where  string
	Reason string
}

// Builder accumulates checks across an application.
type Builder struct {
	engine    *Engine
	report    Report
	before    Findings
	after     Findings
	poleSeen  map[string]bool
	spanSeen  map[string]bool
	sealed    bool
	seenOrder []string
}

// NewBuilder starts a report for one application.
func NewBuilder(e *Engine, application, applicant string, d nesc.District, g nesc.Grade) *Builder {
	return &Builder{
		engine: e,
		report: Report{
			Application: application, Applicant: applicant,
			District: d, Grade: g,
		},
		poleSeen: map[string]bool{},
		spanSeen: map[string]bool{},
	}
}

// AddPole checks one pole twice: once as it stands and once with the
// applicant's facility on it. The caller supplies both contexts because only
// it knows what the applicant is proposing.
func (b *Builder) AddPole(before, after PoleContext) error {
	if b.sealed {
		return fmt.Errorf("%w: the report is already built", ErrBadContext)
	}
	beforeFindings, err := b.engine.CheckPole(before)
	if err != nil {
		b.report.Skipped = append(b.report.Skipped, Skip{Where: before.PoleID(), Reason: err.Error()})
		return err
	}
	afterFindings, err := b.engine.CheckPole(after)
	if err != nil {
		b.report.Skipped = append(b.report.Skipped, Skip{Where: after.PoleID(), Reason: err.Error()})
		return err
	}
	b.before = append(b.before, beforeFindings...)
	b.after = append(b.after, afterFindings...)
	b.notePole(after.PoleID())
	return nil
}

// AddSpan checks one span the same way.
func (b *Builder) AddSpan(before, after SpanContext) error {
	if b.sealed {
		return fmt.Errorf("%w: the report is already built", ErrBadContext)
	}
	beforeFindings, err := b.engine.CheckSpan(before)
	if err != nil {
		b.report.Skipped = append(b.report.Skipped, Skip{Where: before.Sheet.ID, Reason: err.Error()})
		return err
	}
	afterFindings, err := b.engine.CheckSpan(after)
	if err != nil {
		b.report.Skipped = append(b.report.Skipped, Skip{Where: after.Sheet.ID, Reason: err.Error()})
		return err
	}
	b.before = append(b.before, beforeFindings...)
	b.after = append(b.after, afterFindings...)
	b.noteSpan(after.Sheet.ID)
	return nil
}

// AddPrediction folds a predicted proposal into the after set. A prediction is
// always the applicant's, so it never appears in the before set.
func (b *Builder) AddPrediction(p *Prediction, span string) {
	if b.sealed || p == nil {
		return
	}
	if f, ok := p.Finding(span); ok {
		b.after = append(b.after, f)
	}
	b.noteSpan(span)
}

// Skip records a pole or span the caller decided not to check.
func (b *Builder) Skip(where, reason string) {
	b.report.Skipped = append(b.report.Skipped, Skip{Where: where, Reason: reason})
}

func (b *Builder) notePole(id string) {
	if id == "" || b.poleSeen[id] {
		return
	}
	b.poleSeen[id] = true
	b.report.Poles = append(b.report.Poles, id)
}

func (b *Builder) noteSpan(id string) {
	if id == "" || b.spanSeen[id] {
		return
	}
	b.spanSeen[id] = true
	b.report.Spans = append(b.report.Spans, id)
}

// Build attributes everything gathered and returns the finished report. It can
// be called once; a second call returns the same report rather than
// re-attributing a set that has already been classified.
func (b *Builder) Build() Report {
	if !b.sealed {
		b.report.Attribution = Classify(b.before, b.after)
		sort.Strings(b.report.Poles)
		sort.Strings(b.report.Spans)
		sort.SliceStable(b.report.Skipped, func(i, j int) bool {
			return b.report.Skipped[i].Where < b.report.Skipped[j].Where
		})
		b.sealed = true
	}
	return b.report
}

// Clean reports whether there is nothing for anybody to do.
func (r Report) Clean() bool { return len(r.Attribution.All()) == 0 }

// Blocking reports whether anything in the report stops the work.
func (r Report) Blocking() bool { return r.Attribution.All().Blocking() }

// ByOwner groups the findings by attacher, sorted by name.
func (r Report) ByOwner() map[string]Findings {
	out := map[string]Findings{}
	for _, f := range r.Attribution.All() {
		key := strings.TrimSpace(f.Owner)
		if key == "" {
			key = "UNASSIGNED"
		}
		out[key] = append(out[key], f)
	}
	for k := range out {
		out[k].Sort()
	}
	return out
}

// Summary writes the paragraph the report leads with.
func (r Report) Summary() string {
	c := r.Attribution.Counts()
	return fmt.Sprintf("%s for %s: %d poles, %d spans, %d caused, %d worsened, %d pre existing, %d resolved, %d skipped",
		r.Application, r.Applicant, len(r.Poles), len(r.Spans),
		c[BlameApplicant], c[BlameAggravated], c[BlamePreExisting],
		len(r.Attribution.Resolved), len(r.Skipped))
}
