package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/glinskevxer/downguy/internal/apiserver"
	"github.com/glinskevxer/downguy/internal/config"
	"github.com/glinskevxer/downguy/internal/store"
	"github.com/glinskevxer/downguy/internal/survey"
	"github.com/glinskevxer/downguy/internal/unit"
)

func findingsAPI(t *testing.T, lift bool) http.Handler {
	t.Helper()
	db := openStore(t)
	ctx := context.Background()
	loader := store.NewPlant(db)
	if err := loader.SaveAssembly(ctx, sampleAssembly()); err != nil {
		t.Fatal(err)
	}
	second := samplePole()
	second.ID = "NW-441830"
	if err := loader.Poles().Put(ctx, second); err != nil {
		t.Fatal(err)
	}
	sheet := sampleSheet()
	if lift {
		// Bring the cable up to two feet under the neutral so the safety zone
		// rule has something to say.
		for i := range sheet.Measurements {
			if sheet.Measurements[i].Subject == "A3" {
				sheet.Measurements[i].Height = unit.Tenths(265)
			}
		}
	}
	if err := store.NewSurveys(db).Put(ctx, sheet); err != nil {
		t.Fatal(err)
	}
	s, err := apiserver.New(db, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	return s.RegisterFindings().Handler()
}

func TestFindingsEndpointOnACleanPole(t *testing.T) {
	t.Parallel()
	rec, body := get(t, findingsAPI(t, false), "/findings/NW-441829")
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", rec.Code, body)
	}
	var back apiserver.FindingsJSON
	if err := json.Unmarshal(body, &back); err != nil {
		t.Fatal(err)
	}
	if back.Pole != "NW-441829" || back.Surveyed != "2025-05-06" {
		t.Errorf("result = %+v", back)
	}
	if back.District != "heavy" || back.Grade != "C" {
		t.Errorf("result = %+v", back)
	}
	if back.Blocking || len(back.Findings) != 0 {
		t.Errorf("result = %+v", back)
	}
	if back.Findings == nil {
		t.Error("an empty result should still carry a list")
	}
}

func TestFindingsEndpointReportsAViolation(t *testing.T) {
	t.Parallel()
	rec, body := get(t, findingsAPI(t, true), "/findings/NW-441829")
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", rec.Code, body)
	}
	var back apiserver.FindingsJSON
	if err := json.Unmarshal(body, &back); err != nil {
		t.Fatal(err)
	}
	if len(back.Findings) == 0 {
		t.Fatalf("no findings: %s", body)
	}
	f := back.Findings[0]
	if f.Rule != "NESC-235C1" || f.Severity != "stop" {
		t.Errorf("finding = %+v", f)
	}
	if !back.Blocking {
		t.Error("a stop finding blocks")
	}
	if f.Measured == "" || f.Required == "" || f.Deficit == "" {
		t.Errorf("finding = %+v", f)
	}
	if f.Owner != "RIDGEPORT CABLEVISION" {
		t.Errorf("owner = %q", f.Owner)
	}
}

func TestFindingsEndpointTakesTheOverrides(t *testing.T) {
	t.Parallel()
	h := findingsAPI(t, false)
	rec, body := get(t, h, "/findings/NW-441829?district=light&grade=B")
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", rec.Code, body)
	}
	var back apiserver.FindingsJSON
	if err := json.Unmarshal(body, &back); err != nil {
		t.Fatal(err)
	}
	if back.District != "light" || back.Grade != "B" {
		t.Errorf("result = %+v", back)
	}
	for _, bad := range []string{"?district=tropical", "?grade=A", "?as_of=nope"} {
		rec, _ := get(t, h, "/findings/NW-441829"+bad)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s gave code %d", bad, rec.Code)
		}
	}
}

func TestFindingsEndpointReportsStaleness(t *testing.T) {
	t.Parallel()
	h := findingsAPI(t, false)
	rec, body := get(t, h, "/findings/NW-441829?as_of=2025-06-01")
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d", rec.Code)
	}
	var fresh apiserver.FindingsJSON
	if err := json.Unmarshal(body, &fresh); err != nil {
		t.Fatal(err)
	}
	if fresh.Stale {
		t.Error("a month old survey is not stale")
	}
	_, body = get(t, h, "/findings/NW-441829?as_of=2026-06-01")
	var old apiserver.FindingsJSON
	if err := json.Unmarshal(body, &old); err != nil {
		t.Fatal(err)
	}
	if !old.Stale {
		t.Errorf("a year old survey is past the %d day shelf life", survey.StaleAfterDays)
	}
}

func TestFindingsEndpointSaysWhenNobodyHasBeenOut(t *testing.T) {
	t.Parallel()
	h := findingsAPI(t, false)
	// The second pole is on file and has never been surveyed. That is a 404
	// rather than an empty finding list on purpose.
	rec, body := get(t, h, "/findings/NW-441830")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("code = %d, body = %s", rec.Code, body)
	}
	var back apiserver.ErrorJSON
	if err := json.Unmarshal(body, &back); err != nil {
		t.Fatal(err)
	}
	if back.Error == "" {
		t.Error("the refusal should say why")
	}

	rec, _ = get(t, h, "/findings/NW-999999")
	if rec.Code != http.StatusNotFound {
		t.Errorf("a pole that is not on file gave code %d", rec.Code)
	}
	for _, path := range []string{"/findings/nope", "/findings/", "/findings/a/b"} {
		rec, _ := get(t, h, path)
		if rec.Code != http.StatusBadRequest && rec.Code != http.StatusNotFound {
			t.Errorf("%s gave code %d", path, rec.Code)
		}
	}
}

func TestAllRoutesCarriesTheFindingsEndpoint(t *testing.T) {
	t.Parallel()
	s, err := apiserver.New(openStore(t), config.Default())
	if err != nil {
		t.Fatal(err)
	}
	if !s.AllRoutes().HasRoute("/findings/") {
		t.Errorf("the findings endpoint is not registered, got %v", s.Routes())
	}
}
