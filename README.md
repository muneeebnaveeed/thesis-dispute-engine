# dispute-engine

An issuer-side automated fraud dispute resolution engine: the system a cardholder's bank runs
from the moment a customer disputes a transaction. It issues provisional credit or a fast
refund, files chargebacks toward the card network, adjudicates on evidence, communicates with
the customer, and settles against a core-banking ledger.

The dispute lifecycle is one finite state machine parameterised by a regulatory regime. EU
PSD2 card disputes and SEPA Direct Debit are first-class; US Regulation E and Z are modelled
as configuration to show the design generalises. See
[`docs/adr/0002`](docs/adr/0002-regime-parameterised-state-machine.md).

## Stack

Go (standard-library HTTP, PostgreSQL via sqlc, OpenTelemetry) · TypeScript analyst UI
(TanStack Start, Node + pnpm) · Docker Compose for local dependencies.

## Run it

Requires `mise`, Docker with Compose v2, `make`. Optional: a C compiler for `go test -race`.

```sh
mise install          # Go, golangci-lint, air; versions in mise.toml
make db-up            # PostgreSQL on localhost:5432 (plus the API's login role)
make run              # migrate, then the API on http://localhost:8090  ->  curl localhost:8090/healthz
make dev              # same, with live reload
make ci               # what CI runs
make help             # every target, including PR and stack tooling
```

`make up` runs the API from its container image alongside Postgres; `make docker-dev` runs
the API in a Go toolchain container with live reload instead; `make otel-up` adds a Grafana LGTM
stack and the API exports traces, metrics and logs to it (http://localhost:3001). All telemetry is configured through
standard `OTEL_*` variables; see `.env.example`.

## Repository

| Path | What |
|---|---|
| `backend/` | Go module: `cmd/{api,migrate,seed}`, `internal/<context>/{domain,application,infrastructure,ports}`, `internal/platform`, `migrations/` |
| `frontend/` | Analyst dashboard (placeholder) |
| `docs/adr/` | Architecture decision records |
| `docs/thesis/` | Long-form design document |
| `deploy/` | Compose stack and its env files |
