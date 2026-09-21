# 0008: Multi-tenancy with row-level security on a shared schema

**Status:** accepted, 2026-09-21; builds on ADR 0004 and ADR 0005, informs ADR 0007

## Context

The engine should serve several issuers or PSPs (tenants) from one deployment. Tenants share the
regulatory logic and the process; they must never share data, and a missing filter in one query
must not become a data leak. Options were a database per tenant (strong isolation, operational
sprawl, cross-tenant metrics become a federation problem), a schema per tenant (same sprawl in
miniature, migrations multiplied), or one schema with a tenant column and row-level security.

## Decision

One schema; every business table carries `tenant_id`; PostgreSQL row-level security enforces
isolation against the role the API runs as.

- `tenants` holds identity and a `settings jsonb` for per-tenant knobs. Regime rules stay in code:
  they are law, not configuration.
- `tenant_id` is on `accounts`, `transactions`, `disputes`, `dispute_events` (denormalised so the
  policy needs no join) and `idempotency_keys` (whose key is now `(tenant_id, scope, key)`, because
  clients choose keys and two tenants may choose the same one). Primary keys stay global UUIDv7.
- The tenant comes from the request's authenticated principal, never from a client-controlled
  header or path segment. Until per-request authentication lands, `DISPUTE_TENANT_ID` stamps every
  request (single-tenant mode). `tenant.IDFrom(ctx)` is the only way storage learns it.
- `Store.WithTx` binds the tenant with `set_config('app.tenant_id', ..., true)`: transaction-local,
  so a pooled connection never carries a tenant past commit. Columns default to
  `current_tenant_id()`, so inserts need no change; the policy's `WITH CHECK` still verifies them.
- Policies use `tenant_id = current_tenant_id()`, where an unset setting yields NULL and therefore no
  rows: fail closed. A store call without a tenant in context is refused before reaching the database.
- Row-level security is enabled but not forced. The schema owner (migrations, seed, tests' owner
  pool) bypasses it; the API role (`dispute_app`, ADR 0004's role split) does not. The two
  cross-tenant operations the API needs, the disputes-by-state gauge and the idempotency purge, go
  through an owner-defined view and a `SECURITY DEFINER` function with a pinned `search_path`,
  rather than table access. `FORCE` was rejected because it would subject the owner too, and a
  bypass role for the owner's own functions requires `BYPASSRLS`, a superuser-only attribute.
- Business metrics carry a `tenant` label; HTTP metrics do not. Cardinality equals the tenant count,
  which is expected to stay in the tens; past that the label moves to exemplars.
- Backfills that touch the append-only log disable its trigger inside the migration's transaction
  and re-enable it before the transaction ends; `migrations/populated_test.go` runs every
  migration over pre-existing rows so a refused backfill fails in CI, not in deployment.

## Consequences

- Easier: isolation is a database guarantee tested independently of application code
  (`TestTenantIsolationUnderRLS` runs as the API role); one deployment, one migration path, one
  dashboard with a tenant selector; the unit-test store scopes by tenant the same way, so a
  forgotten tenant fails the fastest tests first.
- Harder: every table needs the column, the policy and grants, and a new table without them fails
  `TestAppRolePrivileges`; per-tenant rate limiting becomes a real requirement (ADR 0007 keeps it in
  PostgreSQL); support tooling that must read across tenants needs its own owner-side objects, not
  a switch in the API.
