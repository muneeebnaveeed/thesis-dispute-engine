# 0003: OpenTelemetry instrumentation from the first commit

**Status:** accepted, 2026-09-21

## Context

Observability is easy to add on day one and painful to retrofit: once handlers, storage
calls and background jobs exist without context propagation, threading a trace through them
is a cross-cutting rewrite. The engine's interesting behaviour (a dispute crossing state
boundaries, SLA deadlines firing, ledger postings) is exactly what traces and metrics make
legible, both for debugging and for evaluating the system's behaviour.

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
- `slog` records fan out to stdout and, through the `otelslog` bridge, to the OTLP log
  exporter with trace context attached; request logs also carry `trace_id` and `span_id`.
- Local viewing is Grafana LGTM (Tempo, Mimir, Loki) via `make otel-up`; production points the
  same variables at whatever backend is in use.

## Consequences

- Easier: any OTLP backend, zero code change; every future package gets a tracer with one
  line; real traces of a dispute lifecycle are available for evaluation.
- Harder: a dependency footprint (otel SDK, exporters) in an otherwise lean module; contributors
  must keep span names bounded and put variable parts in attributes. The local stack is a single
  all-in-one image, not a production topology.
