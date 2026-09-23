# Observability cheat sheet

Everything below assumes `make otel:up` (Postgres, API, workbench, Grafana LGTM, synthetic probe; all on
host networking). Stop with `make otel:down`; `make otel:reset` also drops the LGTM volume. Two services
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

Both duration histograms use one set of bucket boundaries in seconds (`telemetry.LatencyBuckets`, from 1 ms
up; otelhttp's default starts at 5 ms, above most of this API's reads), so the two services' latency panels
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

## Interpreting the dashboards

The three signals answer different questions and are used in that order: metrics say whether something is
wrong and for how many, traces say where the time or the error was, logs say what exactly happened in one
case. Counting with traces is wrong (they are sampled in any real deployment) and counting with logs is
expensive; debugging one request with metrics is impossible, because aggregation has already discarded it.

### Percentiles

p95 = 21 ms means 95 requests in 100 finished within 21 ms and says nothing about the other five, so read
three numbers together. p50 is the typical path and moves when the work itself changes; p95 is what to set
expectations and alerts on, because it reacts to contention while staying stable; p99 is where saturation
shows first, and a flat p50 under a climbing p99 is the signature of queueing. Averages hide all of this and
are not on these dashboards on purpose.

Two traps. Percentiles are interpolated inside histogram buckets, so if several unrelated routes report the
same number, suspect the buckets: every API read reported 2.5 ms p50 until `telemetry.LatencyBuckets` gained
1 ms and 2.5 ms boundaries. And percentiles neither average across instances nor add across calls: a page
making two 9 ms calls is slower than 18 ms at p95, because the chance of hitting one slow call compounds.

### Where the milliseconds are

The same dispute page at p95 in run `docs/thesis/data/2026-09-22_1257`:

| Layer | p95 | What it adds |
| --- | --- | --- |
| `GET /disputes/{disputeId}` (API) | 4.5 ms | query, row-level security, serialisation |
| `serverFn getDispute` (workbench) | 9.2 ms | the hop to the API, session token, deserialisation |
| `GET /otp/disputes/:id` (page) | 20.6 ms | SSR, session lookup, the other loaders |

Each layer roughly doubles the one below. Comparing the three converts "the page is slow" into "which layer
added the milliseconds" before anyone opens a trace.

### Request panels: rate, errors, duration

Rate is context, not health: a latency rise with a rate rise is load, without one it is contention or a
dependency. Errors are business outcomes here, not just 5xx: `http.server.problems` by `code` on the API and
`workbench.outcome` on the workbench name the cause, and the difference between `invalid-transition` (clients
are confused or retrying wrongly), `rate-limited` (one tenant is hammering) and `core-declined` (the banking
core refuses) decides whether anyone is woken up. On the workbench, `sign-in` rising means sessions are dying
(Keycloak, or a rotated `SESSION_SECRET`) and `threw` means a bug.

### Runtime panels

These are never the first place to look; they answer whether the process itself is the problem.

The workbench runs on one thread, so its health is that thread. **Event loop delay** is how long a callback
waited to run and is the saturation signal: under 10 ms p99 is healthy, 50 ms is felt by users, hundreds of
ms means the thread is blocked and every concurrent request pays it. **Event loop utilisation** is the
fraction of wall time that thread worked, 0 to 1; under 0.5 there is headroom, above 0.7 delay climbs
non-linearly. The load run sat at 11.5 ms and 0.03, meaning the workbench spends its time waiting on the API,
which is what a proxy-shaped service should do. **Heap used** matters by shape: a sawtooth is healthy, a
staircase that never falls is a leak, and heap high together with GC duration rising is the chain that ends
as event loop delay and then as page latency.

The API is multi-threaded and shows different signals. **Goroutines** is the leak detector and the
backpressure signal: stable and proportional to concurrency is healthy (62 at 20 requests/s), a slow
monotonic rise means something never returns, a spike means requests are piling up behind Postgres, the core
or SMTP. **Memory used against the GC goal** is the Go sawtooth; a band rising under flat load is a leak.
**Postgres p95** from the pgx histogram is the "is it me or the database" panel: it was under 1 ms while
`POST /disputes` was 21 ms, which places those milliseconds in the six domain components, not the database.

### The outbox panels

Nobody waits on an HTTP response here, so read the pair. Backlog is queue depth and is a leading indicator:
flat and non-zero is steady state, rising is the alarm, minutes before anyone notices a missing email.
Backlog rising with flat delivery delay means more work arrived; both rising means the drain is broken.

### Four situations

- The page feels slow: workbench p95 by route, then the three-layer comparison above, then one trace from
  that route. All routes slow at once points at event loop delay instead.
- Users are getting errors: problems by code and outcomes by function, then Loki filtered to that code, then
  the trace id on the line.
- Memory grew overnight: heap and goroutines over 24 hours. Rising together is a leaked goroutine holding
  memory; heap alone is a cache or a buffer. Neither shows in request panels until the latency cliff.
- An email never arrived: Loki for the dispute id, or Tempo `{ name = "notice.deliver" &&
  span.notice.outcome != "sent" }`, then the link back to the request that queued it.

### Principles

Alert on symptoms (5xx share, p99), never on causes (CPU, heap), which are diagnostics and are often high for
benign reasons. There is no universal good number; the honest statement is always "this route was 4.8 ms in
the last run and is 40 ms now", which is why the load runs under `docs/thesis/data/` are committed. Every
panel should match a sentence someone says during an incident: the request body-size histograms were real
data answering no question and were dropped, while the backlog gauge answers "will the emails go out".
Keep labels and span names bounded; unbounded cardinality is the one mistake that takes the stack down.

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

## Load

`make thesis:eval` (docs/thesis/evaluation.md) is the load generator: paced dispute lifecycles through the API with a
tenant key and, with `EVAL_WORKBENCH`, server-rendered pages through the workbench as a signed-in analyst,
then the run's numbers read back from Prometheus into `docs/thesis/data/<date>/load.json`. It is the quickest
way to see every panel on both dashboards move at once.

## Synthetic traffic

`deploy/probe.sh` runs in the `probe` container and, every `PROBE_INTERVAL` seconds, walks four
seeded transactions through a full lifecycle plus one deliberate 409 and one deliberate 400, so the
dashboard is populated during a demo. Its requests carry `actor: probe`; it needs the seed data
from `make db:seed` (run by `make otel:up`).

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
