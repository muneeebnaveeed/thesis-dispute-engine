package telemetry

import (
	"context"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestTraceparentRoundTripsIntoALink(t *testing.T) {
	if Traceparent(context.Background()) != "" {
		t.Fatal("no span, no traceparent")
	}
	ctx, span := sdktrace.NewTracerProvider().Tracer("t").Start(context.Background(), "queued")
	defer span.End()
	link, ok := LinkTo(Traceparent(ctx))
	if !ok || link.SpanContext.TraceID() != span.SpanContext().TraceID() {
		t.Fatalf("link = %+v ok=%v, want the queued span's trace", link, ok)
	}
	if _, ok := LinkTo("garbage"); ok {
		t.Fatal("garbage must not link")
	}
}
