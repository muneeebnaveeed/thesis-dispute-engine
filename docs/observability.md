# Observability cheat sheet

Everything below assumes `make otel-up` (Postgres, API, workbench, Grafana LGTM, synthetic probe; all on
host networking). Stop with `make otel-down`; `make otel-reset` also drops the LGTM volume. Two services
report: `dispute-engine` (the Go API) and `dispute-workbench` (the frontend server); one trace spans both
and continues into the outbox (ADR 0022).

## Where things are

| What | Where | Notes |
| --- | --- | --- |
| Grafana | http://localhost:3001 | `admin` / `GRAFANA_ADMIN_PASSWORD` (`deploy/otel.env`); anonymous access is off |
| Dashboards | Dashboards > Dispute Engine > Dispute Engine, Dispute Workbench | provisioned from `deploy/grafana/dashboards/*.json`, UI edits are not persisted |
| Alert rules | Alerting > Alert rules | provisioned from `deploy/grafana/provisioning/alerting/dispute-engine.yaml` |
| OTLP intake | 127.0.0.1:4317 (gRPC), 127.0.0.1:4318 (HTTP) | loopback only, see `OTELCOL_EXTRA_ARGS` in `deploy/compose.yml` |
| Prometheus API | http://localhost:9090 | `curl localhost:9090/api/v1/query --data-urlencode 'query=dispute_by_state'` |
| Tempo API | http://localhost:3200 | `curl -G localhost:3200/api/search --data-urlencode 'q={ span.db.system = "postgresql" }'` |
| Loki API | http://localhost:3100 | `curl -G localhost:3100/loki/api/v1/query_range --data-urlencode 'query={service_name="dispute-engine"}'` |
| API spec | http://localhost:8090/openapi.json | served from the embedded copy of `docs/api/openapi.yaml` |

