package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/glinskevxer/downguy/internal/plant"
)

// Plant loads whole structures rather than rows. Every engineering package
// takes an assembly or a line section, so something has to put one together
// from four tables, and doing it here means no caller invents its own idea of
// what a complete pole looks like.
type Plant struct {
	poles       *Poles
	attachments *Attachments
	spans       *Spans
	anchors     *Anchors
}

// NewPlant builds a plant loader over a database.
func NewPlant(db *sql.DB) *Plant {
	return &Plant{
		poles:       NewPoles(db),
		attachments: NewAttachments(db),
		spans:       NewSpans(db),
		anchors:     NewAnchors(db),
	}
}

// Poles returns the pole repository.
func (p *Plant) Poles() *Poles { return p.poles }

// Attachments returns the attachment repository.
func (p *Plant) Attachments() *Attachments { return p.attachments }

// Spans returns the span repository.
func (p *Plant) Spans() *Spans { return p.spans }

// Anchors returns the anchor repository.
func (p *Plant) Anchors() *Anchors { return p.anchors }

// Assembly loads a pole with everything on it.
func (p *Plant) Assembly(ctx context.Context, id plant.PoleID) (*plant.Assembly, error) {
	pole, err := p.poles.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	atts, err := p.attachments.OnPole(ctx, id)
	if err != nil {
		return nil, err
	}
	anchors, err := p.anchors.AtPole(ctx, id)
	if err != nil {
		return nil, err
	}
	a := plant.NewAssembly(pole, atts)
	a.Anchors = anchors
	return a, nil
}

// SaveAssembly files a pole and everything on it. The pole goes first, because
// the attachments and anchors reference it.
func (p *Plant) SaveAssembly(ctx context.Context, a *plant.Assembly) error {
	if a == nil {
		return fmt.Errorf("%w: no assembly to save", ErrMigration)
	}
	if err := a.Validate(); err != nil {
		return err
	}
	if err := p.poles.Put(ctx, a.Pole); err != nil {
		return err
	}
	if err := p.attachments.PutAll(ctx, a.Attachments); err != nil {
		return err
	}
	for _, anchor := range a.Anchors {
		if err := p.anchors.Put(ctx, anchor); err != nil {
			return err
		}
	}
	return nil
}

// Line walks the plant from a starting pole and builds the section it sits in.
// A dead end has one span and the walk runs one way; an interior pole has two
// and the walk runs both ways from it, with the first half reversed so the
// section reads end to end. A pole carrying three spans is a tap rather than a
// run and has no single section, so it is refused rather than guessed at.
func (p *Plant) Line(ctx context.Context, id string, start plant.PoleID) (*plant.Line, error) {
	spans, err := p.spans.AtPole(ctx, start)
	if err != nil {
		return nil, err
	}
	switch {
	case len(spans) == 0:
		return nil, fmt.Errorf("%w: no spans at %s", ErrNotFound, start)
	case len(spans) > 2:
		return nil, fmt.Errorf("%w: %s carries %d spans, so it is a tap rather than a run",
			ErrNotFound, start, len(spans))
	}

	first, err := p.walk(ctx, start, spans[0])
	if err != nil {
		return nil, err
	}
	if len(spans) == 1 {
		return plant.NewLine(id, first)
	}
	second, err := p.walk(ctx, start, spans[1])
	if err != nil {
		return nil, err
	}
	ordered := append(reverseRun(first), second...)
	return plant.NewLine(id, ordered)
}

// walkLimit stops a walk that has found a loop the plant records should not
// contain, rather than running until something else notices.
const walkLimit = 500

// walk follows the line away from a pole along one span, returning the spans
// it crosses, each oriented in the direction of travel. It stops at a dead end
// and at a tap, because neither has a single way on.
func (p *Plant) walk(ctx context.Context, from plant.PoleID, first plant.SpanRecord) ([]plant.SpanRecord, error) {
	var out []plant.SpanRecord
	at := from
	current := first
	for len(out) < walkLimit {
		next, ok := current.Other(at)
		if !ok {
			break
		}
		out = append(out, orient(current, at))
		onward, err := p.spans.AtPole(ctx, next)
		if err != nil {
			return nil, err
		}
		var candidates []plant.SpanRecord
		for _, candidate := range onward {
			if candidate.ID == current.ID {
				continue
			}
			if _, ok := candidate.Other(next); !ok {
				continue
			}
			candidates = append(candidates, candidate)
		}
		if len(candidates) != 1 {
			break
		}
		at, current = next, candidates[0]
	}
	return out, nil
}

// orient turns a span record so that it runs away from the pole given.
func orient(s plant.SpanRecord, from plant.PoleID) plant.SpanRecord {
	if s.FromPole == from {
		return s
	}
	s.FromPole, s.ToPole = s.ToPole, s.FromPole
	s.Drop = -s.Drop
	return s
}

// reverseRun turns a run around: the spans come back in the opposite order,
// each pointing the other way.
func reverseRun(spans []plant.SpanRecord) []plant.SpanRecord {
	out := make([]plant.SpanRecord, 0, len(spans))
	for i := len(spans) - 1; i >= 0; i-- {
		out = append(out, orient(spans[i], spans[i].ToPole))
	}
	return out
}
