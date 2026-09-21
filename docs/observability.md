# Observability cheat sheet

Everything below assumes `make otel-up` (Postgres, API, Grafana LGTM, synthetic probe; all on host
networking). Stop with `make otel-down`; `make otel-reset` also drops the LGTM volume.

## Where things are

| What | Where | Notes |
| --- | --- | --- |
| Grafana | http://localhost:3001 | `admin` / `GRAFANA_ADMIN_PASSWORD` (`deploy/otel.env`); anonymous access is off |
| Dashboard | Dashboards > Dispute Engine > Dispute Engine | provisioned from `deploy/grafana/dashboards/dispute-engine.json`, UI edits are not persisted |
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

Traces: one server span per request named by route (`POST /disputes/{disputeId}/events`), an
application span per use case (`dispute.apply_event`), and one `otelpgx` span per statement named
after the sqlc query (`GetDisputeForUpdate`, `AppendEvent`, `BEGIN`, `COMMIT`).

Logs: JSON on stdout and, via `otelslog`, in Loki with `trace_id`, `span_id`, `request_id`, `route`
and `status` as labels. Keys listed in `internal/platform/telemetry/redact.go` are replaced with
`[redacted]` before either sink sees them. `DISPUTE_LOG_LEVEL` sets the floor (`debug`, `info`,
`warn`, `error`).

## Typical questions

- Why is this request slow: dashboard latency panel > click an exemplar dot > trace, or Explore >
  Tempo with `{ resource.service.name = "dispute-engine" && duration > 500ms }`.
- What happened to dispute X: Explore > Loki `{service_name="dispute-engine"} |= "<disputeId>"`,
  then follow the `trace_id` link on a line.
- Which errors are clients hitting: dashboard "Problems by code" panel, or
  `sum by (code) (rate(http_server_problems_total[5m]))`.
- Are transitions being rejected: `sum by (code) (increase(http_server_problems_total{status="409"}[1h]))`.
- Is the DB the bottleneck: Tempo `{ span.db.system = "postgresql" && duration > 50ms }`.

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
- New span attribute: put variable data in attributes, never in span names.
