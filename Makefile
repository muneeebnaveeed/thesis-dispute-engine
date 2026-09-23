# Root task runner. Targets are namespaced by domain: db:, api:, workbench:, stack:, auth:, otel:,
# thesis: and pr:. The colon is escaped in this file and typed plainly on the command line
# (`make db:up`). Backend targets delegate into backend/, frontend targets into frontend/.
.DEFAULT_GOAL := help
SHELL := /bin/bash

BACKEND  := backend
FRONTEND := frontend
GO       := cd $(BACKEND) && go
PNPM     := pnpm -C $(FRONTEND)
COMPOSE  := docker compose -f deploy/compose.yml

TEST_DB_URL     ?= postgres://dispute:dispute@localhost:5432/dispute?sslmode=disable
TEST_APP_DB_URL ?= postgres://dispute_api:dispute_api@localhost:5432/dispute?sslmode=disable
GENERATED       := $(BACKEND)/internal/dispute/infrastructure/postgres/sqlcgen $(BACKEND)/internal/dispute/ports/http/oapi deploy/keycloak/import

.PHONY: help demo demo\:check demo\:down ci versions hooks \
	db\:up db\:down db\:logs db\:seed db\:migrate \
	api\:run api\:dev api\:build api\:test api\:test-race api\:test-integration api\:cover \
	api\:generate api\:generate-check api\:lint api\:vet api\:fmt api\:fmt-fix api\:tidy api\:vuln api\:image \
	workbench\:install workbench\:dev workbench\:check workbench\:fix workbench\:generate workbench\:cover \
	workbench\:build workbench\:image workbench\:e2e \
	stack\:up stack\:dev stack\:down auth\:up auth\:down otel\:up otel\:down otel\:reset \
	thesis\:build thesis\:eval pr\:open pr\:ci-logs pr\:comments pr\:stack

##@ Everyday

help: ## Every target, grouped by domain
	@awk '/^##@ / { printf "\n  \033[1m%s\033[0m\n", substr($$0, 5); next } \
		/^[a-zA-Z0-9_\\:-]+:.*## / { \
			line = $$0; gsub(/\\/, "", line); match(line, /^[a-zA-Z0-9_:-]+:/); \
			printf "    \033[36m%-22s\033[0m %s\n", substr(line, 1, RLENGTH - 1), substr(line, index(line, "## ") + 3); \
		}' $(MAKEFILE_LIST)

demo: ## The whole system for a demonstration, verified: Keycloak, API, workbench, Mailpit, Grafana
	scripts/demo

demo\:check: ## Verify a demonstration stack that is already up, change nothing
	scripts/demo --check

demo\:down: ## Stop everything the demonstration started, keep the data
	scripts/demo --down

ci: versions api\:generate-check api\:fmt api\:vet api\:lint api\:test api\:tidy workbench\:check ## Everything CI runs, locally (the race detector and integration tests need extras; see api:test-race, api:test-integration)

versions: ## Check go.mod, Dockerfile and CI agree with mise.toml
	scripts/check-runtime-versions.sh

hooks: ## Per-clone git setup: hooks path, and a credential helper that pushes as the repo owner whichever gh account is active
	git config core.hooksPath .githooks
	git config --replace-all credential.helper ""
	git config --add credential.helper '!f() { echo "username=muneeebnaveeed"; echo "password=$$(gh auth token --hostname github.com --user muneeebnaveeed)"; }; f'

##@ Database

db\:up: ## Start PostgreSQL via compose (host networking; see deploy/compose.yml) and create the API login role
	$(COMPOSE) up -d --wait postgres
	$(COMPOSE) exec -T postgres psql -U dispute -d dispute -v ON_ERROR_STOP=1 -q < deploy/postgres/roles.sql

db\:down: ## Stop PostgreSQL and drop its volume
	$(COMPOSE) down -v

db\:logs: ## Tail PostgreSQL logs
	$(COMPOSE) logs -f postgres

db\:migrate: ## Apply pending migrations as the schema owner (the API only checks, never migrates)
	$(GO) run ./cmd/migrate

