# thesis-dispute-engine

Monorepo for a BSc thesis project. What the system does is in `README.md` and the ADRs in
`docs/adr/`; this file is only how to work in the repo.

## Stack

- `backend/`: Go 1.27 (pinned in `mise.toml`), standard-library HTTP, PostgreSQL via sqlc
  (from week of 2026-09-28), `shopspring/decimal` for money, OpenTelemetry, golangci-lint v2.
- `frontend/`: TypeScript, TanStack Start, shadcn/ui, Tailwind, on Node with pnpm (Phase 4,
  from 2026-11-30; placeholder until then).
- `docs/thesis/`: the thesis document; tooling (LaTeX or Word) not yet decided.
- `deploy/`: Docker Compose for local dependencies; multi-stage distroless image in
  `backend/Dockerfile`.
- CI: GitHub Actions, one job per check (`.github/workflows/ci.yml`).

## Commands (run from the repo root)

- `make ci`: everything CI runs: versions, fmt, vet, lint, test, tidy. Run before every push.
- `make test` / `make lint` / `make fmt-fix`: the individual steps. `make test-race` needs cgo
  (a C compiler); the dev machine has none, CI has it.
- `make run`: API natively on :8090. `make dev`: same with live reload (air).
- `make db-up`: PostgreSQL in Docker. `make up`: Postgres + the API image. `make otel-up`:
  plus Jaeger; UI at http://localhost:16686 (query API is `/api/v3/...`).
- `make image`: build `backend/Dockerfile` locally.

Toolchain is pinned in `mise.toml` (Go, golangci-lint, air); `go.mod` carries `govulncheck`
as a `tool`. `scripts/check-runtime-versions.sh` fails if go.mod or the Dockerfile disagree.

## Layout

```
backend/cmd/api            main: config → telemetry → server, graceful shutdown
backend/internal/config    env-driven (DISPUTE_*), defaults match deploy/compose.yml
backend/internal/httpserver stdlib ServeMux + Chain middleware; routes.go is the routing table
backend/internal/telemetry OpenTelemetry providers, configured only via OTEL_* env
backend/internal/dispute   domain: regimes, states, transitions (pure, no I/O)
backend/migrations         SQL, forward-only (arrives week of 2026-09-28 with sqlc)
deploy/                    compose.yml (host networking, see below), otel.env
docs/adr, docs/thesis      decisions; the thesis document
```

## Per-area guides

Each area has its own CLAUDE.md with the conventions that only matter there; Claude Code
loads it when working under that path. `backend/CLAUDE.md` exists; `frontend/` and
`docs/thesis/` get theirs when they gain content. Cross-cutting rules stay in this file.

- **Commits:** `type(scope): summary`, imperative, no trailer lines. Small, bisectable.
- **Writing (docs, comments, commits, CI names):** no em dashes, no emojis.
- **Decisions:** an ADR in `docs/adr/` for anything a thesis reader would ask "why?" about.

## Sharp edges

- **Docker uses host networking on purpose** (`network_mode: host`, `docker build --network
  host`): the dev machine's corporate VPN drops traffic on Docker's bridge, so port mappings
  and in-build `go mod download` silently time out. Do not revert to `ports:`.
- **Port 8090, not 8080:** 8080 is held by an unrelated local service on the dev machine.
- **No C compiler locally:** `go test -race` fails with "requires cgo"; use `make test`
  and let CI run the race detector.
- **Jaeger v2** ignores metrics; `deploy/otel.env` sets `OTEL_METRICS_EXPORTER=none`.

## Thesis workflow

- The supervisor reviews documents on Fridays; a short note goes out every Thursday.
- Every feature lands with a paragraph and a screenshot for `docs/thesis/`; the document
  is written alongside the code, not after it (development stops mid-to-late March 2027).
- Scope pressure resolves toward *writing about what exists*, not one more feature.