Retention is `LGTM_RETENTION` (default `168h`) and is applied to all three stores: Prometheus via
`--storage.tsdb.retention.time`, Loki via compactor flags, Tempo via `deploy/lgtm/tempo-config.yaml`
(Tempo has no CLI flag for it, so that file is the image's stock config plus one override).

## Signals the service emits

Metrics (OTel names; Prometheus flattens dots to underscores and adds `_total` to counters):

| Metric | Type | Labels | Meaning |
| --- | --- | --- | --- |
| `http.server.request.duration` | histogram | `http.route`, `http.response.status_code` | from `otelhttp`; carries exemplars linking to traces |
| `http.server.problems` | counter | `code`, `status` | every problem+json response, keyed by the error taxonomy code |
| `dispute.transitions` | counter | `tenant`, `regime`, `event`, `to` | accepted state transitions |
| `dispute.idempotent_replays` | counter | `route` | requests answered from the idempotency store |
| `dispute.time_in_state` | histogram | `regime`, `state` | seconds a dispute spent in the state it just left |
| `dispute.by_state` | gauge | `tenant`, `regime`, `state` | current population, read from Postgres on each scrape |
| `dispute.notices_sent` | counter | `kind`, `outcome` | every outbox attempt handed to the mailer: `sent`, `failed`, `render-failed` |
| `dispute.notice_delivery_delay` | histogram | `kind`, `outcome` | seconds from a notice being queued to the relay accepting it |
| `dispute.outbox_backlog` | gauge | | emails queued and not yet sent, read from Postgres on each scrape |
| `go.*` | runtime | | goroutines, memory used, GC goal, allocations (contrib runtime instrumentation) |
| `db.client.operation.duration` | histogram | `pgx.operation_type` | every pgx operation (otelpgx) |

`http.server.request.duration` on the API keeps only `http.request.method`, `http.route` and
`http.response.status_code` (a metric view drops otelhttp's constant protocol and address labels), and the
request and response body-size histograms are dropped. Health checks (`/healthz`) are neither traced nor
counted.

Signals the workbench server emits (`service_name="dispute-workbench"`):

| Metric | Type | Labels | Meaning |
| --- | --- | --- | --- |
| `http.server.request.duration` | histogram | `http.request.method`, `http.route`, `http.response.status_code` | every request the frontend server answers; `http.route` has ids masked (`/otp/disputes/:id`) and every RPC is `/_serverFn/:fn` |
| `workbench.server_fn.duration` | histogram | `workbench.server_fn`, `workbench.outcome` | one series per server function; outcome is `ok`, the API's problem code, `sign-in` (session gone) or `threw` |
| `nodejs.eventloop.*`, `v8js.*` | runtime | | event loop delay percentiles and utilisation, heap, GC (runtime-node instrumentation) |

Both duration histograms use the API's bucket boundaries in seconds, so the two services' latency panels
compare like for like; a server function span also carries `tenant.slug` once the session is known.

Traces, from the browser inwards: the workbench's server span per request (`GET /otp/disputes/:id`,
`POST /_serverFn/:fn`), one span per server function it runs (`serverFn applyEvent`, with the outcome as
an attribute), an undici client span per call to the API or Keycloak (`url.full` with ids masked; the
session id never reaches a trace), then the API's server span named by route (`POST
/disputes/{disputeId}/events`) with `auth.authenticate` (credential kind, accepted) and
`ratelimit.bump` (count, limit) under it, an application span per use case (`dispute.apply_event`),
and one `otelpgx` span per statement named after the sqlc query (pool acquisition spans and the SQL text are off:
microseconds and static text that only bulked traces up). `traceparent` crosses the workbench-to-API
hop automatically. Work that outlives the request is its own trace, linked: every notice row stores
the queuing request's traceparent, and the dispatcher's `notice.deliver` span (kind, attempt, outcome)
carries a link to it and a child `smtp.send` span for the relay.

Logs: JSON on stdout and, via `otelslog`, in Loki with `trace_id`, `span_id`, `request_id`, `route`
and `status` as labels. The access line reads as the request it describes (`POST /disputes/{disputeId}/events 409`),
at INFO, or WARN for a 5xx, so the Loki list is legible without expanding a line. The workbench logs the same way (`src/server/telemetry/log.ts`): sign-in,
sign-out, back-channel logout, a session whose refresh was refused, and a server function that was
refused or threw, each with the active trace's ids and never a token or a body. Keys listed in `internal/platform/telemetry/redact.go` are replaced with
`[redacted]` before either sink sees them. `DISPUTE_LOG_LEVEL` sets the floor (`debug`, `info`,
`warn`, `error`).

## Typical questions

- Why is this request slow: dashboard latency panel > click an exemplar dot > trace, or Explore >
  Tempo with `{ resource.service.name = "dispute-engine" && duration > 500ms }`.
- What happened to dispute X: Explore > Loki `{service_name="dispute-engine"} |= "<disputeId>"`,
  then follow the `trace_id` link on a line.
- Housekeeping sweeps (idempotency keys, rate windows, web sessions, draft attachments) log at INFO only
  when they removed something; an idle system is quiet at INFO.
- Which errors are clients hitting: dashboard "Problems by code" panel, or
  `sum by (code) (rate(http_server_problems_total[5m]))`.
- Are transitions being rejected: `sum by (code) (increase(http_server_problems_total{status="409"}[1h]))`.
- Is the DB the bottleneck: Tempo `{ span.db.system = "postgresql" && duration > 50ms }`.
- What did the analyst's click do end to end: Tempo `{ resource.service.name = "dispute-workbench" &&
  name = "POST /_serverFn/:fn" }`, open one; the API and Postgres spans are in the same trace.
- Did the email for that transition go out: from the transition's trace, follow the link on
  `notice.deliver` (Tempo shows linked traces), or Explore > Tempo `{ name = "notice.deliver" &&
  span.notice.outcome != "sent" }` for the ones that did not.
- Is the outbox draining: Workbench dashboard "Outbox backlog" and "Notice delivery delay p95"; the
  alert fires when more than 20 emails have waited for 10 minutes.

## Synthetic traffic

`deploy/probe.sh` runs in the `probe` container and, every `PROBE_INTERVAL` seconds, walks four
seeded transactions through a full lifecycle plus one deliberate 409 and one deliberate 400, so the
dashboard is populated during a demo. Its requests carry `actor: probe`; it needs the seed data
from `make db-seed` (run by `make otel-up`).

## Changing things

- Dashboard: edit in the UI, then Share > Export > "Export for sharing externally" off, save the JSON
  over `deploy/grafana/dashboards/dispute-engine.json` (keep `uid: dispute-engine`). Grafana reloads
  the file within 10 seconds.
- Alerts: edit the provisioning yaml; rules are `provenance: file` so the UI shows them read only.
- New metric: create it next to the code that emits it (see `application/service.go`), keep labels
  bounded (enums, routes), never IDs.
- New span attribute: put variable data in attributes, never in span names; never an id that is a
  credential (session ids are masked before undici spans see them), never a recipient address.
- The workbench exports with the same `OTEL_*` variables as the API (`deploy/otel.env`); with no
  `OTEL_EXPORTER_OTLP_ENDPOINT` the SDK is not started and every span is a no-op.

The probe opens its disputes on a seeded account with no email or postal address ("Probe Account"), so
synthetic traffic exercises every component except the mail relay and never fills an inbox.
