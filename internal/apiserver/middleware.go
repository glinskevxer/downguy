package apiserver

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// A read side service in front of a sqlite file has two ways to hurt: a query
// somebody left running, and no way to tell afterwards which request it was.
// The middleware here answers both, and it is deliberately small: this is not
// a framework, it is four wrappers with one job each.

// requestIDKey is the context key the request id is carried under. It is an
// unexported type so nothing outside this package can collide with it.
type requestIDKey struct{}

// RequestID pulls the id out of a request context, or the empty string when
// the request did not come through the middleware.
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

// idCounter is the source of request ids. They are a counter and the process
// start time rather than a random value, because the only thing they have to
// do is match a line in a log to a line in a report.
type idCounter struct {
	prefix string
	next   atomic.Uint64
}

func newIDCounter(started time.Time) *idCounter {
	return &idCounter{prefix: strconv.FormatInt(started.Unix(), 36)}
}

func (c *idCounter) take() string {
	return fmt.Sprintf("%s-%d", c.prefix, c.next.Add(1))
}

// Access is one finished request, as the log records it.
type Access struct {
	ID       string
	Method   string
	Path     string
	Status   int
	Bytes    int
	Duration time.Duration
}

// String prints the log line.
func (a Access) String() string {
	return fmt.Sprintf("%s %s %s %d %dB %s",
		a.ID, a.Method, a.Path, a.Status, a.Bytes, a.Duration.Round(time.Millisecond))
}

// Logger is where finished requests go. It is an interface of one method so a
// caller can send them wherever it likes without this package caring.
type Logger interface {
	Log(Access)
}

// WriterLogger writes access lines to a stream.
type WriterLogger struct{ Out io.Writer }

// Log writes one line.
func (w WriterLogger) Log(a Access) {
	if w.Out == nil {
		return
	}
	fmt.Fprintln(w.Out, a)
}

// CollectingLogger keeps what it was given, which is what the tests read.
type CollectingLogger struct{ Lines []Access }

// Log records one request.
func (c *CollectingLogger) Log(a Access) { c.Lines = append(c.Lines, a) }

// WithLogger sends finished requests somewhere.
func (s *Server) WithLogger(l Logger) *Server {
	s.logger = l
	return s
}

// WithTimeout sets how long a request may run before it is cut off.
func (s *Server) WithTimeout(d time.Duration) *Server {
	if d > 0 {
		s.timeout = d
	}
	return s
}

// recorder wraps a response writer so the middleware can see what went out.
type recorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *recorder) WriteHeader(status int) {
	if r.status == 0 {
		r.status = status
	}
	r.ResponseWriter.WriteHeader(status)
}

func (r *recorder) Write(p []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(p)
	r.bytes += n
	return n, err
}

// withRequestID stamps every request with an id and puts it on the response,
// so a caller reporting a slow request can say which one it was.
func (s *Server) withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := s.ids.take()
		w.Header().Set("X-Request-Id", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey{}, id)))
	})
}

// withTimeout cuts a request off rather than letting it hold a connection and
// a database handle indefinitely.
func (s *Server) withTimeout(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// withLogging records what each request did and how long it took.
func (s *Server) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.logger == nil {
			next.ServeHTTP(w, r)
			return
		}
		started := s.now()
		rec := &recorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)
		if rec.status == 0 {
			rec.status = http.StatusOK
		}
		s.logger.Log(Access{
			ID: RequestID(r.Context()), Method: r.Method,
			Path: cleanPath(r.URL.Path), Status: rec.status,
			Bytes: rec.bytes, Duration: s.now().Sub(started),
		})
	})
}

// cleanPath trims a path for the log. A query string can carry an attacher
// name and a log is a thing people paste into tickets, so it does not go in.
func cleanPath(path string) string {
	if path == "" {
		return "/"
	}
	if len(path) > 200 {
		return path[:200] + "..."
	}
	return strings.TrimRight(path, " ")
}

// DefaultTimeout is how long a request may run when nothing else says.
const DefaultTimeout = 20 * time.Second
