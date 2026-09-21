package httpserver

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/errs"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("GET /ping", NameSpanByRoute(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"pong":true}`))
	})))
	mux.HandleFunc("GET /boom", func(http.ResponseWriter, *http.Request) { panic("boom") })
	return New(":0", slog.New(slog.NewTextHandler(io.Discard, nil)), mux)
}

func TestRoutedRequestCarriesRequestID(t *testing.T) {
	srv := newTestServer(t)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ping", nil))
	if rec.Code != http.StatusOK || rec.Header().Get("X-Request-ID") == "" {
		t.Fatalf("status = %d, request id = %q", rec.Code, rec.Header().Get("X-Request-ID"))
	}
	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("X-Request-ID", "abc-123")
	srv.Handler().ServeHTTP(rec, req)
	if got := rec.Header().Get("X-Request-ID"); got != "abc-123" {
		t.Errorf("X-Request-ID = %q, want abc-123", got)
	}
}

func TestWrongMethodIs405(t *testing.T) {
	srv := newTestServer(t)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/ping", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

func TestPanicBecomes500(t *testing.T) {
	srv := newTestServer(t)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/boom", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func TestChainOrder(t *testing.T) {
	var order []string
	mw := func(name string) Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, name)
				next.ServeHTTP(w, r)
			})
		}
	}
	h := Chain(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { order = append(order, "handler") }), mw("outer"), mw("inner"))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	if got := strings.Join(order, ","); got != "outer,inner,handler" {
		t.Errorf("order = %s", got)
	}
}

func TestSpanIsNamedByRoute(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev) })

	srv := newTestServer(t)
	srv.Handler().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/ping", nil))
	spans := exporter.GetSpans()
	if len(spans) != 1 || spans[0].Name != "GET /ping" {
		t.Fatalf("spans = %+v", spans)
	}
}

func TestProblemFromClassifiesAndHidesInternals(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	ctx := req.Context()

	p := ProblemFrom(ctx, "/x", errs.Wrap(errs.New(errs.Conflict, "concurrent-update", "reload and retry"), "db says %s", "23505"))
	if p.Status != 409 || p.Code != "concurrent-update" || !p.Retryable || p.Detail != "reload and retry" {
		t.Errorf("classified problem = %+v", p)
	}
	if p.Type != "urn:dispute-engine:error:concurrent-update" {
		t.Errorf("type = %s", p.Type)
	}

	p = ProblemFrom(ctx, "/x", errors.New("pq: password authentication failed for user dispute"))
	if p.Status != 500 || p.Code != "internal" || p.Retryable || strings.Contains(p.Detail, "password") {
		t.Errorf("internal problem leaked or misclassified: %+v", p)
	}

	p = ProblemFrom(ctx, "/x", errs.New(errs.Unavailable, "unavailable", ""))
	if p.Status != 503 || p.RetryAfterSeconds == nil || !p.Retryable {
		t.Errorf("unavailable problem = %+v", p)
	}
}

// Inner middleware clones the request; the log line must still carry the matched route and any annotations.
func TestRequestLogCarriesRouteAndAnnotationsThroughClonedRequests(t *testing.T) {
	var buf strings.Builder
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	mux := http.NewServeMux()
	mux.Handle("GET /ping", NameSpanByRoute(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Annotate(r.Context(), "tenant", "t-1")
		w.WriteHeader(http.StatusNoContent)
	})))
	clone := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { next.ServeHTTP(w, r.WithContext(r.Context())) })
	}
	srv := New(":0", logger, mux, clone)
	srv.Handler().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/ping", nil))
	line := buf.String()
	for _, want := range []string{`"route":"GET /ping"`, `"tenant":"t-1"`, `"status":204`} {
		if !strings.Contains(line, want) {
			t.Errorf("log line missing %s: %s", want, line)
		}
	}
}
