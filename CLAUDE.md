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

- `make ci`: everything CI runs (versions, generate-check, fmt, vet, lint, test, tidy). Run
  before every push. `make test-integration` runs the Postgres-backed suites against
  `make db-up` (CI runs them with a service container); without `DISPUTE_TEST_DATABASE_URL`
  those tests skip.
- `make generate`: regenerate sqlc queries and the OpenAPI server; commit the output.
- `make db-seed`: fixed dev accounts and transactions (idempotent; migrates first).
- `make test` / `make lint` / `make fmt-fix`: the individual steps. `make test-race` needs cgo;
  the dev machine has no C compiler, CI has one.
- `make run`: API natively on :8090. `make dev`: same with live reload (air).
- `make db-up`: PostgreSQL in Docker. `make up`: Postgres + the API image. `make docker-dev`:
  Postgres + the API in a Go toolchain container with live reload. `make otel-up`: plus Grafana LGTM
  (Grafana at http://localhost:3001, admin/admin; traces in Tempo, metrics in Mimir, logs in Loki).
- `make image`: build `backend/Dockerfile` locally.

Toolchain is pinned in `mise.toml` (Go, golangci-lint, air, sqlc); `go.mod` carries
`govulncheck` and `oapi-codegen` as `tool`s. `scripts/check-runtime-versions.sh` fails if go.mod or the Dockerfile disagree.

## Layout

```
backend/cmd/api                          wiring: config -> telemetry -> server, graceful shutdown
backend/internal/<context>/domain        entities, value objects, invariants; pure, no I/O
backend/internal/<context>/application   use cases; orchestrates domain and ports
backend/internal/<context>/infrastructure adapters: Postgres, external systems
backend/internal/<context>/ports/http    HTTP handlers implementing the generated interface (ports/http/oapi)
backend/internal/platform/{config,httpserver,telemetry,postgres,errs}  shared kernel, no domain knowledge
backend/migrations                       SQL, forward-only, embedded; applied at startup and by cmd/seed
docs/api/openapi.yaml                    the API contract; everything HTTP is generated from it
deploy/                                  compose.yml (host networking, see below), otel.env
docs/adr, docs/thesis                    decisions; the design document
```

Bounded contexts so far: `dispute` (all four layers). Create a layer only when a context
needs it.

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
  Merges are squash-only; the PR title and body become the commit on `main`, so they are
  written to the same standard.
- **Writing (docs, comments, commits, CI names):** no em dashes, no emojis.
- **Comments:** as few as possible, one line, and only to say why; never to restate what the
  code does. Doc comments on exported identifiers stay, one line each.
- **Spec first:** any HTTP change starts in `docs/api/openapi.yaml`, then `make generate`,
  then the handler. Never hand-write a route or a response type (docs/adr/0006).
- **Errors:** create failures with `errs.New(kind, code, userMessage)` and wrap with
  `errs.Wrap` for log context; ports never map errors by hand, `httpserver.ProblemFrom` does.
  New codes go into the spec's `ErrorCode` enum in the same change.
- **Decisions:** an ADR in `docs/adr/` for anything a later reader would ask "why?" about.

## Sharp edges

- **Docker uses host networking on purpose** (`network_mode: host`, `docker build --network
  host`): the dev machine's corporate VPN drops traffic on Docker's bridge, so port mappings
  and in-build `go mod download` silently time out. Do not revert to `ports:`.
- **Ports 8090 and 3001:** 8080 and 3000 are held by unrelated local services on the dev
  machine (API and Grafana moved accordingly).
- **No C compiler locally:** `go test -race` fails with "requires cgo"; use `make test` and
  let CI run the race detector.
- **`gh` accounts:** the dev machine has several. The scripts export the repo owner's token
  themselves (`gh auth token --user`), and the repo's local `credential.helper` does the
  same for `git push`, so the active `gh` account does not matter here. Set the helper once
  per clone: see `make hooks`.
