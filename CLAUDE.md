# dispute-engine

How to work in this repo. What the system does is in `README.md` and `docs/adr/`.

## Stack

- `backend/`: Go 1.27 (pinned in `mise.toml`), standard-library HTTP, PostgreSQL via sqlc,
  `shopspring/decimal` for money, OpenTelemetry, golangci-lint v2.
- `frontend/`: TypeScript, TanStack Start (React 19, SSR via Nitro), Tailwind v4, oxlint and oxfmt,
  vitest; Node and pnpm pinned in `mise.toml`.
- `docs/thesis/`: long-form design document; tooling not yet decided.
- `deploy/`: Docker Compose for local dependencies; distroless image in `backend/Dockerfile`.
- CI: GitHub Actions, one job per area (backend checks, backend test, frontend checks, images) with a named step per check (`.github/workflows/ci.yml`).

## Commands (run from the repo root)

- `make eval`: the thesis evaluation run (correctness suites, paced load, Prometheus read-back) into
  `docs/thesis/data/<date>/`; needs `make otel-up`. Runs are committed and cited by directory name.
- `make e2e`: the Playwright browser suite against the full local stack; in GitHub it is the
  separate `e2e` workflow, informative on PRs and on main, never a required check.
- `make ci`: everything the `ci` workflow runs (versions, generate-check, fmt, vet, lint, test, tidy, fe-check). Run
  before every push. Tests are split by build tag: `make test` is unit only (in-memory store,
  real handlers, no database, no network); `make test-integration` compiles the
  `//go:build integration` files and runs them against `make db-up` (CI uses a service
  container), failing loudly if the database is absent. `make cover` prints the coverage
  table CI posts on every PR.
- `make generate`: regenerate sqlc queries and the OpenAPI server; commit the output.
- `make migrate`: apply pending migrations as the schema owner; `make run` does it first. The
  API only checks the schema and refuses to start behind it.
- `make db-seed`: fixed dev accounts and transactions (idempotent; migrates first).
- `make test` / `make lint` / `make fmt-fix`: the individual steps. `make test-race` needs cgo;
  the dev machine has no C compiler, CI has one.
- `make run`: API natively on :8090. `make dev`: same with live reload (air).
- `make db-up`: PostgreSQL in Docker. `make up`: Postgres + the API image. `make docker-dev`:
  Postgres + the API in a Go toolchain container with live reload. `make otel-up`: plus Grafana LGTM
  (Grafana at http://localhost:3001, admin/admin; traces in Tempo, metrics in Mimir, logs in Loki)
  plus a synthetic probe. Cheat sheet: `docs/observability.md`. `make auth-up`: plus Keycloak with
  one realm per seeded tenant (`otp`, `erste`; analyst/analyst; admin console admin/admin on :8180).
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
backend/migrations                       SQL, forward-only, embedded; applied by cmd/migrate and cmd/seed, checked by cmd/api
frontend/                                TanStack Start app (pnpm); see frontend/README.md and frontend/CLAUDE.md
docs/api/openapi.yaml                    the API contract; everything HTTP is generated from it
deploy/                                  compose.yml (host networking, see below), otel.env, probe.sh, grafana/ and lgtm/ provisioning
backend/internal/websession              opaque session store behind /internal/sessions (frontend's, ADR 0011)
docs/adr, docs/thesis                    decisions; the thesis outline, figure register, evaluation protocol and runs (make eval)
docs/authentication.md, docs/operations.md, docs/observability.md  how callers become a tenant; operator runbooks; where telemetry lives
```

Bounded contexts so far: `dispute` (all four layers). Create a layer only when a context
needs it.

## Agent workflow

Skills in `.claude/skills/`: `validate` (scoped checks, `make ci`), `commit` (message rules,
hook), `create-pr` (`scripts/create-pr`, never raw `gh pr create`), `tenant-ops` (`scripts/tenant`), `fix-ci`
(`scripts/fetch-ci-logs` into `notes/pr-<n>/`), `review-comments`
(`scripts/fetch-pr-comments`), `stack` (`scripts/stack show|restack|retarget` for stacked PRs). `make hooks` once per clone installs the pre-commit hook.
`notes/` is gitignored scratch space for those scripts.

## Per-area guides

Each area has its own CLAUDE.md, loaded when working under that path. `backend/CLAUDE.md` and
`frontend/CLAUDE.md` exist; `docs/thesis/` gets one when it gains content.

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
- **Ports 8090, 3001, 3002, 8180:** 8080 and 3000 are held by unrelated local services on the dev
  machine (API and Grafana moved accordingly); the frontend is on 3002 and Keycloak on 8180.
- **No C compiler locally:** `go test -race` fails with "requires cgo"; use `make test` and
  let CI run the race detector.
- **`gh` accounts:** the dev machine has several. The scripts export the repo owner's token
  themselves (`gh auth token --user`), and the repo's local `credential.helper` does the
  same for `git push`, so the active `gh` account does not matter here. Set the helper once
  per clone: see `make hooks`.
