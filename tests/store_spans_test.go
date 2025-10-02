package tests

import (
	"context"
	"errors"
	"testing"

	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/store"
	"github.com/glinskevxer/downguy/internal/unit"
)

func spanRepo(t *testing.T) (*store.Spans, context.Context) {
	t.Helper()
	db := openStore(t)
	ctx := context.Background()
	poles := store.NewPoles(db)
	for _, id := range []plant.PoleID{"NW-441829", "NW-441830", "NW-441831", "NW-441832"} {
		p := samplePole()
		p.ID = id
		if err := poles.Put(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	return store.NewSpans(db), ctx
}

func loadSpans(t *testing.T, repo *store.Spans, ctx context.Context) {
	t.Helper()
	road := span("SP-1", "NW-441829", "NW-441830", 200)
	road.Cover = plant.CoverRoad
	road.Drop = unit.Tenths(-8)
	for _, s := range []plant.SpanRecord{
		road,
		span("SP-2", "NW-441830", "NW-441831", 260),
		span("SP-3", "NW-441831", "NW-441832", 140),
	} {
		if err := repo.Put(ctx, s); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPutAndGetASpan(t *testing.T) {
	repo, ctx := spanRepo(t)
	loadSpans(t, repo, ctx)
	got, err := repo.Get(ctx, "SP-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.FromPole != "NW-441829" || got.ToPole != "NW-441830" {
		t.Errorf("span = %+v", got)
	}
	if got.Length.Feet() != 200 || got.Cover != plant.CoverRoad {
		t.Errorf("span = %+v", got)
	}
	if got.Drop.Tenths() != -8 {
		t.Errorf("drop = %s", got.Drop)
	}
	if err := got.Validate(); err != nil {
		t.Errorf("what came back does not validate: %v", err)
	}
	if _, err := repo.Get(ctx, "SP-0"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("error = %v", err)
	}
}

func TestPutRefusesAnInvalidSpan(t *testing.T) {
	repo, ctx := spanRepo(t)
	bad := span("SP-9", "NW-441829", "NW-441830", 4)
	if err := repo.Put(ctx, bad); err == nil {
		t.Error("a four foot span should not reach the table")
	}
	count, err := repo.Count(ctx)
	if err != nil || count != 0 {
		t.Errorf("count = %d %v", count, err)
	}
}

func TestSpansAtAPole(t *testing.T) {
	repo, ctx := spanRepo(t)
	loadSpans(t, repo, ctx)
	interior, err := repo.AtPole(ctx, "NW-441830")
	if err != nil {
		t.Fatal(err)
	}
	if len(interior) != 2 || interior[0].ID != "SP-1" {
		t.Errorf("interior = %v", interior)
	}
	end, err := repo.AtPole(ctx, "NW-441829")
	if err != nil {
		t.Fatal(err)
	}
	if len(end) != 1 {
		t.Errorf("dead end = %v", end)
	}
	if got, err := repo.AtPole(ctx, "SE-1129043"); err != nil || len(got) != 0 {
		t.Errorf("a pole with no spans = %v %v", got, err)
	}
}

func TestSpanBetweenIgnoresDirection(t *testing.T) {
	repo, ctx := spanRepo(t)
	loadSpans(t, repo, ctx)
	forward, err := repo.Between(ctx, "NW-441830", "NW-441831")
	if err != nil {
		t.Fatal(err)
	}
	backward, err := repo.Between(ctx, "NW-441831", "NW-441830")
	if err != nil {
		t.Fatal(err)
	}
	if forward.ID != "SP-2" || backward.ID != "SP-2" {
		t.Errorf("between = %s and %s", forward.ID, backward.ID)
	}
	if _, err := repo.Between(ctx, "NW-441829", "NW-441832"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("poles three apart share no span: %v", err)
	}
}

func TestSpansOverACoverAndByLength(t *testing.T) {
	repo, ctx := spanRepo(t)
	loadSpans(t, repo, ctx)
	roads, err := repo.OverCover(ctx, plant.CoverRoad)
	if err != nil {
		t.Fatal(err)
	}
	if len(roads) != 1 || roads[0].ID != "SP-1" {
		t.Errorf("roads = %v", roads)
	}
	if got, err := repo.OverCover(ctx, plant.CoverRailroad); err != nil || len(got) != 0 {
		t.Errorf("no railroad crossings, got %v %v", got, err)
	}
	long, err := repo.LongerThan(ctx, 180)
	if err != nil {
		t.Fatal(err)
	}
	if len(long) != 2 || long[0].ID != "SP-2" {
		t.Errorf("long spans should come back longest first, got %v", long)
	}
	if got, err := repo.LongerThan(ctx, 400); err != nil || len(got) != 0 {
		t.Errorf("nothing that long, got %v %v", got, err)
	}
}

func TestDeleteASpan(t *testing.T) {
	repo, ctx := spanRepo(t)
	loadSpans(t, repo, ctx)
	removed, err := repo.Delete(ctx, "SP-2")
	if err != nil || !removed {
		t.Fatalf("delete = %v %v", removed, err)
	}
	count, err := repo.Count(ctx)
	if err != nil || count != 2 {
		t.Errorf("count = %d %v", count, err)
	}
	removed, err = repo.Delete(ctx, "SP-2")
	if err != nil || removed {
		t.Errorf("deleting twice = %v %v", removed, err)
	}
}

func TestPutReplacesASpan(t *testing.T) {
	repo, ctx := spanRepo(t)
	loadSpans(t, repo, ctx)
	resurveyed := span("SP-1", "NW-441829", "NW-441830", 212)
	resurveyed.Cover = plant.CoverDriveway
	if err := repo.Put(ctx, resurveyed); err != nil {
		t.Fatal(err)
	}
	got, err := repo.Get(ctx, "SP-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Length.Feet() != 212 || got.Cover != plant.CoverDriveway {
		t.Errorf("the later record should win, got %+v", got)
	}
	count, err := repo.Count(ctx)
	if err != nil || count != 3 {
		t.Errorf("count = %d %v", count, err)
	}
}
