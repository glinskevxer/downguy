package tests

import (
	"context"
	"errors"
	"testing"

	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/store"
)

func plantStore(t *testing.T) (*store.Plant, context.Context) {
	t.Helper()
	return store.NewPlant(openStore(t)), context.Background()
}

func TestSaveAndLoadAnAssembly(t *testing.T) {
	t.Parallel()
	p, ctx := plantStore(t)
	a := sampleAssembly()
	a.Anchors = []plant.Anchor{sampleAnchor()}
	if err := p.SaveAssembly(ctx, a); err != nil {
		t.Fatal(err)
	}
	got, err := p.Assembly(ctx, "NW-441829")
	if err != nil {
		t.Fatal(err)
	}
	if got.Pole.ID != "NW-441829" {
		t.Errorf("pole = %s", got.Pole.ID)
	}
	if len(got.Attachments) != 4 || got.Attachments[0].ID != "A1" {
		t.Errorf("attachments = %v", got.Attachments)
	}
	if len(got.Anchors) != 1 || len(got.Anchors[0].Guys) != 1 {
		t.Errorf("anchors = %v", got.Anchors)
	}
	if err := got.Validate(); err != nil {
		t.Errorf("what came back does not validate: %v", err)
	}
	gap, ok := got.SafetyZone()
	if !ok || gap.Tenths() != 85 {
		t.Errorf("safety zone = %s ok=%v", gap, ok)
	}
}

func TestSaveAssemblyRefusesAnInvalidOne(t *testing.T) {
	t.Parallel()
	p, ctx := plantStore(t)
	if err := p.SaveAssembly(ctx, nil); err == nil {
		t.Error("no assembly should be rejected")
	}
	bad := sampleAssembly()
	bad.Pole.Owner = ""
	if err := p.SaveAssembly(ctx, bad); err == nil {
		t.Error("an invalid pole should be rejected")
	}
	count, err := p.Poles().Count(ctx)
	if err != nil || count != 0 {
		t.Errorf("nothing should have been written, got %d %v", count, err)
	}
}

func TestLoadAMissingAssembly(t *testing.T) {
	t.Parallel()
	p, ctx := plantStore(t)
	if _, err := p.Assembly(ctx, "NW-000000"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("error = %v", err)
	}
}

func TestRepositoryAccessors(t *testing.T) {
	t.Parallel()
	p, _ := plantStore(t)
	if p.Poles() == nil || p.Attachments() == nil || p.Spans() == nil || p.Anchors() == nil {
		t.Error("every repository should be reachable")
	}
}

func loadRun(t *testing.T, p *store.Plant, ctx context.Context) {
	t.Helper()
	for _, id := range []plant.PoleID{"NW-441829", "NW-441830", "NW-441831", "NW-441832"} {
		pole := samplePole()
		pole.ID = id
		if err := p.Poles().Put(ctx, pole); err != nil {
			t.Fatal(err)
		}
	}
	for _, s := range []plant.SpanRecord{
		span("SP-1", "NW-441829", "NW-441830", 200),
		span("SP-2", "NW-441831", "NW-441830", 260),
		span("SP-3", "NW-441831", "NW-441832", 140),
	} {
		if err := p.Spans().Put(ctx, s); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLineWalksTheSectionFromEitherEnd(t *testing.T) {
	t.Parallel()
	p, ctx := plantStore(t)
	loadRun(t, p, ctx)
	fromEnd, err := p.Line(ctx, "LN-7", "NW-441829")
	if err != nil {
		t.Fatal(err)
	}
	if fromEnd.Len() != 3 {
		t.Fatalf("spans = %d", fromEnd.Len())
	}
	want := []plant.PoleID{"NW-441829", "NW-441830", "NW-441831", "NW-441832"}
	for i, id := range want {
		if fromEnd.Poles[i] != id {
			t.Errorf("pole %d = %s, want %s", i, fromEnd.Poles[i], id)
		}
	}
	fromMiddle, err := p.Line(ctx, "LN-7", "NW-441831")
	if err != nil {
		t.Fatal(err)
	}
	if fromMiddle.Len() != fromEnd.Len() {
		t.Errorf("starting in the middle found %d spans", fromMiddle.Len())
	}
	if fromMiddle.RulingSpan() != fromEnd.RulingSpan() {
		t.Errorf("the ruling span should not depend on where the walk started: %v against %v",
			fromMiddle.RulingSpan(), fromEnd.RulingSpan())
	}
}

func TestLineFlipsASpanRecordedTheOtherWayRound(t *testing.T) {
	t.Parallel()
	p, ctx := plantStore(t)
	loadRun(t, p, ctx)
	line, err := p.Line(ctx, "LN-7", "NW-441829")
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(line.Spans); i++ {
		if line.Spans[i].FromPole != line.Spans[i-1].ToPole {
			t.Fatalf("the run does not join up: %v", line.Spans)
		}
	}
}

func TestLineStopsAtATap(t *testing.T) {
	t.Parallel()
	p, ctx := plantStore(t)
	loadRun(t, p, ctx)
	tap := samplePole()
	tap.ID = "NW-441840"
	if err := p.Poles().Put(ctx, tap); err != nil {
		t.Fatal(err)
	}
	if err := p.Spans().Put(ctx, span("SP-4", "NW-441830", "NW-441840", 180)); err != nil {
		t.Fatal(err)
	}
	line, err := p.Line(ctx, "LN-7", "NW-441829")
	if err != nil {
		t.Fatal(err)
	}
	if line.Len() != 1 {
		t.Errorf("the walk should stop at the tap, got %d spans", line.Len())
	}
	if _, err := p.Line(ctx, "LN-8", "NW-441830"); err == nil {
		t.Error("a tap itself has no single section")
	}
}

func TestLineRejectsAPoleWithNoSpans(t *testing.T) {
	t.Parallel()
	p, ctx := plantStore(t)
	loadRun(t, p, ctx)
	lonely := samplePole()
	lonely.ID = "SE-1129043"
	if err := p.Poles().Put(ctx, lonely); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Line(ctx, "LN-9", "SE-1129043"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("error = %v", err)
	}
}
