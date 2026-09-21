package telemetry

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

func TestSetupWithoutEnvInstallsProvidersAndExportsNothing(t *testing.T) {
	for _, key := range []string{"OTEL_EXPORTER_OTLP_ENDPOINT", "OTEL_TRACES_EXPORTER", "OTEL_METRICS_EXPORTER"} {
		t.Setenv(key, "")
	}
	shutdown, err := Setup(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("Setup() error = %v", err)
	}
	t.Cleanup(func() { _ = shutdown(context.Background()) })

	// A span is still recorded, so instrumentation is live even without an exporter.
	_, span := otel.Tracer("test").Start(context.Background(), "op")
	if !span.SpanContext().IsValid() {
		t.Error("expected a valid span context from the installed provider")
	}
	span.End()
}

func TestSetupHonoursConsoleExporterEnv(t *testing.T) {
	t.Setenv("OTEL_TRACES_EXPORTER", "console")
	t.Setenv("OTEL_METRICS_EXPORTER", "none")
	shutdown, err := Setup(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("Setup() error = %v", err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Errorf("shutdown error = %v", err)
	}
}

func TestSpanIDs(t *testing.T) {
	if tid, sid := SpanIDs(context.Background()); tid != "" || sid != "" {
		t.Errorf("SpanIDs(no span) = %q,%q, want empty", tid, sid)
	}
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: trace.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		SpanID:  trace.SpanID{1, 2, 3, 4, 5, 6, 7, 8},
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)
	tid, sid := SpanIDs(ctx)
	if tid != "0102030405060708090a0b0c0d0e0f10" || sid != "0102030405060708" {
		t.Errorf("SpanIDs = %q,%q", tid, sid)
	}
}
