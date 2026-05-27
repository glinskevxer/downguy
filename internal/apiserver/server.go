// Package apiserver is the read side http service.
//
// It is read only on purpose. Everything that changes plant goes through the
// command line, where it is a file somebody can look at and an exit code a
// pipeline can branch on. What the service is for is the two systems that want
// to ask questions: the mapping front end and the attacher portal.
package apiserver

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/glinskevxer/downguy/internal/config"
	"github.com/glinskevxer/downguy/internal/store"
	"github.com/glinskevxer/downguy/internal/version"
)

// ErrBadRequest is what a handler returns for something the caller sent.
var ErrBadRequest = errors.New("apiserver: bad request")

// Server holds the handlers and what they read from.
type Server struct {
	db      *sql.DB
	cfg     config.Config
	plant   *store.Plant
	routes  map[string]http.HandlerFunc
	now     func() time.Time
	ids     *idCounter
	logger  Logger
	timeout time.Duration
}

// New builds a server over an open database.
func New(db *sql.DB, cfg config.Config) (*Server, error) {
	if db == nil {
		return nil, fmt.Errorf("%w: no database", ErrBadRequest)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	s := &Server{
		db: db, cfg: cfg, plant: store.NewPlant(db),
		routes:  map[string]http.HandlerFunc{},
		now:     time.Now,
		ids:     newIDCounter(time.Now()),
		timeout: DefaultTimeout,
	}
	s.route("/healthz", s.handleHealth)
	s.route("/readyz", s.handleReady)
	s.route("/version", s.handleVersion)
	return s, nil
}

// WithClock replaces the clock, which is how the tests get a fixed uptime out
// of a handler that reports one.
func (s *Server) WithClock(now func() time.Time) *Server {
	if now != nil {
		s.now = now
	}
	return s
}

// route registers a handler on an exact path.
func (s *Server) route(path string, handler http.HandlerFunc) { s.mustRoute(path, handler) }

// mustRoute is the registration guard. Two handlers on one path is a
// programming error that should stop the process starting rather than let one
// of them win silently.
func (s *Server) mustRoute(path string, handler http.HandlerFunc) {
	if _, seen := s.routes[path]; seen {
		panic("apiserver: two handlers on " + path)
	}
	s.routes[path] = handler
}

// Routes returns every registered path, sorted.
func (s *Server) Routes() []string {
	out := make([]string, 0, len(s.routes))
	for path := range s.routes {
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}

// Handler returns the mux with the middleware wrapped round it.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	for path, handler := range s.routes {
		mux.HandleFunc(path, handler)
	}
	mux.HandleFunc("/", s.handleIndex)
	// Outermost first: an id before anything that logs one, recovery before
	// anything that can panic, and the timeout innermost so the handler sees a
	// context that is already bounded.
	return s.withRequestID(s.withLogging(s.withRecovery(s.withReadOnly(s.withTimeout(mux)))))
}

// withReadOnly refuses anything but a read. The service has no write side and
// saying so with a 405 is more useful than a 404 on a path that exists.
func (s *Server) withReadOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead:
			next.ServeHTTP(w, r)
		default:
			w.Header().Set("Allow", "GET, HEAD")
			writeError(w, http.StatusMethodNotAllowed, "this service is read only")
		}
	})
}

// withRecovery turns a panic in a handler into a 500 rather than a dropped
// connection, and keeps the rest of the process up.
func (s *Server) withRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				writeError(w, http.StatusInternalServerError, "internal error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// HealthJSON is what the liveness probe returns.
type HealthJSON struct {
	Status  string `json:"status"`
	Release string `json:"release"`
	Time    string `json:"time"`
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, HealthJSON{
		Status:  "ok",
		Release: version.Current().Release,
		Time:    s.now().UTC().Format(time.RFC3339),
	})
}

// ReadyJSON is what the readiness probe returns.
type ReadyJSON struct {
	Status  string `json:"status"`
	Schema  int    `json:"schema_version"`
	Poles   int    `json:"poles"`
	Message string `json:"message,omitempty"`
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	schema, err := store.CurrentVersion(ctx, s.db)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, ReadyJSON{Status: "unready", Message: err.Error()})
		return
	}
	if schema != store.LatestVersion() {
		writeJSON(w, http.StatusServiceUnavailable, ReadyJSON{
			Status: "unready", Schema: schema,
			Message: fmt.Sprintf("the database is at schema %d and the binary wants %d",
				schema, store.LatestVersion()),
		})
		return
	}
	poles, err := s.plant.Poles().Count(ctx)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, ReadyJSON{Status: "unready", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, ReadyJSON{Status: "ready", Schema: schema, Poles: poles})
}

// VersionJSON is what the version endpoint returns.
type VersionJSON struct {
	Name     string `json:"name"`
	Release  string `json:"release"`
	Commit   string `json:"commit,omitempty"`
	Go       string `json:"go"`
	Platform string `json:"platform"`
}

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	info := version.Current()
	writeJSON(w, http.StatusOK, VersionJSON{
		Name: info.Name, Release: info.Release, Commit: info.ShortCommit(),
		Go: info.GoVersion, Platform: info.Platform,
	})
}

// IndexJSON lists what the service answers.
type IndexJSON struct {
	Service string   `json:"service"`
	Utility string   `json:"utility"`
	Paths   []string `json:"paths"`
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		writeError(w, http.StatusNotFound, "no such path")
		return
	}
	writeJSON(w, http.StatusOK, IndexJSON{
		Service: version.Name, Utility: s.cfg.Utility, Paths: s.Routes(),
	})
}

// ErrorJSON is the shape every failure comes back as, so a caller never has to
// guess whether a body is a result or a message.
type ErrorJSON struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, ErrorJSON{Error: message})
}

// Serve runs the service until the context is cancelled, then shuts it down
// with a grace period so an in flight request is not cut off mid answer.
func (s *Server) Serve(ctx context.Context, listen func(*http.Server) error) error {
	server := &http.Server{
		Addr:         s.cfg.HTTP.Address,
		Handler:      s.Handler(),
		ReadTimeout:  time.Duration(s.cfg.HTTP.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(s.cfg.HTTP.WriteTimeout) * time.Second,
	}
	errs := make(chan error, 1)
	go func() { errs <- listen(server) }()

	select {
	case err := <-errs:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdown)
	}
}

// PathParam pulls the trailing segment off a path, which is how every record
// endpoint here names what it is being asked about.
func PathParam(path, prefix string) (string, error) {
	if !strings.HasPrefix(path, prefix) {
		return "", fmt.Errorf("%w: %q is not under %q", ErrBadRequest, path, prefix)
	}
	rest := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	if rest == "" {
		return "", fmt.Errorf("%w: %q names nothing", ErrBadRequest, path)
	}
	if strings.Contains(rest, "/") {
		return "", fmt.Errorf("%w: %q has more path than this endpoint takes", ErrBadRequest, path)
	}
	return rest, nil
}
