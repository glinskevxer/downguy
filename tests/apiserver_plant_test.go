package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/glinskevxer/downguy/internal/apiserver"
	"github.com/glinskevxer/downguy/internal/config"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/store"
)

func plantAPI(t *testing.T) http.Handler {
	t.Helper()
	db := openStore(t)
	ctx := context.Background()
	loader := store.NewPlant(db)

	a := sampleAssembly()
	a.Anchors = []plant.Anchor{sampleAnchor()}
	if err := loader.SaveAssembly(ctx, a); err != nil {
		t.Fatal(err)
	}
	second := samplePole()
	second.ID = "NW-441830"
	if err := loader.Poles().Put(ctx, second); err != nil {
		t.Fatal(err)
	}
	third := samplePole()
	third.ID = "SE-1129043"
	third.Owner = "CASCADE POWER"
	if err := loader.Poles().Put(ctx, third); err != nil {
		t.Fatal(err)
	}
	if err := loader.Spans().Put(ctx, span("SP-1", "NW-441829", "NW-441830", 200)); err != nil {
		t.Fatal(err)
	}
	s, err := apiserver.New(db, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	return s.RegisterPlant().Handler()
}

func TestPoleIndex(t *testing.T) {
	t.Parallel()
	rec, body := get(t, plantAPI(t), "/poles")
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", rec.Code, body)
	}
	var back apiserver.PoleIndexJSON
	if err := json.Unmarshal(body, &back); err != nil {
		t.Fatal(err)
	}
	if back.Count != 3 || len(back.Poles) != 3 {
		t.Fatalf("index = %+v", back)
	}
	if back.Poles[0].ID != "NW-441829" || back.Poles[0].District != "NW" {
		t.Errorf("pole = %+v", back.Poles[0])
	}
	if back.Poles[0].Attachments != 4 {
		t.Errorf("attachment count = %d", back.Poles[0].Attachments)
	}
	if back.Poles[2].ID != "SE-1129043" {
		t.Errorf("poles should come back in tag order, got %v", back.Poles)
	}
}

func TestPoleIndexFilters(t *testing.T) {
	t.Parallel()
	h := plantAPI(t)
	rec, body := get(t, h, "/poles?district=nw")
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d", rec.Code)
	}
	var byDistrict apiserver.PoleIndexJSON
	if err := json.Unmarshal(body, &byDistrict); err != nil {
		t.Fatal(err)
	}
	if byDistrict.Count != 2 {
		t.Errorf("district = %+v", byDistrict)
	}

	rec, body = get(t, h, "/poles?owner=CASCADE+POWER")
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d", rec.Code)
	}
	var byOwner apiserver.PoleIndexJSON
	if err := json.Unmarshal(body, &byOwner); err != nil {
		t.Fatal(err)
	}
	if byOwner.Count != 1 || byOwner.Poles[0].ID != "SE-1129043" {
		t.Errorf("owner = %+v", byOwner)
	}
}

func TestPoleIndexLimit(t *testing.T) {
	t.Parallel()
	h := plantAPI(t)
	rec, body := get(t, h, "/poles?limit=2")
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d", rec.Code)
	}
	var back apiserver.PoleIndexJSON
	if err := json.Unmarshal(body, &back); err != nil {
		t.Fatal(err)
	}
	if back.Count != 3 || len(back.Poles) != 2 {
		t.Errorf("the count is the whole set and the page is what was asked for: %+v", back)
	}
	for _, bad := range []string{"/poles?limit=0", "/poles?limit=-1", "/poles?limit=many"} {
		rec, _ := get(t, h, bad)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s gave code %d", bad, rec.Code)
		}
	}
	rec, body = get(t, h, "/poles?limit=99")
	if rec.Code != http.StatusOK {
		t.Fatal(rec.Code)
	}
	if err := json.Unmarshal(body, &back); err != nil {
		t.Fatal(err)
	}
	if len(back.Poles) != 3 {
		t.Errorf("a limit above the count should not pad, got %d", len(back.Poles))
	}
}

