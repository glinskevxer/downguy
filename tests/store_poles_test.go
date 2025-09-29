package tests

import (
	"context"
	"errors"
	"testing"

	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/store"
	"github.com/glinskevxer/downguy/internal/unit"
)

func poleRepo(t *testing.T) (*store.Poles, context.Context) {
	t.Helper()
	return store.NewPoles(openStore(t)), context.Background()
}

func otherPole() plant.Pole {
	p := samplePole()
	p.ID = "SE-1129043"
	p.Owner = "CASCADE POWER"
	p.Class = plant.Class2
	p.Length = 45
	return p
}

func TestPutAndGetAPole(t *testing.T) {
	repo, ctx := poleRepo(t)
	if err := repo.Put(ctx, samplePole()); err != nil {
		t.Fatal(err)
	}
	got, err := repo.Get(ctx, "NW-441829")
	if err != nil {
		t.Fatal(err)
	}
	want := samplePole()
	if got.ID != want.ID || got.Length != want.Length || got.Class != want.Class {
		t.Errorf("pole = %+v", got)
	}
	if got.Species != want.Species || got.Condition != want.Condition {
		t.Errorf("pole = %+v", got)
	}
	if got.SetDepth != want.SetDepth || got.Owner != want.Owner {
		t.Errorf("pole = %+v", got)
	}
	if got.InstalledIn != want.InstalledIn {
		t.Errorf("installed in %d", got.InstalledIn)
	}
	if err := got.Validate(); err != nil {
		t.Errorf("what came back does not validate: %v", err)
	}
}

func TestGetAMissingPole(t *testing.T) {
	repo, ctx := poleRepo(t)
	_, err := repo.Get(ctx, "NW-000000")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
	found, err := repo.Exists(ctx, "NW-000000")
	if err != nil || found {
		t.Errorf("Exists = %v %v", found, err)
	}
}

func TestPutRefusesAnInvalidPole(t *testing.T) {
	repo, ctx := poleRepo(t)
	bad := samplePole()
	bad.Owner = ""
	if err := repo.Put(ctx, bad); err == nil {
		t.Error("an invalid pole should not reach the table")
	}
	count, err := repo.Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("nothing should have been written, got %d", count)
	}
}

func TestPutReplacesAnEarlierRecord(t *testing.T) {
	repo, ctx := poleRepo(t)
	if err := repo.Put(ctx, samplePole()); err != nil {
		t.Fatal(err)
	}
	regraded := samplePole()
	regraded.Condition = plant.ConditionRestorable
	regraded.SetDepth = unit.Tenths(70)
	if err := repo.Put(ctx, regraded); err != nil {
		t.Fatal(err)
	}
	got, err := repo.Get(ctx, "NW-441829")
	if err != nil {
		t.Fatal(err)
	}
	if got.Condition != plant.ConditionRestorable || got.SetDepth.Tenths() != 70 {
		t.Errorf("the later record should win, got %+v", got)
	}
	count, err := repo.Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("count = %d", count)
	}
}

func TestDeleteAPole(t *testing.T) {
	repo, ctx := poleRepo(t)
	if err := repo.Put(ctx, samplePole()); err != nil {
		t.Fatal(err)
	}
	removed, err := repo.Delete(ctx, "NW-441829")
	if err != nil || !removed {
		t.Fatalf("delete = %v %v", removed, err)
	}
	removed, err = repo.Delete(ctx, "NW-441829")
	if err != nil {
		t.Fatal(err)
	}
	if removed {
		t.Error("deleting twice should report false")
	}
}

func TestListPolesByDistrictAndOwner(t *testing.T) {
	repo, ctx := poleRepo(t)
	second := samplePole()
	second.ID = "NW-441830"
	for _, p := range []plant.Pole{samplePole(), second, otherPole()} {
		if err := repo.Put(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	northwest, err := repo.InDistrict(ctx, "NW")
	if err != nil {
		t.Fatal(err)
	}
	if len(northwest) != 2 || northwest[0].ID != "NW-441829" {
		t.Errorf("district = %v", northwest)
	}
	southeast, err := repo.InDistrict(ctx, "SE")
	if err != nil {
		t.Fatal(err)
	}
	if len(southeast) != 1 {
		t.Errorf("district = %v", southeast)
	}
	if got, err := repo.InDistrict(ctx, "XX"); err != nil || len(got) != 0 {
		t.Errorf("an empty district = %v %v", got, err)
	}
	summit, err := repo.OwnedBy(ctx, "SUMMIT VALLEY ELECTRIC")
	if err != nil {
		t.Fatal(err)
	}
	if len(summit) != 2 {
		t.Errorf("owned = %v", summit)
	}
	all, err := repo.All(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 || all[0].ID != "NW-441829" || all[2].ID != "SE-1129043" {
		t.Errorf("all should come back in tag order, got %v", all)
	}
	count, err := repo.Count(ctx)
	if err != nil || count != 3 {
		t.Errorf("count = %d %v", count, err)
	}
}

func TestDistrictIsDerivedOnTheWayIn(t *testing.T) {
	repo, ctx := poleRepo(t)
	if err := repo.Put(ctx, otherPole()); err != nil {
		t.Fatal(err)
	}
	got, err := repo.InDistrict(ctx, "SE")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "SE-1129043" {
		t.Errorf("the district should come off the tag, got %v", got)
	}
}
