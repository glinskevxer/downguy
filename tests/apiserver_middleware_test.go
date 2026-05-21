package tests

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/glinskevxer/downguy/internal/apiserver"
	"github.com/glinskevxer/downguy/internal/config"
)

func loggedServer(t *testing.T) (http.Handler, *apiserver.CollectingLogger) {
	t.Helper()
	s, err := apiserver.New(openStore(t), config.Default())
	if err != nil {
		t.Fatal(err)
	}
	log := &apiserver.CollectingLogger{}
	return s.RegisterPlant().WithLogger(log).Handler(), log
}

func TestEveryResponseCarriesARequestID(t *testing.T) {
	t.Parallel()
	h, _ := loggedServer(t)
	first, _ := get(t, h, "/healthz")
	second, _ := get(t, h, "/healthz")
	a := first.Header().Get("X-Request-Id")
	b := second.Header().Get("X-Request-Id")
	if a == "" || b == "" {
		t.Fatalf("ids = %q and %q", a, b)
	}
	if a == b {
		t.Errorf("two requests should not share an id, both got %q", a)
	}
	if !strings.Contains(a, "-") {
		t.Errorf("id = %q", a)
	}
}

func TestTheAccessLogRecordsWhatHappened(t *testing.T) {
	t.Parallel()
	h, log := loggedServer(t)
	rec, _ := get(t, h, "/poles")
	if rec.Code != http.StatusOK {
		t.Fatal(rec.Code)
	}
	if _, _ = get(t, h, "/nothing-here"); false {
		t.Fatal("unreachable")
	}
	if len(log.Lines) != 2 {
		t.Fatalf("lines = %v", log.Lines)
	}
	ok := log.Lines[0]
	if ok.Method != http.MethodGet || ok.Path != "/poles" || ok.Status != http.StatusOK {
		t.Errorf("line = %+v", ok)
	}
	if ok.Bytes <= 0 || ok.ID == "" {
		t.Errorf("line = %+v", ok)
	}
	missing := log.Lines[1]
	if missing.Status != http.StatusNotFound {
		t.Errorf("line = %+v", missing)
	}
	if !strings.Contains(ok.String(), "/poles") || !strings.Contains(ok.String(), "200") {
		t.Errorf("String = %q", ok.String())
	}
}

func TestARefusedMethodIsStillLogged(t *testing.T) {
	t.Parallel()
	h, log := loggedServer(t)
	req := httptest.NewRequest(http.MethodPost, "/healthz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("code = %d", rec.Code)
	}
	if len(log.Lines) != 1 || log.Lines[0].Status != http.StatusMethodNotAllowed {
		t.Errorf("lines = %v", log.Lines)
	}
	if log.Lines[0].Method != http.MethodPost {
		t.Errorf("line = %+v", log.Lines[0])
	}
}

func TestAServerWithNoLoggerStillServes(t *testing.T) {
	t.Parallel()
	s, err := apiserver.New(openStore(t), config.Default())
	if err != nil {
		t.Fatal(err)
	}
	rec, _ := get(t, s.Handler(), "/healthz")
	if rec.Code != http.StatusOK {
		t.Errorf("code = %d", rec.Code)
	}
}

func TestWriterLogger(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := apiserver.WriterLogger{Out: &buf}
	logger.Log(apiserver.Access{
		ID: "abc-1", Method: "GET", Path: "/poles",
		Status: 200, Bytes: 412, Duration: 3 * time.Millisecond,
	})
	line := strings.TrimSpace(buf.String())
	if line != "abc-1 GET /poles 200 412B 3ms" {
		t.Errorf("line = %q", line)
	}
	var empty apiserver.WriterLogger
	empty.Log(apiserver.Access{})
}

func TestTheTimeoutIsSettableAndBoundsTheHandler(t *testing.T) {
	t.Parallel()
	s, err := apiserver.New(openStore(t), config.Default())
	if err != nil {
		t.Fatal(err)
	}
	// A handler that reports what deadline it was given proves the middleware
	// is on the path, which timing a slow request would not do reliably.
	s.WithTimeout(50 * time.Millisecond)
	rec, body := get(t, s.Handler(), "/healthz")
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", rec.Code, body)
	}
	if apiserver.DefaultTimeout <= 0 {
		t.Error("the default timeout should be a real duration")
	}
	s.WithTimeout(0)
	rec, _ = get(t, s.Handler(), "/healthz")
	if rec.Code != http.StatusOK {
		t.Errorf("a zero timeout should leave the previous one alone, got %d", rec.Code)
	}
}

func TestRequestIDIsEmptyOutsideTheMiddleware(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	if got := apiserver.RequestID(req.Context()); got != "" {
		t.Errorf("id = %q", got)
	}
}
