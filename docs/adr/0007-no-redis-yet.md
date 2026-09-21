# 0007: No Redis yet

**Status:** accepted, 2026-09-21; extends ADR 0005

## Context

Redis keeps coming up, most recently for multi-tenancy: per-tenant rate limits and quotas, frontend
sessions, locks for background sweeps, and pub/sub for pushing dispute updates to the UI. ADR 0005
made PostgreSQL the only stateful dependency on principle; this records the concrete comparison now
that the needs are known, and the triggers that would reverse it.

## Decision

Stay on PostgreSQL alone. For each candidate use:

| Need | With Redis | Here |
| --- | --- | --- |
| Concurrent writes to a dispute | Redlock | Not needed: optimistic CAS on `version` (ADR 0004) |
| One-at-a-time background sweeps | `SET NX PX` | `pg_try_advisory_xact_lock` (idempotency purge, migrations) |
| Idempotency store | TTL keys | `idempotency_keys` table, transactional with the write, swept after `DISPUTE_IDEMPOTENCY_TTL` |
| Per-tenant rate limiting | `INCR` + `EXPIRE` | `tenant_rate_windows (tenant_id, window_start, count)` with `INSERT ... ON CONFLICT DO UPDATE SET count = count + 1 RETURNING count`: one round trip, same connection, same trace |
| Frontend sessions | classic | `sessions` table with a hashed token and `expires_at`, purged by the same sweep |
| Push updates to the UI | pub/sub | `LISTEN/NOTIFY` |
| Regime rules cache | pointless | rules are compiled in |

## Consequences

- Easier: one dependency in compose, CI, `/readyz`, tracing and the write-up; every mechanism above
  participates in the request's transaction and trace; nothing to explain in the evaluation chapter
  except PostgreSQL.
- Harder: the fixed-window counter is a hot row per tenant under sustained load, and sliding windows
  are awkward in SQL.
- Reversal triggers, any one of which reopens this with its own ADR: sustained request rates where the
  counter row shows lock contention in traces (order of hundreds of requests per second per tenant);
  a requirement for sliding-window limits shared across replicas with sub-millisecond budgets; session
  counts that make the table's purge a measurable cost; or fan-out beyond what `LISTEN/NOTIFY` carries.
  A preference is not a trigger; a measurement is.
