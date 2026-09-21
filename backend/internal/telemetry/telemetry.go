// Package telemetry wires OpenTelemetry tracing and metrics.
//
// Configuration is entirely through the standard OTEL_* environment variables
// (OTEL_SERVICE_NAME, OTEL_EXPORTER_OTLP_ENDPOINT, OTEL_TRACES_EXPORTER,
// OTEL_METRICS_EXPORTER, OTEL_RESOURCE_ATTRIBUTES, ...), so any collector or
// backend that speaks OTLP works without code changes. When none of them is
// set, providers are still installed (instrumentation runs, spans and
// metrics are created) but nothing is exported and nothing is logged about
// it, so a plain `make run` stays quiet.
package telemetry

import (
	"context"
	"errors"
	"log/slog"
	"os"

	"go.opentelemetry.io/contrib/exporters/autoexport"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
)

// ServiceName is the default OTEL_SERVICE_NAME when the environment sets none.
const ServiceName = "dispute-engine"

// Version is stamped onto every resource; overridden at build time via
// -ldflags "-X .../telemetry.Version=<git sha>".
var Version = "dev"

// Shutdown flushes and stops every provider Setup installed.
type Shutdown func(context.Context) error

// Setup installs global tracer and meter providers and W3C context
// propagation. It returns a Shutdown that must be called before exit so the
// last spans and metrics are flushed.
func Setup(ctx context.Context, logger *slog.Logger) (Shutdown, error) {
	res, err := resource.Merge(resource.Default(), resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(serviceName()),
		semconv.ServiceVersion(Version),
	))
	if err != nil {
		return nil, err
	}

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	shutdowns := make([]Shutdown, 0, 2)

	tracerOpts := []sdktrace.TracerProviderOption{sdktrace.WithResource(res)}
	meterOpts := []metric.Option{metric.WithResource(res)}

	if exportEnabled() {
		spanExporter, err := autoexport.NewSpanExporter(ctx)
		if err != nil {
			return nil, err
		}
		tracerOpts = append(tracerOpts, sdktrace.WithBatcher(spanExporter))

		reader, err := autoexport.NewMetricReader(ctx)
		if err != nil {
			return nil, err
		}
		meterOpts = append(meterOpts, metric.WithReader(reader))

		logger.Info("telemetry: exporting",
			"traces", envOr("OTEL_TRACES_EXPORTER", "otlp"),
			"metrics", envOr("OTEL_METRICS_EXPORTER", "otlp"),
			"endpoint", os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		)
	}

	tp := sdktrace.NewTracerProvider(tracerOpts...)
	otel.SetTracerProvider(tp)
	shutdowns = append(shutdowns, tp.Shutdown)

	mp := metric.NewMeterProvider(meterOpts...)
	otel.SetMeterProvider(mp)
	shutdowns = append(shutdowns, mp.Shutdown)

	return func(ctx context.Context) error {
		var errs []error
		// Reverse order: stop metrics before the tracer that may still be
		// recording the shutdown itself.
		for i := len(shutdowns) - 1; i >= 0; i-- {
			errs = append(errs, shutdowns[i](ctx))
		}
		return errors.Join(errs...)
	}, nil
}

// SpanIDs returns the trace and span IDs of the current span for log
// correlation, or empty strings when there is no recording span.
func SpanIDs(ctx context.Context) (traceID, spanID string) {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return "", ""
	}
	return sc.TraceID().String(), sc.SpanID().String()
}

// exportEnabled reports whether the environment asked for any exporter. With
// nothing set, autoexport would default to OTLP against localhost:4318 and
// log connection failures forever; staying silent is the better default for a
// service most often run without a collector.
func exportEnabled() bool {
	for _, key := range []string{
		"OTEL_EXPORTER_OTLP_ENDPOINT",
		"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT",
		"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT",
		"OTEL_TRACES_EXPORTER",
		"OTEL_METRICS_EXPORTER",
	} {
		if os.Getenv(key) != "" {
			return true
		}
	}
	return false
}

func serviceName() string { return envOr("OTEL_SERVICE_NAME", ServiceName) }

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
