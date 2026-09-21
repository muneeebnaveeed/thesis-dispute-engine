package httpserver

import (
	"net/http"

	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
)

func registerRoutes(mux *http.ServeMux) {
	route(mux, "GET /healthz", handleHealthz)
}

// route names the span after the pattern; ServeMux only sets r.Pattern after matching, so this cannot be outer middleware.
func route(mux *http.ServeMux, pattern string, h http.HandlerFunc) {
	mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		span := trace.SpanFromContext(r.Context())
		span.SetName(pattern)
		span.SetAttributes(semconv.HTTPRoute(pattern))
		h(w, r)
	})
}

func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
