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

- **HTTP:** standard library only (docs/adr/0001). Register routes through `route()` in
  `platform/httpserver/routes.go` so spans get the pattern name; respond via `writeJSON`.
- **Regime first (docs/adr/0002):** gate on `Rules` properties (`HasProvisionalCredit()`),
  never on `if regime == X` outside the rules table. A new regime is a new row plus an entry in
  the reachability test.
- **Money:** `NUMERIC` in Postgres, `shopspring/decimal` in Go, never floats.
- **Errors:** `fmt.Errorf("pkg: what: %w", err)`; sentinel errors are exported vars.
- **Logging:** `log/slog`, key/value pairs only. Request logs already carry `request_id`,
  `trace_id`, `span_id`, `route`.
- **Telemetry:** `otel.Tracer("<import path>")` in the package doing the work; bounded span
  names, variable parts as attributes. Never add exporters in code.
- **Tests:** table-driven, `testing` only. Postgres-backed tests use a real database.
- **Style:** Google/Uber Go style; `.golangci.yml` is the enforced subset. `gofmt` +
  `goimports` with local prefix `github.com/muneeebnaveeed/`.
