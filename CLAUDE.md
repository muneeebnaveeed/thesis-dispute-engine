# dispute-engine

How to work in this repo. What the system does is in `README.md` and `docs/adr/`.

## Stack

- `backend/`: Go 1.27 (pinned in `mise.toml`), standard-library HTTP, PostgreSQL via sqlc,
  `shopspring/decimal` for money, OpenTelemetry, golangci-lint v2.
- `frontend/`: TypeScript, TanStack Start, shadcn/ui, Tailwind, Node with pnpm (placeholder).
- `docs/thesis/`: long-form design document; tooling not yet decided.
- `deploy/`: Docker Compose for local dependencies; distroless image in `backend/Dockerfile`.
- CI: GitHub Actions, one job per check (`.github/workflows/ci.yml`).

## Commands (run from the repo root)

- `make ci`: everything CI runs (versions, fmt, vet, lint, test, tidy). Run before every push.
- `make test` / `make lint` / `make fmt-fix`: the individual steps. `make test-race` needs cgo;
  the dev machine has no C compiler, CI has one.
- `make run`: API natively on :8090. `make dev`: same with live reload (air).
- `make db-up`: PostgreSQL in Docker. `make up`: Postgres + the API image. `make docker-dev`:
  Postgres + the API in a Go toolchain container with live reload. `make otel-up`: plus Jaeger,
  UI at http://localhost:16686 (query API under `/api/v3/`).
- `make image`: build `backend/Dockerfile` locally.

Toolchain is pinned in `mise.toml` (Go, golangci-lint, air); `go.mod` carries `govulncheck`
as a `tool`. `scripts/check-runtime-versions.sh` fails if go.mod or the Dockerfile disagree.

## Layout

```
backend/cmd/api                          wiring: config -> telemetry -> server, graceful shutdown
backend/internal/<context>/domain        entities, value objects, invariants; pure, no I/O
backend/internal/<context>/application   use cases; orchestrates domain and ports
backend/internal/<context>/infrastructure adapters: Postgres, external systems
backend/internal/<context>/ports/http    HTTP handlers and DTOs for the context
backend/internal/platform/{config,httpserver,telemetry}  shared kernel, no domain knowledge
backend/migrations                       SQL, forward-only
deploy/                                  compose.yml (host networking, see below), otel.env
docs/adr, docs/thesis                    decisions; the design document
```

Bounded contexts so far: `dispute`. Only `domain` exists until a context needs the other
layers; do not create empty ones.

## Agent workflow

Skills in `.claude/skills/`: `validate` (scoped checks, `make ci`), `commit` (message rules,
hook), `create-pr` (`scripts/create-pr`, never raw `gh pr create`), `fix-ci`
(`scripts/fetch-ci-logs` into `notes/pr-<n>/`), `review-comments`
(`scripts/fetch-pr-comments`), `stack` (`scripts/stack show|restack|retarget` for stacked PRs). `make hooks` once per clone installs the pre-commit hook.
`notes/` is gitignored scratch space for those scripts.

## Per-area guides

Each area has its own CLAUDE.md, loaded when working under that path. `backend/CLAUDE.md`
exists; `frontend/` and `docs/thesis/` get theirs when they gain content.

- **Commits:** `type(scope): summary`, imperative, no trailer lines. Small, bisectable.
- **Writing (docs, comments, commits, CI names):** no em dashes, no emojis.
- **Comments:** as few as possible, one line, and only to say why; never to restate what the
  code does. Doc comments on exported identifiers stay, one line each.
- **Decisions:** an ADR in `docs/adr/` for anything a later reader would ask "why?" about.

## Sharp edges

- **Docker uses host networking on purpose** (`network_mode: host`, `docker build --network
  host`): the dev machine's corporate VPN drops traffic on Docker's bridge, so port mappings
  and in-build `go mod download` silently time out. Do not revert to `ports:`.
- **Port 8090, not 8080:** 8080 is held by an unrelated local service on the dev machine.
- **No C compiler locally:** `go test -race` fails with "requires cgo"; use `make test` and
  let CI run the race detector.
- **`gh` accounts:** the dev machine has several; the scripts refuse to run unless the active
  account owns the repo (`gh auth switch --user muneeebnaveeed`).
- **Jaeger v2** ignores metrics; `deploy/otel.env` sets `OTEL_METRICS_EXPORTER=none`.
