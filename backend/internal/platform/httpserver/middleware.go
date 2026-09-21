package httpserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/telemetry"
)

// Middleware wraps a handler with cross-cutting behaviour.
type Middleware func(http.Handler) http.Handler

// Chain applies middlewares so the first listed is the outermost.
func Chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

type ctxKey int

const requestKey ctxKey = iota

// request is the mutable per-request record inner middleware writes to and Log reads; inner layers clone the
// *http.Request, so anything they learn (the matched route, the tenant) has to travel back through a pointer.
type request struct {
	id    string
	mu    sync.Mutex
	route string
	attrs []slog.Attr
}

func requestFrom(ctx context.Context) *request {
	r, _ := ctx.Value(requestKey).(*request)
	return r
}

// RequestIDFrom returns the request ID set by RequestID, or "".
func RequestIDFrom(ctx context.Context) string {
	if r := requestFrom(ctx); r != nil {
		return r.id
	}
	return ""
}

// Annotate adds a field to the request log line; a no-op outside a request.
func Annotate(ctx context.Context, key string, value any) {
	if r := requestFrom(ctx); r != nil {
		r.mu.Lock()
		r.attrs = append(r.attrs, slog.Any(key, value))
		r.mu.Unlock()
	}
}

func setRoute(ctx context.Context, route string) {
	if r := requestFrom(ctx); r != nil {
		r.mu.Lock()
		r.route = route
		r.mu.Unlock()
	}
}

// RequestID honours an inbound X-Request-ID or mints one, and echoes it back.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = newRequestID()
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestKey, &request{id: id})))
	})
}

func newRequestID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Not recoverable; a timestamp is unique enough for log correlation.
		return time.Now().UTC().Format("20060102T150405.000000000")
	}
	return hex.EncodeToString(b[:])
}

// Recover converts a handler panic into a logged 500 instead of a dropped connection.
func Recover(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if p := recover(); p != nil {
					logger.ErrorContext(r.Context(), "handler panicked",
						"panic", p,
						"request_id", RequestIDFrom(r.Context()),
						"method", r.Method,
						"path", r.URL.Path,
					)
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	return s.ResponseWriter.Write(b)
}

// Log writes one structured line per request, correlated with the trace.
func Log(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w}
			next.ServeHTTP(rec, r)
			if rec.status == 0 {
				rec.status = http.StatusOK
			}
			traceID, spanID := telemetry.SpanIDs(r.Context())
			route, extra := r.Pattern, []slog.Attr(nil)
			if info := requestFrom(r.Context()); info != nil {
				info.mu.Lock()
				if info.route != "" {
					route = info.route
				}
				extra = append(extra, info.attrs...)
				info.mu.Unlock()
			}
			logger.LogAttrs(r.Context(), slog.LevelInfo, "request", append([]slog.Attr{
				slog.String("request_id", RequestIDFrom(r.Context())),
				slog.String("trace_id", traceID),
				slog.String("span_id", spanID),
				slog.String("method", r.Method),
				slog.String("route", route),
				slog.String("path", r.URL.Path),
				slog.Int("status", rec.status),
				slog.Int64("duration_ms", time.Since(start).Milliseconds()),
			}, extra...)...)
		})
	}
}
