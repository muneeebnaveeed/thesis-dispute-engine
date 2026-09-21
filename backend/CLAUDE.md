# backend

Go module `github.com/muneeebnaveeed/thesis-dispute-engine/backend`. Run `make` targets from
the repo root; `go` commands from here. Root `CLAUDE.md` has the project context, commands,
layout, and the machine's sharp edges (host networking, port 8090, no cgo locally).

## Conventions


- **HTTP:** standard library only (ADR 0001). Register routes through `route()` in
  `routes.go` so the OTel span gets the pattern name. Handlers return JSON via `writeJSON`.
- **Regime first (ADR 0002):** anything that depends on jurisdiction reads it from the
  dispute's `Regime`; gate on regime *properties* (`HasProvisionalCredit()`), never on
  `if regime == X` outside the regime table itself.
- **Money:** `NUMERIC` in Postgres, `shopspring/decimal` in Go, never floats.
- **Errors:** wrap with `fmt.Errorf("pkg: what: %w", err)`; sentinel errors are exported vars.
- **Logging:** `log/slog`, JSON in prod. Request logs already carry `request_id`, `trace_id`,
  `span_id`, `route`; add key/value pairs, never format strings.
- **Telemetry:** instrument with `otel.Tracer("<package>")` / `otel.Meter(...)`; span names
  are bounded (route patterns, operation names), attributes carry the variable parts. Do
  not add exporters in code; the environment decides where telemetry goes.
- **Tests:** table-driven, `testing` only for now; Postgres-backed tests use a real
  database (dockertest or compose), no mocks of the storage layer.
- **Style:** Google/Uber Go style; golangci-lint v2 config in `.golangci.yml` is the
  enforced subset. `gofmt` + `goimports` with local-prefix `github.com/muneeebnaveeed/...`.

## Packages

- `internal/dispute` is pure: regimes, states, transitions, no I/O. Extend the regime table
  in `regime.go` and the transition table in `state.go`; every new arm must gate on a `Rules`
  property. Add the regime to the reachability test so the state set it can visit is pinned.
- `internal/httpserver` owns routing and middleware only; handlers call into domain and
  storage packages, never the other way round.
- `internal/telemetry` is wiring; instrument in the package doing the work with
  `otel.Tracer("<import path>")`.
