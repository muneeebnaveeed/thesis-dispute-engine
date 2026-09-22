# Evaluation protocol

What the thesis measures, how it is produced, and where it lands. Everything is reproducible from
the repository with `make eval` against the local stack; the numbers used in the text are copied
from a dated run under `data/`, never typed by hand.

## Questions and measures

| Question | Measure | Produced by |
| --- | --- | --- |
| Does the engine apply each regime's rules correctly? | table-driven transition tests per regime: accepted and refused transitions, appeal limits, reachability pins | `go test -json ./internal/dispute/domain/` |
| Is the log append-only and are concurrent writers serialised? | database refusal of update, delete and truncate; eight writers on one dispute produce exactly one winner per version | `go test ./internal/dispute/infrastructure/postgres/` |
| Are retries safe? | identical replays return the stored response, mismatched bodies are refused | same, plus `dispute_idempotent_replays_total` under load |
| Is tenant isolation a database guarantee? | cross-tenant reads, writes, replays and raw queries as the API role return nothing | `TestTenantIsolationUnderRLS`, browser suite |
| How does it perform under sustained load? | p50, p95, p99 latency per route, achieved requests per second, error share, 409 share | `cmd/eval` load run read back from Prometheus |
| Is it operable? | traces per request with statement spans, problems by code, disputes by state, alert rules | dashboard and alert provisioning; screenshots |

## Running it

```sh
make otel-up            # Postgres, API, Keycloak-free profile with Grafana LGTM; seeds the tenants
make eval               # correctness suites, then a load run, then Prometheus queries; writes data/<date>/
```

`make eval` accepts `EVAL_RPS` (lifecycle iterations per second, about five requests each; default 1.5, which stays inside the tenant budget of 600 requests per minute; raise `DISPUTE_RATE_PER_MINUTE` on the API to go faster), `EVAL_SECONDS`
(default 120), `EVAL_KEY` (default the seeded OTP key), `EVAL_SILENT=1` (disputes on the probe account, which
has no address, so the run sends no mail) and `EVAL_WORKBENCH=http://localhost:3002` with `EVAL_PAGES` (page
loads per second through the frontend server, signed in as the seeded analyst; the cookie comes from
`frontend/scripts/session-cookie.ts`). It writes:

- `data/<date>/correctness.json`: test counts per package and the list of transition cases.
- `data/<date>/load.json`: achieved rate, status histogram, replay count, per-route latency percentiles
  from `http_server_request_duration_seconds` over the run window; with a workbench, page and server-function
  percentiles for it; and the runtime picture of both processes (goroutines, heap, Postgres p95, event loop
  delay and utilisation, outbox backlog and delivery delay).
- `data/<date>/environment.txt`: Go, Postgres and image versions, CPU and memory of the machine.

Runs are committed; the thesis cites the directory name.

| Run | What the API did per transition | Create p99 | Event p99 | Read p99 |
| --- | --- | --- | --- | --- |
| `2026-09-21_2304` | state row, event log | 25 ms | 5 ms | 5 ms |
| `2026-09-22_0126` | plus clocks, ledger, simulated core call, notices, risk assessment | 25 ms | 14 ms | 5 ms |
| `2026-09-22_1257` | the same, at 20 requests/s with 3 workbench pages/s beside it | 24 ms | 9 ms | 5 ms |

Server-side percentiles from Prometheus; the first two runs at 7.6 requests/s on the laptop, the second showing
the cost of the six domain components landing in the same transaction. The third run (2439 API requests, 359
server-rendered pages, one signed-in analyst) adds the frontend server: a dispute page renders in 21 ms at
p95, of which the two server functions it runs take 9 ms each against the API; the workbench's event loop
stayed under 12 ms p99 delay at 3 percent utilisation, the API at 62 goroutines and 2 MB of heap, Postgres
under 1 ms at p95, the outbox empty. Every dispute-page trace in the window contained both services, no
trace carried an error and none exceeded 50 ms.

## Limitations to state

Single machine, synthetic traffic, one Postgres, no network between browser and API beyond
loopback; numbers show the shape of behaviour, not capacity planning.
