package tests

import (
	"context"
	"errors"
	"testing"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/store"
)

func loadedStore(t *testing.T) (*store.Poles, *store.Attachments, context.Context) {
	t.Helper()
	db := openStore(t)
	ctx := context.Background()
	poles := store.NewPoles(db)
	if err := poles.Put(ctx, samplePole()); err != nil {
		t.Fatal(err)
	}
	second := samplePole()
	second.ID = "NW-441830"
	if err := poles.Put(ctx, second); err != nil {
		t.Fatal(err)
	}
	return poles, store.NewAttachments(db), ctx
}

func TestPutAndGetAnAttachment(t *testing.T) {
	_, repo, ctx := loadedStore(t)
	want := sampleAttachment()
	want.Face = plant.FaceStreet
	want.Note = "on the transfer list"
	if err := repo.Put(ctx, want); err != nil {
		t.Fatal(err)
	}
	got, err := repo.Get(ctx, want.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Pole != want.Pole || got.Owner != want.Owner || got.Kind != want.Kind {
		t.Errorf("attachment = %+v", got)
	}
	if got.Height != want.Height || got.Authorized != want.Authorized {
		t.Errorf("attachment = %+v", got)
	}
	if got.Face != plant.FaceStreet || got.Note != "on the transfer list" {
		t.Errorf("attachment = %+v", got)
	}
	if !got.InForce.From.Equal(want.InForce.From) || !got.InForce.To.IsZero() {
		t.Errorf("in force = %s", got.InForce)
	}
	if err := got.Validate(); err != nil {
		t.Errorf("what came back does not validate: %v", err)
	}
}

func TestAnAttachmentWithBothEndsOfItsRange(t *testing.T) {
	_, repo, ctx := loadedStore(t)
	att := sampleAttachment()
	att.InForce.To = calendar.MustNew(2024, 12, 31)
	if err := repo.Put(ctx, att); err != nil {
		t.Fatal(err)
	}
	got, err := repo.Get(ctx, att.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.InForce.To.String() != "2024-12-31" {
		t.Errorf("in force to %s", got.InForce.To)
	}
	if got.ActiveOn(calendar.MustNew(2025, 6, 1)) {
		t.Error("it came down in 2024")
	}
}

func TestPutRefusesAnInvalidAttachment(t *testing.T) {
	_, repo, ctx := loadedStore(t)
	bad := sampleAttachment()
	bad.Kind = plant.Kind("clothesline")
	if err := repo.Put(ctx, bad); err == nil {
		t.Error("an invalid attachment should not reach the table")
	}
	if err := repo.PutAll(ctx, []plant.Attachment{sampleAttachment(), bad}); err == nil {
		t.Error("a batch with a bad record should be refused whole")
	}
	count, err := repo.CountOnPole(ctx, "NW-441829")
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("nothing should have been written, got %d", count)
	}
}

func TestPutAllIsOneTransaction(t *testing.T) {
	_, repo, ctx := loadedStore(t)
	batch := []plant.Attachment{
		att("A1", "SUMMIT VALLEY ELECTRIC", plant.KindPrimary, 335),
		att("A2", "SUMMIT VALLEY ELECTRIC", plant.KindNeutral, 285),
		att("A3", "RIDGEPORT CABLEVISION", plant.KindCommCable, 200),
	}
	if err := repo.PutAll(ctx, batch); err != nil {
		t.Fatal(err)
	}
	count, err := repo.CountOnPole(ctx, "NW-441829")
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Errorf("count = %d", count)
	}
	orphan := att("A9", "MERIDIAN FIBER", plant.KindFiber, 174)
	orphan.Pole = "NW-999999"
	if err := repo.PutAll(ctx, append(batch, orphan)); err == nil {
		t.Error("an attachment on a pole that does not exist should be refused")
	}
	count, err = repo.CountOnPole(ctx, "NW-441829")
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Errorf("the rolled back batch should have changed nothing, got %d", count)
	}
}

func TestOnPoleComesBackHighestFirst(t *testing.T) {
	_, repo, ctx := loadedStore(t)
	if err := repo.PutAll(ctx, []plant.Attachment{
		att("A3", "RIDGEPORT CABLEVISION", plant.KindCommCable, 200),
		att("A1", "SUMMIT VALLEY ELECTRIC", plant.KindPrimary, 335),
		att("A2", "SUMMIT VALLEY ELECTRIC", plant.KindNeutral, 285),
	}); err != nil {
		t.Fatal(err)
	}
	got, err := repo.OnPole(ctx, "NW-441829")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].ID != "A1" || got[2].ID != "A3" {
		t.Errorf("attachments = %v", got)
	}
	if empty, err := repo.OnPole(ctx, "NW-441830"); err != nil || len(empty) != 0 {
		t.Errorf("a bare pole = %v %v", empty, err)
	}
}

func TestOwnedByAndUnauthorized(t *testing.T) {
	_, repo, ctx := loadedStore(t)
	orphan := att("A9", "UNIDENTIFIED", plant.KindCommCable, 174)
	orphan.Authorized = false
	second := att("B3", "RIDGEPORT CABLEVISION", plant.KindCommCable, 200)
	second.Pole = "NW-441830"
	if err := repo.PutAll(ctx, []plant.Attachment{
		att("A3", "RIDGEPORT CABLEVISION", plant.KindCommCable, 200),
		second, orphan,
	}); err != nil {
		t.Fatal(err)
	}
	owned, err := repo.OwnedBy(ctx, "RIDGEPORT CABLEVISION")
	if err != nil {
		t.Fatal(err)
	}
	if len(owned) != 2 || owned[0].Pole != "NW-441829" {
		t.Errorf("owned = %v", owned)
	}
	if got, err := repo.OwnedBy(ctx, "nobody"); err != nil || len(got) != 0 {
		t.Errorf("nobody owns nothing, got %v %v", got, err)
	}
	unauthorized, err := repo.Unauthorized(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(unauthorized) != 1 || unauthorized[0].ID != "A9" {
		t.Errorf("unauthorized = %v", unauthorized)
	}
}

func TestGetAndDeleteAMissingAttachment(t *testing.T) {
	_, repo, ctx := loadedStore(t)
	if _, err := repo.Get(ctx, "A0"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
	removed, err := repo.Delete(ctx, "A0")
	if err != nil || removed {
		t.Errorf("delete = %v %v", removed, err)
	}
	if err := repo.Put(ctx, sampleAttachment()); err != nil {
		t.Fatal(err)
	}
	removed, err = repo.Delete(ctx, sampleAttachment().ID)
	if err != nil || !removed {
		t.Errorf("delete = %v %v", removed, err)
	}
}
