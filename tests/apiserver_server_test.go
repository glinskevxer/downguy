package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/glinskevxer/downguy/internal/apiserver"
	"github.com/glinskevxer/downguy/internal/config"
	"github.com/glinskevxer/downguy/internal/store"
)

func apiServer(t *testing.T) http.Handler {
	t.Helper()
	db := openStore(t)
	cfg := config.Default()
	s, err := apiserver.New(db, cfg)
	if err != nil {
		t.Fatal(err)
	}
	fixed := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	return s.WithClock(func() time.Time { return fixed }).Handler()
}

func get(t *testing.T, h http.Handler, path string) (*httptest.ResponseRecorder, []byte) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec, rec.Body.Bytes()
}

func TestNewRejectsBadInput(t *testing.T) {
	t.Parallel()
	if _, err := apiserver.New(nil, config.Default()); err == nil {
		t.Error("no database should be rejected")
	}
	bad := config.Default()
	bad.District = "tropical"
	if _, err := apiserver.New(openStore(t), bad); err == nil {
		t.Error("an invalid configuration should be rejected")
	}
}

func TestHealth(t *testing.T) {
	t.Parallel()
	rec, body := get(t, apiServer(t), "/healthz")
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Errorf("content type = %q", got)
	}
	var back apiserver.HealthJSON
	if err := json.Unmarshal(body, &back); err != nil {
		t.Fatal(err)
	}
	if back.Status != "ok" || back.Release == "" {
		t.Errorf("health = %+v", back)
	}
	if back.Time != "2026-02-01T12:00:00Z" {
		t.Errorf("the handler should use the clock it was given, got %q", back.Time)
	}
}

func TestReady(t *testing.T) {
	t.Parallel()
	rec, body := get(t, apiServer(t), "/readyz")
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", rec.Code, body)
	}
	var back apiserver.ReadyJSON
	if err := json.Unmarshal(body, &back); err != nil {
		t.Fatal(err)
	}
	if back.Status != "ready" || back.Schema != store.LatestVersion() {
		t.Errorf("ready = %+v", back)
	}
	if back.Poles != 0 {
		t.Errorf("an empty database has no poles, got %d", back.Poles)
	}
}

func TestReadyOnAnUnmigratedDatabase(t *testing.T) {
	t.Parallel()
	db, err := store.OpenRaw(store.Memory)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	s, err := apiserver.New(db, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	rec, body := get(t, s.Handler(), "/readyz")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("code = %d, body = %s", rec.Code, body)
	}
	var back apiserver.ReadyJSON
	if err := json.Unmarshal(body, &back); err != nil {
		t.Fatal(err)
	}
	if back.Status != "unready" || back.Message == "" {
		t.Errorf("ready = %+v", back)
	}
}

func TestVersionEndpoint(t *testing.T) {
	t.Parallel()
	rec, body := get(t, apiServer(t), "/version")
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d", rec.Code)
	}
	var back apiserver.VersionJSON
	if err := json.Unmarshal(body, &back); err != nil {
		t.Fatal(err)
	}
	if back.Name != "downguy" || back.Go == "" || back.Platform == "" {
		t.Errorf("version = %+v", back)
	}
}

func TestIndexListsTheRoutes(t *testing.T) {
	t.Parallel()
	rec, body := get(t, apiServer(t), "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d", rec.Code)
	}
	var back apiserver.IndexJSON
	if err := json.Unmarshal(body, &back); err != nil {
		t.Fatal(err)
	}
	if back.Service != "downguy" || back.Utility != config.Default().Utility {
		t.Errorf("index = %+v", back)
	}
	if len(back.Paths) < 3 {
		t.Errorf("paths = %v", back.Paths)
	}
	for i := 1; i < len(back.Paths); i++ {
		if back.Paths[i] < back.Paths[i-1] {
			t.Fatalf("paths are not sorted: %v", back.Paths)
		}
	}
}

func TestAnUnknownPath(t *testing.T) {
	t.Parallel()
	rec, body := get(t, apiServer(t), "/nothing-here")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("code = %d", rec.Code)
	}
	var back apiserver.ErrorJSON
	if err := json.Unmarshal(body, &back); err != nil {
		t.Fatal(err)
	}
	if back.Error == "" {
		t.Error("a failure should carry a message")
	}
}

func TestTheServiceIsReadOnly(t *testing.T) {
	t.Parallel()
	h := apiServer(t)
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		req := httptest.NewRequest(method, "/healthz", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s gave code %d", method, rec.Code)
		}
		if got := rec.Header().Get("Allow"); got != "GET, HEAD" {
			t.Errorf("%s Allow header = %q", method, got)
		}
	}
	req := httptest.NewRequest(http.MethodHead, "/healthz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("HEAD gave code %d", rec.Code)
	}
}

func TestPathParam(t *testing.T) {
	t.Parallel()
	got, err := apiserver.PathParam("/poles/NW-441829", "/poles/")
	if err != nil || got != "NW-441829" {
		t.Errorf("PathParam = %q %v", got, err)
	}
	got, err = apiserver.PathParam("/poles/NW-441829/", "/poles/")
	if err != nil || got != "NW-441829" {
		t.Errorf("a trailing slash should not matter, got %q %v", got, err)
	}
	for _, path := range []string{"/spans/SP-1", "/poles/", "/poles", "/poles/a/b"} {
		if _, err := apiserver.PathParam(path, "/poles/"); err == nil {
			t.Errorf("PathParam(%q) should have failed", path)
		}
	}
}

func TestServeStopsWithTheContext(t *testing.T) {
	t.Parallel()
	db := openStore(t)
	s, err := apiserver.New(db, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	started := make(chan struct{})
	go func() {
		done <- s.Serve(ctx, func(server *http.Server) error {
			close(started)
			<-ctx.Done()
			return http.ErrServerClosed
		})
	}()
	<-started
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Serve = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not stop")
	}
}

func TestServeReturnsAListenFailure(t *testing.T) {
	t.Parallel()
	db := openStore(t)
	s, err := apiserver.New(db, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	sentinel := context.DeadlineExceeded
	err = s.Serve(context.Background(), func(server *http.Server) error { return sentinel })
	if err != sentinel {
		t.Errorf("Serve = %v, want the listen failure", err)
	}
	closed := s.Serve(context.Background(), func(server *http.Server) error { return http.ErrServerClosed })
	if closed != nil {
		t.Errorf("a clean close is not a failure, got %v", closed)
	}
}
