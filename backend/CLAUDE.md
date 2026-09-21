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
- **Persistence:** sqlc queries in `infrastructure/postgres/queries/*.sql`, `make generate`,
  never hand-written SQL in Go. Money is `decimal.Decimal`. Every write path goes through
  `application.Store.WithTx`; the event log is append-only (docs/adr/0004). The API connects as
  `dispute_api`, a member of the `dispute_app` group role whose grants live in the migration
  that creates each table; `migrations/roles_test.go` fails on a table without them. Only
  `cmd/migrate` and `cmd/seed` use the owner URL (`DISPUTE_MIGRATE_DATABASE_URL`).
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
