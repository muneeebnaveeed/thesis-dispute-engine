package httpserver

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
)

// NameSpanByRoute renames the server span to the matched pattern and puts http.route on the request metrics;
// ServeMux only sets r.Pattern after matching, so this must wrap the route handler, not the mux. otelhttp records
// its histogram in the outer handler, which is why the route reaches it through the Labeler rather than directly.
func NameSpanByRoute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Pattern != "" {
			span := trace.SpanFromContext(r.Context())
			span.SetName(r.Pattern)
			span.SetAttributes(semconv.HTTPRoute(r.Pattern))
			if labeler, ok := otelhttp.LabelerFromContext(r.Context()); ok {
				labeler.Add(semconv.HTTPRoute(r.Pattern))
			}
			setRoute(r.Context(), r.Pattern)
		}
		next.ServeHTTP(w, r)
	})
}
