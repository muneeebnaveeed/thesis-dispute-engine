# backend

Go module `github.com/muneeebnaveeed/thesis-dispute-engine/backend`. Run `make` targets from
the repo root and `go` commands from here. Root `CLAUDE.md` has commands, layout, and the
machine's sharp edges.

## Layers (per bounded context under `internal/`)

- `domain`: types, invariants, state machines. Imports nothing from other layers or from
  `platform`. Table-driven tests here are the specification.
- `application`: use cases. Depends on `domain` and on port interfaces it declares itself.
- `infrastructure`: implementations of those ports (Postgres via sqlc, external systems).
- `ports/http`: handlers and DTOs; translate HTTP to application calls, nothing more.
- `platform/*`: shared kernel (config, HTTP server, telemetry). No domain knowledge.

Dependencies point inward: `ports` and `infrastructure` -> `application` -> `domain`. A
second context that needs another's data goes through that context's `application`, never
its `domain` or storage.

## Conventions

- **HTTP:** standard library only (docs/adr/0001) behind the generated strict server
  (docs/adr/0006). Implement `oapi.StrictServerInterface` in `ports/http`; `Mount` wires
  validation and `httpserver.NameSpanByRoute`. Never register routes by hand.
- **Errors:** sentinels are `errs.New(...)` values in the package that owns the rule; wrap with
  `errs.Wrap`; infrastructure maps driver errors onto application sentinels in `mapErr`.
- **Persistence:** sqlc queries in `infrastructure/postgres/queries/*.sql`, `make api:generate`,
  never hand-written SQL in Go. Money is `decimal.Decimal`. Every write path goes through
  `application.Store.WithTx`; the event log is append-only (docs/adr/0004). The API connects as
  `dispute_api`, a member of the `dispute_app` group role whose grants live in the migration
  that creates each table; `migrations/roles_test.go` fails on a table without them. Only
  `cmd/migrate` and `cmd/seed` use the owner URL (`DISPUTE_MIGRATE_DATABASE_URL`).
- **Auth (docs/adr/0009):** `auth.Bearer` resolves the tenant key into the tenant; the OpenAPI spec says
  which operations need one (top-level `security`, `security: []` to opt out) and the request
  validator enforces it through `auth.Required`. Handlers never look at `Authorization`. Tests use a
  map-backed resolver and pass `Authorization: ""` to simulate an anonymous call.
- **Tenancy (docs/adr/0008):** the tenant travels in the context (`tenant.IDFrom`); `WithTx` binds
  it to the transaction and row-level security does the filtering, so queries never add
  `tenant_id` predicates by hand. Anything that must read or write across tenants goes through an
  owner-defined view or `SECURITY DEFINER` function in a migration, never through table grants.
  Tests: `apptest.Ctx()` for unit tests, `pgtest.AppPool` for anything that must prove isolation.
  A backfill over `dispute_events` disables the append-only trigger inside the migration and
  re-enables it; `migrations/populated_test.go` proves migrations run over existing rows.
- **Regime first (docs/adr/0002):** gate on `Rules` properties (`HasProvisionalCredit()`),
  never on `if regime == X` outside the rules table. A new regime is a new row plus an entry in
  the reachability test.
- **Money:** `NUMERIC` in Postgres, `shopspring/decimal` in Go, never floats.
- **Errors:** `fmt.Errorf("pkg: what: %w", err)`; sentinel errors are exported vars.
- **Logging:** `log/slog`, key/value pairs only. Request logs already carry `request_id`,
  `trace_id`, `span_id`, `route`.
- **Telemetry:** `otel.Tracer("<import path>")` in the package doing the work; bounded span
  names, variable parts as attributes. Never add exporters in code.
- **Tests:** table-driven, `testing` only. Fast tests use `application/apptest.MemStore`;
  Postgres-backed tests use `platform/postgres/pgtest.Pool` (fresh schema per test, skipped
  without `DISPUTE_TEST_DATABASE_URL`).
- **Style:** Google/Uber Go style; `.golangci.yml` is the enforced subset. `gofmt` +
  `goimports` with local prefix `github.com/muneeebnaveeed/`.
