# 0004: Persistence as a state row plus an append-only event log

**Status:** accepted, 2026-09-21

## Context

A dispute's current state must be cheap to read and safe to update under concurrency, and
every transition must remain auditable afterwards: regulators, the analyst UI and the
evaluation of this system all ask "how did it get here". Two designs were considered: pure
event sourcing (state rebuilt by replaying events) and a state row alongside a log.

## Decision

- `disputes` holds the current state, appeals count and a `version`; `dispute_events` holds
  one row per transition with a per-dispute `seq`, the from and to states, actor, payload,
  the idempotency key that caused it, and the trace ID of the request.
- The log is append-only **at the database**: a trigger refuses `UPDATE`, `DELETE` and
  `TRUNCATE`. Convention is not relied on.
- Concurrency is optimistic. `ApplyEvent` reads the row, runs the domain state machine in
  memory, then `UPDATE ... WHERE version = $expected`; zero rows means a concurrent writer won
  and the caller gets a conflict. `UNIQUE (dispute_id, seq)` is the second, independent
  guard. No row locks: a lock would turn "fail fast, re-read, decide again" into "wait on a
  slow request".
- Client retries are idempotent through an `Idempotency-Key` header. Inside the same
  transaction as the write, the key is looked up in `idempotency_keys` (scoped per operation);
  a hit with the same request hash returns the stored response, a hit with a different hash
  is refused, a miss stores the response before commit.
- Identifiers are UUIDv7, generated in Go: time-ordered for index locality; no compliance
  regime constrains the format.

## Consequences

- Easier: reads are one row; the FSM check happens on fresh data; the log answers audit
  questions and could rebuild state if ever needed; every event row links to its trace.
- Harder: two writes per transition; the CAS forces callers to handle 409 by re-reading.
  Schema changes to the log must be additive.
- The application connects as the table owner today, so the trigger is the only append-only
  enforcement; a dedicated `dispute_app` role with `INSERT, SELECT` on the log is a
  deployment step still to do.
