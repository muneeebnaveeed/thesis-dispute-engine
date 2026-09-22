// Package httpserver is the HTTP layer: stdlib ServeMux routing and middleware (see docs/adr/0001).
package httpserver

import (
	"context"
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

// New builds the middleware-wrapped server around a mux the caller has populated; extra middleware runs innermost.
func New(addr string, logger *slog.Logger, mux http.Handler, extra ...Middleware) *Server {
	// Unmatched requests keep a method-only span name so arbitrary URLs cannot explode cardinality.
	handler := otelhttp.NewHandler(
		Chain(mux,
			append([]Middleware{Recover(logger), RequestID, Log(logger)}, extra...)...,
		),
		"http.server",
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			return r.Method + " (unmatched)"
		}),
		// a health check is a heartbeat, not a request anyone will look for in a trace
		otelhttp.WithFilter(func(r *http.Request) bool { return r.URL.Path != "/healthz" }),
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

// ListenAndServe blocks until the server stops; a graceful close is not an error.
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
