# thesis-dispute-engine

An issuer-side **automated fraud dispute resolution engine**: the system a cardholder's bank
runs from the moment a customer disputes a transaction: provisional credit or fast refund,
chargeback toward the card network, evidence-based adjudication, customer communication,
and final settlement against a mock core-banking ledger. BSc thesis at the University of
Debrecen, Faculty of Informatics (2026–27).

The dispute lifecycle is a single finite state machine **parameterised by a regulatory
regime**: EU PSD2 card disputes and SEPA Direct Debit are first-class; US Regulation E and Z
are modelled as configuration to show the design generalises. See
[`docs/adr/0002`](docs/adr/0002-regime-parameterised-state-machine.md).

## Stack

Go (standard-library HTTP, PostgreSQL via sqlc, OpenTelemetry) · TypeScript analyst UI
(TanStack Start, Node + pnpm, Phase 4) · Docker Compose for local dependencies.

## Run it

```sh
mise install          # Go, golangci-lint, air; versions in mise.toml
make db-up            # PostgreSQL on localhost:5432
make run              # API on http://localhost:8090  →  curl localhost:8090/healthz
make dev              # same, with live reload
make ci               # what CI runs
```

`make up` runs the API from its container image alongside Postgres; `make otel-up` adds
Jaeger and the API exports traces to it (UI: http://localhost:16686). All telemetry is
configured through standard `OTEL_*` variables; see `.env.example`.

## Repository

| Path | What |
|---|---|
| `backend/` | Go module: `cmd/api`, `internal/{config,httpserver,telemetry,dispute}`, `migrations/` |
| `frontend/` | Analyst dashboard, Phase 4 |
| `docs/adr/` | Architecture decision records |
| `docs/thesis/` | The thesis document |
| `deploy/` | Compose stack and its env files |

Plan and phase schedule: Research (Sep 2026) → Backend core → Backend advanced → Frontend
→ Integration (to Feb 2027) → Writing and submission (spring 2027).
