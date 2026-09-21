package httpserver

import (
	"net/http"

	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
)

// registerRoutes is the routing table. Method/path patterns are Go 1.22+
// ServeMux syntax; keep it as the one list a reader can scan.
func registerRoutes(mux *http.ServeMux) {
	route(mux, "GET /healthz", handleHealthz)
}

// route registers a handler and, once ServeMux has matched it, renames the
// server span to the route pattern and tags it with http.route. ServeMux only
// fills r.Pattern after matching, which is why this cannot live in the outer
// middleware chain; doing it per route keeps span names bounded to the
// routing table instead of the raw URL space.
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