db\:seed: ## Insert the fixed dev accounts and transactions (idempotent)
	$(GO) run ./cmd/seed

##@ API (Go)

api\:run: db\:migrate ## Run the API natively (reads DISPUTE_* / OTEL_* env vars; defaults target deploy/compose.yml)
	$(GO) run ./cmd/api

api\:dev: ## Run the API natively with live reload (air, pinned in mise.toml; config in backend/.air.toml)
	cd $(BACKEND) && air

api\:build: ## Build the api, migrate and seed binaries into backend/bin/
	$(GO) build -o bin/ ./cmd/...

api\:test: ## Unit tests: no database, no network (in-memory store, real handlers)
	$(GO) test ./...

api\:test-race: ## Unit tests with the race detector (needs a C compiler for cgo)
	$(GO) test -race -count=1 ./...

api\:test-integration: ## Postgres-backed tests behind the integration build tag (make db:up first); each test gets its own schema
	cd $(BACKEND) && DISPUTE_TEST_DATABASE_URL="$(TEST_DB_URL)" DISPUTE_TEST_APP_DATABASE_URL="$(TEST_APP_DB_URL)" go test -tags integration -count=1 ./...

api\:cover: ## Coverage table as CI reports it (uses PostgreSQL if make db:up is running); coverage.html for line detail
	cd $(BACKEND) && DISPUTE_TEST_DATABASE_URL="$(TEST_DB_URL)" DISPUTE_TEST_APP_DATABASE_URL="$(TEST_APP_DB_URL)" go test -tags integration -count=1 -coverprofile=coverage.out -covermode=atomic ./... > /dev/null
	cd $(BACKEND) && go tool cover -html=coverage.out -o coverage.html
	scripts/coverage-report $(BACKEND)/coverage.out

api\:generate: ## Regenerate sqlc queries, the OpenAPI server, and the Keycloak realm imports
	cd $(BACKEND) && sqlc generate && go generate ./...
	deploy/keycloak/render-realms.sh >/dev/null

api\:generate-check: ## Fail if generated code is out of date (compares before and after, so a dirty tree is fine)
	@before=$$(find $(GENERATED) -type f | sort | xargs sha256sum | sha256sum); \
	$(MAKE) --no-print-directory 'api:generate'; \
	after=$$(find $(GENERATED) -type f | sort | xargs sha256sum | sha256sum); \
	if [ "$$before" != "$$after" ]; then echo "generated code is out of date, commit the output of make api:generate"; git --no-pager diff --stat -- $(GENERATED); exit 1; fi

api\:lint: ## golangci-lint (config in backend/.golangci.yml)
	cd $(BACKEND) && golangci-lint run ./...

api\:vet: ## go vet, including the integration-tagged tests
	$(GO) vet -tags integration ./...

api\:fmt: ## Format check via golangci-lint (gofmt + goimports); fails if anything would change
	cd $(BACKEND) && golangci-lint fmt --diff

api\:fmt-fix: ## Apply formatting
	cd $(BACKEND) && golangci-lint fmt

api\:tidy: ## go mod tidy, then fail if it changed anything
	$(GO) mod tidy && git diff --exit-code -- $(BACKEND)/go.mod $(BACKEND)/go.sum

api\:vuln: ## govulncheck (pinned as a go tool)
	cd $(BACKEND) && go tool govulncheck ./...

api\:image: ## Build the API image locally
	docker build --network host -t dispute-engine-api:local --build-arg VERSION=$$(git rev-parse --short HEAD) $(BACKEND)

##@ Workbench (frontend)

workbench\:install: ## Install frontend dependencies (pnpm, pinned in mise.toml)
	$(PNPM) install --frozen-lockfile

workbench\:dev: ## Frontend dev server on http://localhost:3002 (expects the API on :8090)
	$(PNPM) dev

workbench\:check: ## Frontend typecheck, lint, dead code, format check and tests (what CI runs)
	$(PNPM) generate-routes && $(PNPM) generate:check && $(PNPM) typecheck && $(PNPM) lint && $(PNPM) knip && $(PNPM) fmt && $(PNPM) test

