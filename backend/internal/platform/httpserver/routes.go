package httpserver

import (
	"net/http"

	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
)

// NameSpanByRoute renames the server span to the matched pattern; ServeMux only sets r.Pattern after matching, so this must wrap the route handler, not the mux.
func NameSpanByRoute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Pattern != "" {
			span := trace.SpanFromContext(r.Context())
			span.SetName(r.Pattern)
			span.SetAttributes(semconv.HTTPRoute(r.Pattern))
		}
		next.ServeHTTP(w, r)
	})
}
