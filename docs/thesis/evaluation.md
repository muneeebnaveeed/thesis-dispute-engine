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

`make eval` accepts `EVAL_RPS` (lifecycle iterations per second, about five requests each; default 1.5, which stays inside the tenant budget of 600 requests per minute), `EVAL_SECONDS`
(default 120) and `EVAL_KEY` (default the seeded OTP key). It writes:

- `data/<date>/correctness.json`: test counts per package and the list of transition cases.
- `data/<date>/load.json`: achieved rate, status histogram, replay count, per-route latency percentiles
  from `http_server_request_duration_seconds` over the run window.
- `data/<date>/environment.txt`: Go, Postgres and image versions, CPU and memory of the machine.

Runs are committed; the thesis cites the directory name.

| Run | What the API did per transition | Create p99 | Event p99 | Read p99 |
| --- | --- | --- | --- | --- |
| `2026-09-21_2304` | state row, event log | 25 ms | 5 ms | 5 ms |
| `2026-09-22_0126` | plus clocks, ledger, simulated core call, notices, risk assessment | 25 ms | 14 ms | 5 ms |

Server-side percentiles from Prometheus at 7.6 requests/s on the laptop; the second run shows the
cost of the six domain components landing in the same transaction.

## Limitations to state

Single machine, synthetic traffic, one Postgres, no network between browser and API beyond
loopback; numbers show the shape of behaviour, not capacity planning.