workbench\:fix: ## Apply frontend lint and format fixes
	$(PNPM) lint:fix && $(PNPM) fmt:fix

workbench\:generate: ## Regenerate the frontend API types and schemas from docs/api/openapi.yaml; commit the output
	$(PNPM) generate

workbench\:cover: ## Coverage for the layers that hold logic (src/server, src/lib); reported, never gated
	$(PNPM) test:cover

workbench\:build: ## Production build into frontend/.output
	$(PNPM) generate-routes && $(PNPM) build

# --network host for the build stage: on the primary dev machine the VPN drops
# traffic on Docker's bridge, so `go mod download` inside the build would time
# out. Runtime containers get the same via compose. CI does not need it.
workbench\:image: ## Build the frontend container image
	docker build --network host -t dispute-engine-frontend:dev --build-arg NODE_VERSION=$$(sed -n 's/^node = "\(.*\)"$$/\1/p' mise.toml) --build-arg PNPM_VERSION=$$(sed -n 's/^pnpm = "\(.*\)"$$/\1/p' mise.toml) $(FRONTEND)

workbench\:e2e: ## Browser suite against the full local stack (starts Postgres, API and Keycloak, seeds, builds the frontend)
	deploy/keycloak/render-realms.sh >/dev/null
	$(COMPOSE) --profile app --profile auth up -d --build --wait postgres migrate api keycloak
	$(MAKE) --no-print-directory 'db:up' 'db:seed'
	$(PNPM) e2e

##@ Stack (Docker Compose)

stack\:up: ## PostgreSQL + the API built from backend/Dockerfile
	$(COMPOSE) --profile app up -d --build

stack\:dev: ## PostgreSQL + the API in a Go toolchain container with live reload (bind mount)
	$(COMPOSE) --profile dev up --build

stack\:down: ## Stop everything started by stack:up, otel:up or auth:up (keeps the DB volume)
	$(COMPOSE) --profile app --profile otel --profile auth down

auth\:up: ## PostgreSQL + Keycloak (http://localhost:8180, admin/admin; one realm per tenant, analyst/analyst in each); seeds the DB
	deploy/keycloak/render-realms.sh
	$(COMPOSE) --profile auth up -d --wait
	$(MAKE) --no-print-directory 'db:seed'

auth\:down: ## Stop Keycloak (keeps its database)
	$(COMPOSE) --profile auth down

otel\:up: ## PostgreSQL + API + Grafana LGTM (http://localhost:3001, admin/admin) + synthetic probe; seeds the DB
	$(COMPOSE) --env-file deploy/otel.env --profile otel up -d --build
	$(MAKE) --no-print-directory 'db:seed'

otel\:down: ## Stop the otel stack
	$(COMPOSE) --profile otel down

otel\:reset: ## Stop the otel stack and drop its data volume
	$(COMPOSE) --profile otel down -v

##@ Thesis

thesis\:build: ## Build docs/thesis/latex/main.tex to docs/thesis/latex/build/main.pdf (TeX Live in Docker; Overleaf builds the same sources)
	docker run --rm --network host -v "$(CURDIR)/docs/thesis/latex:/work" -w /work texlive/texlive:latest latexmk -pdf -interaction=nonstopmode main.tex >/dev/null && echo "docs/thesis/latex/build/main.pdf"

thesis\:eval: ## Evaluation run into docs/thesis/data/<date>/ (needs make otel:up); EVAL_RPS, EVAL_SECONDS, EVAL_KEY
	scripts/eval

##@ Pull requests

pr\:open: ## Open a PR: make pr:open ARGS='--description "..."' (see .claude/skills/create-pr)
	scripts/create-pr $(ARGS)

pr\:ci-logs: ## Download failing CI logs for the current PR into notes/ (ARGS='--pr N --wait')
	scripts/fetch-ci-logs $(ARGS)

pr\:comments: ## Export review comments for the current PR into notes/ (ARGS='--pr N')
	scripts/fetch-pr-comments $(ARGS)

pr\:stack: ## Stacked PRs: make pr:stack ARGS='show|restack --push|retarget'
	scripts/stack $(ARGS)
