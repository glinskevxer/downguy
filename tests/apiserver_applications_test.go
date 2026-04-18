package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/glinskevxer/downguy/internal/apiserver"
	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/config"
	"github.com/glinskevxer/downguy/internal/permit"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/store"
)

func applicationAPI(t *testing.T) http.Handler {
	t.Helper()
	db := openStore(t)
	ctx := context.Background()
	repo := store.NewApplications(db)

	first, err := permit.New("APP-1042", "MERIDIAN FIBER", "SUMMIT VALLEY ELECTRIC",
		[]plant.PoleID{"NW-441829", "NW-441830"}, calendar.MustNew(2025, 3, 3), "j.aldana")
	if err != nil {
		t.Fatal(err)
	}
	if err := first.MoveTo(permit.StateSubmitted, calendar.MustNew(2025, 3, 5), "j.aldana", "filed"); err != nil {
		t.Fatal(err)
	}
	second, err := permit.New("APP-1043", "CASCADE BROADBAND", "SUMMIT VALLEY ELECTRIC",
		[]plant.PoleID{"SE-1129043"}, calendar.MustNew(2025, 4, 1), "r.beltre")
	if err != nil {
		t.Fatal(err)
	}
	if err := second.MoveTo(permit.StateWithdrawn, calendar.MustNew(2025, 4, 20), "r.beltre", ""); err != nil {
		t.Fatal(err)
	}
	for _, app := range []*permit.Application{first, second} {
		if err := repo.Put(ctx, app); err != nil {
			t.Fatal(err)
		}
	}
	s, err := apiserver.New(db, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	return s.RegisterApplications().Handler()
}

func TestApplicationIndexListsTheLiveOnes(t *testing.T) {
	t.Parallel()
	rec, body := get(t, applicationAPI(t), "/applications")
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", rec.Code, body)
	}
	var back apiserver.ApplicationIndexJSON
	if err := json.Unmarshal(body, &back); err != nil {
		t.Fatal(err)
	}
	if back.Count != 1 || back.Applications[0].ID != "APP-1042" {
		t.Fatalf("index = %+v", back)
	}
	row := back.Applications[0]
	if row.State != "submitted" || row.HeldBy != "utility" || !row.Live {
		t.Errorf("row = %+v", row)
	}
	if row.Poles != 2 || row.Opened != "2025-03-03" {
		t.Errorf("row = %+v", row)
	}
}

func TestApplicationIndexFilters(t *testing.T) {
	t.Parallel()
	h := applicationAPI(t)
	for _, c := range []struct {
		path string
		id   string
	}{
		{"/applications?state=withdrawn", "APP-1043"},
		{"/applications?applicant=CASCADE%20BROADBAND", "APP-1043"},
		{"/applications?pole=NW-441829", "APP-1042"},
	} {
		rec, body := get(t, h, c.path)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s gave code %d", c.path, rec.Code)
		}
		var back apiserver.ApplicationIndexJSON
		if err := json.Unmarshal(body, &back); err != nil {
			t.Fatal(err)
		}
		if back.Count != 1 || back.Applications[0].ID != c.id {
			t.Errorf("%s = %+v", c.path, back)
		}
	}
	for _, bad := range []string{"/applications?state=pending", "/applications?pole=nope"} {
		rec, _ := get(t, h, bad)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s gave code %d", bad, rec.Code)
		}
	}
	rec, body := get(t, h, "/applications?applicant=NOBODY")
	if rec.Code != http.StatusOK {
		t.Fatal(rec.Code)
	}
	var empty apiserver.ApplicationIndexJSON
	if err := json.Unmarshal(body, &empty); err != nil {
		t.Fatal(err)
	}
	if empty.Count != 0 || empty.Applications == nil {
		t.Errorf("an empty listing should still carry a list, got %+v", empty)
	}
}

func TestApplicationDetail(t *testing.T) {
	t.Parallel()
	h := applicationAPI(t)
	rec, body := get(t, h, "/applications/APP-1042")
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", rec.Code, body)
	}
	var back apiserver.ApplicationDetailJSON
	if err := json.Unmarshal(body, &back); err != nil {
		t.Fatal(err)
	}
	if back.ID != "APP-1042" || back.Applicant != "MERIDIAN FIBER" {
		t.Errorf("application = %+v", back)
	}
	if len(back.PoleTags) != 2 || back.PoleTags[0] != "NW-441829" {
		t.Errorf("poles = %v", back.PoleTags)
	}
	if len(back.History) != 2 {
		t.Fatalf("history = %v", back.History)
	}
	if back.History[1].To != "submitted" || back.History[1].Note != "filed" {
		t.Errorf("event = %+v", back.History[1])
	}

	rec, _ = get(t, h, "/applications/APP-9999")
	if rec.Code != http.StatusNotFound {
		t.Errorf("code = %d", rec.Code)
	}
	rec, _ = get(t, h, "/applications/")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("code = %d", rec.Code)
	}
}

func TestAllRoutesRegistersEveryGroup(t *testing.T) {
	t.Parallel()
	s, err := apiserver.New(openStore(t), config.Default())
	if err != nil {
		t.Fatal(err)
	}
	s.AllRoutes()
	for _, path := range []string{
		"/healthz", "/readyz", "/version",
		"/poles", "/poles/", "/spans/", "/attachers/",
		"/rates/", "/rules", "/applications", "/applications/",
	} {
		if !s.HasRoute(path) {
			t.Errorf("%s is not registered, got %v", path, s.Routes())
		}
	}
	if s.HasRoute("/nothing") {
		t.Error("HasRoute should miss")
	}
}
