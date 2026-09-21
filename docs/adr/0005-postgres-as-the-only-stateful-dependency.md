# 0005: PostgreSQL is the only stateful dependency until a measured need says otherwise

**Status:** accepted, 2026-09-21

## Context

Redis came up for locks, idempotency responses, sessions, rate limiting and background work.
Each has a PostgreSQL answer at this system's scale, and a second stateful dependency costs
compose, CI, backups, failure modes and explanation.

## Decision

- Locks: none needed; optimistic versioning (ADR 0004) covers dispute writes and an advisory
  lock covers migrations. Distributed locks over Redis are explicitly rejected.
- Idempotency responses: a table, transactional with the write they protect.
- Sessions (frontend): a table or signed cookies; rate limiting: a table; background work: an
  outbox table drained with `SKIP LOCKED`; caching: nothing yet.
- Revisit when a measurement, not a preference, shows PostgreSQL is the bottleneck for one of
  these; the new dependency then gets its own ADR.

## Consequences

- Easier: one datastore to run, test, back up and reason about; every write is transactional
  with its neighbours.
- Harder: some patterns are less idiomatic on PostgreSQL (rate limiting, pub/sub) and would
  need care if load ever demanded them.
