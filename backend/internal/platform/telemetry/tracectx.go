package telemetry

import (
	"context"

	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type carrier map[string]string

func (c carrier) Get(key string) string { return c[key] }
func (c carrier) Set(key, value string) { c[key] = value }
func (c carrier) Keys() []string        { return []string{"traceparent"} }
func (c carrier) has() bool             { return c["traceparent"] != "" }
func (c carrier) traceparent() string   { return c["traceparent"] }
func (c carrier) with(v string) carrier { c["traceparent"] = v; return c }
func (c carrier) into(ctx context.Context) context.Context {
	return propagation.TraceContext{}.Extract(ctx, c)
}

// Traceparent is the W3C trace context of the span active in ctx, or "" when nothing is being traced; stored on
// work that outlives its request so a later span can link back to it.
func Traceparent(ctx context.Context) string {
	c := carrier{}
	propagation.TraceContext{}.Inject(ctx, c)
	return c.traceparent()
}

// LinkTo turns a stored traceparent back into a span link; false when it is empty or unparsable.
func LinkTo(traceparent string) (trace.Link, bool) {
	c := carrier{}.with(traceparent)
	if !c.has() {
		return trace.Link{}, false
	}
	sc := trace.SpanContextFromContext(c.into(context.Background()))
	if !sc.IsValid() {
		return trace.Link{}, false
	}
	return trace.Link{SpanContext: sc}, true
}
