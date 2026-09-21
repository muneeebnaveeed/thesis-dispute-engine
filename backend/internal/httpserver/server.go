package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// Server owns the mux and the http.Server lifecycle.
type Server struct {
	http   *http.Server
	logger *slog.Logger
}

// New builds the routed, middleware-wrapped server. Routes are registered in
// routes.go so this constructor stays a pure wiring step.
func New(addr string, logger *slog.Logger) *Server {
	mux := http.NewServeMux()
	registerRoutes(mux)

	// Outermost: the OpenTelemetry server span, so every middleware below runs
	// inside it and the request log can carry its IDs. Matched routes rename
	// the span to their pattern (see route in routes.go); anything that falls
	// through keeps this method-only name so unmatched URLs cannot explode
	// span cardinality.
	handler := otelhttp.NewHandler(
		Chain(mux,
			Recover(logger),
			RequestID,
			Log(logger),
		),
		"http.server",
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			return r.Method + " (unmatched)"
		}),
	)

	return &Server{
		logger: logger,
		http: &http.Server{
			Addr:              addr,
			Handler:           handler,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       15 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       60 * time.Second,
		},
	}
}

// Handler exposes the composed handler for in-process tests.
func (s *Server) Handler() http.Handler { return s.http.Handler }

// ListenAndServe blocks until the server stops. A closed server is not an error.
func (s *Server) ListenAndServe() error {
	s.logger.Info("listening", "addr", s.http.Addr)
	if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Shutdown drains in-flight requests within ctx's deadline.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}

// writeJSON is the single place responses are encoded, so every handler agrees
// on content type and encoding errors are not silently dropped.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// Headers are already out; nothing better than noting it can be done.
		slog.Default().Error("encode response", "err", err)
	}
}