func TestPoleDetail(t *testing.T) {
	t.Parallel()
	rec, body := get(t, plantAPI(t), "/poles/NW-441829")
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", rec.Code, body)
	}
	var back apiserver.PoleDetailJSON
	if err := json.Unmarshal(body, &back); err != nil {
		t.Fatal(err)
	}
	if back.ID != "NW-441829" || back.AboveGrade != "34.0 ft" {
		t.Errorf("pole = %+v", back)
	}
	if back.SafetyZone != "8.5 ft" {
		t.Errorf("safety zone = %q", back.SafetyZone)
	}
	if len(back.Facilities) != 4 || back.Facilities[0].ID != "A1" {
		t.Errorf("facilities = %v", back.Facilities)
	}
	if len(back.Anchors) != 1 || back.Anchors[0].Guys != 1 {
		t.Errorf("anchors = %v", back.Anchors)
	}
	if len(back.Spans) != 1 || back.Spans[0].Other != "NW-441830" {
		t.Errorf("spans = %v", back.Spans)
	}
}

func TestPoleDetailNotFoundAndBadTag(t *testing.T) {
	t.Parallel()
	h := plantAPI(t)
	rec, _ := get(t, h, "/poles/NW-999999")
	if rec.Code != http.StatusNotFound {
		t.Errorf("code = %d", rec.Code)
	}
	for _, path := range []string{"/poles/nope", "/poles/", "/poles/a/b"} {
		rec, _ := get(t, h, path)
		if rec.Code != http.StatusBadRequest && rec.Code != http.StatusNotFound {
			t.Errorf("%s gave code %d", path, rec.Code)
		}
	}
}

func TestSpanDetail(t *testing.T) {
	t.Parallel()
	h := plantAPI(t)
	rec, body := get(t, h, "/spans/SP-1")
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", rec.Code, body)
	}
	var back apiserver.SpanDetailJSON
	if err := json.Unmarshal(body, &back); err != nil {
		t.Fatal(err)
	}
	if back.ID != "SP-1" || back.FromPole != "NW-441829" || back.Length != 200 {
		t.Errorf("span = %+v", back)
	}
	if !back.Level {
		t.Error("a span with no drop is level")
	}
	rec, _ = get(t, h, "/spans/SP-9")
	if rec.Code != http.StatusNotFound {
		t.Errorf("code = %d", rec.Code)
	}
	rec, _ = get(t, h, "/spans/")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("code = %d", rec.Code)
	}
}

func TestAttacherDetail(t *testing.T) {
	t.Parallel()
	h := plantAPI(t)
	// A plus is a space only in a query string, so a name in the path is
	// percent encoded.
	rec, body := get(t, h, "/attachers/RIDGEPORT%20CABLEVISION")
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", rec.Code, body)
	}
	var back apiserver.AttacherJSON
	if err := json.Unmarshal(body, &back); err != nil {
		t.Fatal(err)
	}
	if back.Count != 1 || back.Unauthorized != 0 {
		t.Errorf("attacher = %+v", back)
	}
	if len(back.Poles) != 1 || back.Poles[0] != "NW-441829" {
		t.Errorf("poles = %v", back.Poles)
	}
	rec, _ = get(t, h, "/attachers/NOBODY")
	if rec.Code != http.StatusNotFound {
		t.Errorf("code = %d", rec.Code)
	}
}

func TestPlantRoutesAreOnTheIndex(t *testing.T) {
	t.Parallel()
	rec, body := get(t, plantAPI(t), "/")
	if rec.Code != http.StatusOK {
		t.Fatal(rec.Code)
	}
	var back apiserver.IndexJSON
	if err := json.Unmarshal(body, &back); err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"/poles": false, "/poles/": false, "/spans/": false, "/attachers/": false}
	for _, path := range back.Paths {
		if _, ok := want[path]; ok {
			want[path] = true
		}
	}
	for path, found := range want {
		if !found {
			t.Errorf("%s is not on the index: %v", path, back.Paths)
		}
	}
}
