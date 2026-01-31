package tests

import (
	"context"
	"errors"
	"testing"

	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/store"
	"github.com/glinskevxer/downguy/internal/unit"
)

func anchorRepo(t *testing.T) (*store.Anchors, context.Context) {
	t.Helper()
	db := openStore(t)
	ctx := context.Background()
	poles := store.NewPoles(db)
	for _, id := range []plant.PoleID{"NW-441829", "NW-441830"} {
		p := samplePole()
		p.ID = id
		if err := poles.Put(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	return store.NewAnchors(db), ctx
}

func twoGuyAnchor() plant.Anchor {
	a := sampleAnchor()
	a.Guys = append(a.Guys, plant.Guy{ID: "G2", Height: unit.Tenths(280), Size: "1/4 EHS"})
	return a
}

func TestPutAndGetAnAnchorWithItsGuys(t *testing.T) {
	t.Parallel()
	repo, ctx := anchorRepo(t)
	if err := repo.Put(ctx, twoGuyAnchor()); err != nil {
		t.Fatal(err)
	}
	got, err := repo.Get(ctx, "ANC-11")
	if err != nil {
		t.Fatal(err)
	}
	if got.Pole != "NW-441829" || got.Lead != 20 || got.Bearing != 270 {
		t.Errorf("anchor = %+v", got)
	}
	if len(got.Guys) != 2 {
		t.Fatalf("guys = %v", got.Guys)
	}
	if got.Guys[0].ID != "G1" || got.Guys[0].Height.Tenths() != 300 {
		t.Errorf("guys should come back highest first, got %v", got.Guys)
	}
	if got.Guys[1].Size != "1/4 EHS" {
		t.Errorf("guy = %+v", got.Guys[1])
	}
	if err := got.Validate(); err != nil {
		t.Errorf("what came back does not validate: %v", err)
	}
	capacity, err := got.HoldingCapacity()
	if err != nil || capacity <= 0 {
		t.Errorf("holding capacity = %v %v", capacity, err)
	}
}

func TestPutRefusesAnInvalidAnchor(t *testing.T) {
	t.Parallel()
	repo, ctx := anchorRepo(t)
	bad := sampleAnchor()
	bad.Guys[0].Size = "baling wire"
	if err := repo.Put(ctx, bad); err == nil {
		t.Error("an unstocked strand should not reach the table")
	}
	naked := sampleAnchor()
	naked.Guys = nil
	if err := repo.Put(ctx, naked); err == nil {
		t.Error("an anchor with no guys should be rejected")
	}
	count, err := repo.CountGuys(ctx)
	if err != nil || count != 0 {
		t.Errorf("guys = %d %v", count, err)
	}
}

func TestPutReplacesTheGuysOutright(t *testing.T) {
	t.Parallel()
	repo, ctx := anchorRepo(t)
	if err := repo.Put(ctx, twoGuyAnchor()); err != nil {
		t.Fatal(err)
	}
	restrung := sampleAnchor()
	restrung.Guys = []plant.Guy{{ID: "G3", Height: unit.Tenths(310), Size: "1/2 EHS"}}
	if err := repo.Put(ctx, restrung); err != nil {
		t.Fatal(err)
	}
	got, err := repo.Get(ctx, "ANC-11")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Guys) != 1 || got.Guys[0].ID != "G3" {
		t.Errorf("guys = %v", got.Guys)
	}
	count, err := repo.CountGuys(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("the old guys should be gone, got %d", count)
	}
}

func TestAnchorsAtAPole(t *testing.T) {
	t.Parallel()
	repo, ctx := anchorRepo(t)
	first := sampleAnchor()
	second := sampleAnchor()
	second.ID = "ANC-12"
	second.Bearing = 90
	elsewhere := sampleAnchor()
	elsewhere.ID = "ANC-20"
	elsewhere.Pole = "NW-441830"
	for _, a := range []plant.Anchor{first, second, elsewhere} {
		if err := repo.Put(ctx, a); err != nil {
			t.Fatal(err)
		}
	}
	got, err := repo.AtPole(ctx, "NW-441829")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "ANC-11" {
		t.Errorf("anchors = %v", got)
	}
	for _, a := range got {
		if len(a.Guys) == 0 {
			t.Errorf("%s came back with no guys", a.ID)
		}
	}
	if empty, err := repo.AtPole(ctx, "SE-1129043"); err != nil || len(empty) != 0 {
		t.Errorf("a pole with no anchors = %v %v", empty, err)
	}
}

func TestDeleteAnAnchorTakesItsGuys(t *testing.T) {
	t.Parallel()
	repo, ctx := anchorRepo(t)
	if err := repo.Put(ctx, twoGuyAnchor()); err != nil {
		t.Fatal(err)
	}
	removed, err := repo.Delete(ctx, "ANC-11")
	if err != nil || !removed {
		t.Fatalf("delete = %v %v", removed, err)
	}
	count, err := repo.CountGuys(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("the guys should have gone with it, got %d", count)
	}
	removed, err = repo.Delete(ctx, "ANC-11")
	if err != nil || removed {
		t.Errorf("deleting twice = %v %v", removed, err)
	}
	if _, err := repo.Get(ctx, "ANC-11"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("error = %v", err)
	}
}

func TestAnAnchorOnAPoleThatIsNotThereIsRefused(t *testing.T) {
	t.Parallel()
	repo, ctx := anchorRepo(t)
	orphan := sampleAnchor()
	orphan.Pole = "NW-999999"
	if err := repo.Put(ctx, orphan); err == nil {
		t.Error("an anchor on a pole that does not exist should be refused")
	}
	count, err := repo.CountGuys(ctx)
	if err != nil || count != 0 {
		t.Errorf("the rolled back write should have left nothing, got %d %v", count, err)
	}
}
