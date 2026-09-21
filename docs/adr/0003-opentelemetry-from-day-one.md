# 0003: OpenTelemetry instrumentation from the first commit

**Status:** accepted, 2026-09-21

## Context

Observability is easy to add on day one and painful to retrofit: once handlers, storage
calls and background jobs exist without context propagation, threading a trace through them
is a cross-cutting rewrite. The engine's interesting behaviour (a dispute crossing state
boundaries, SLA deadlines firing, ledger postings) is exactly what traces and metrics make
legible, both for debugging and for the thesis's evaluation chapter.

## Decision

Instrument with OpenTelemetry from the first commit, and configure it **only** through the
standard `OTEL_*` environment variables (`contrib/exporters/autoexport`), never in code:

- `internal/telemetry.Setup` installs global tracer and meter providers with a resource
  (`service.name`, `service.version`) and W3C trace-context + baggage propagation.
- With no `OTEL_*` set, providers are still installed, spans and metrics are created, but
  nothing is exported and nothing is logged. A plain `make run` stays quiet.
- `otelhttp` wraps the whole HTTP stack; matched routes rename the server span to their
  pattern (`GET /disputes/{id}`) and tag `http.route`, so span cardinality is bounded by the
  routing table.
- Request logs carry `trace_id` and `span_id`, so a log line and its trace are one search apart.
- Local viewing is Jaeger v2 via `make otel-up`; production points the same variables at
  whatever backend is in use.

## Consequences

- Easier: any OTLP backend, zero code change; every future package gets a tracer with one
  line; the evaluation chapter can show real traces of a dispute lifecycle.
- Harder: a dependency footprint (otel SDK, exporters) in an otherwise lean module; contributors
  must keep span names bounded and put variable parts in attributes. Metrics have no local
  sink yet (Jaeger is traces-only); add a collector or Prometheus when a metric matters.
