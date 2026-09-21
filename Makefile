# Root task runner; backend targets delegate into backend/.
.DEFAULT_GOAL := help
SHELL := /bin/bash

BACKEND := backend
GO      := cd $(BACKEND) && go

.PHONY: help hooks run dev build test test-race test-integration lint vet fmt fmt-fix tidy vuln versions generate generate-check db-up db-down db-logs db-seed up down docker-dev otel-up otel-down otel-reset image pr ci-logs pr-comments stack ci

help: ## List targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

hooks: ## Per-clone git setup: hooks path, and a credential helper that pushes as the repo owner whichever gh account is active
	git config core.hooksPath .githooks
	git config --replace-all credential.helper ""
	git config --add credential.helper '!f() { echo "username=muneeebnaveeed"; echo "password=$$(gh auth token --hostname github.com --user muneeebnaveeed)"; }; f'

run: ## Run the API natively (reads DISPUTE_* / OTEL_* env vars; defaults target deploy/compose.yml)
	$(GO) run ./cmd/api

dev: ## Run the API natively with live reload (air, pinned in mise.toml; config in backend/.air.toml)
	cd $(BACKEND) && air

build: ## Build the API binary into backend/bin/
	$(GO) build -o bin/api ./cmd/api

test: ## Unit tests
	$(GO) test ./...

test-race: ## Unit tests with the race detector (what CI runs; needs a C compiler for cgo)
	$(GO) test -race -count=1 ./...

TEST_DB_URL ?= postgres://dispute:dispute@localhost:5432/dispute?sslmode=disable
test-integration: ## Tests that need PostgreSQL (make db-up first); each test gets its own schema
	cd $(BACKEND) && DISPUTE_TEST_DATABASE_URL="$(TEST_DB_URL)" go test -count=1 ./...

generate: ## Regenerate sqlc queries and the OpenAPI server from docs/api/openapi.yaml
	cd $(BACKEND) && sqlc generate && go generate ./...

GENERATED := $(BACKEND)/internal/dispute/infrastructure/postgres/sqlcgen $(BACKEND)/internal/dispute/ports/http/oapi
generate-check: generate ## Fail if generated code is out of date
	git diff --exit-code -- $(GENERATED)

lint: ## golangci-lint (config in backend/.golangci.yml)
	cd $(BACKEND) && golangci-lint run ./...

versions: ## Check go.mod, Dockerfile and CI agree with mise.toml
	scripts/check-runtime-versions.sh

vet: ## go vet
	$(GO) vet ./...

fmt: ## Format check via golangci-lint (gofmt + goimports); fails if anything would change
	cd $(BACKEND) && golangci-lint fmt --diff

fmt-fix: ## Apply formatting
	cd $(BACKEND) && golangci-lint fmt

tidy: ## go mod tidy, then fail if it changed anything
	$(GO) mod tidy && git diff --exit-code -- $(BACKEND)/go.mod $(BACKEND)/go.sum

vuln: ## govulncheck (pinned as a go tool)
	cd $(BACKEND) && go tool govulncheck ./...

db-up: ## Start PostgreSQL via compose (host networking; see deploy/compose.yml)
	docker compose -f deploy/compose.yml up -d

db-down: ## Stop PostgreSQL and drop its volume
	docker compose -f deploy/compose.yml down -v

db-logs: ## Tail PostgreSQL logs
	docker compose -f deploy/compose.yml logs -f postgres

db-seed: ## Insert the fixed dev accounts and transactions (idempotent)
	$(GO) run ./cmd/seed

up: ## PostgreSQL + the API built from backend/Dockerfile
	docker compose -f deploy/compose.yml --profile app up -d --build

docker-dev: ## PostgreSQL + the API in a Go toolchain container with live reload (bind mount)
	docker compose -f deploy/compose.yml --profile dev up --build

down: ## Stop everything started by up/otel-up (keeps the DB volume)
	docker compose -f deploy/compose.yml --profile app --profile otel down

otel-up: ## PostgreSQL + API + Grafana LGTM (http://localhost:3001, admin/admin); API exports traces, metrics and logs
	docker compose -f deploy/compose.yml --env-file deploy/otel.env --profile otel up -d --build

otel-down: ## Stop the otel stack
	docker compose -f deploy/compose.yml --profile otel down

otel-reset: ## Stop the otel stack and drop its data volume
	docker compose -f deploy/compose.yml --profile otel down -v

# --network host for the build stage: on the primary dev machine the VPN drops
# traffic on Docker's bridge, so `go mod download` inside the build would time
# out. Runtime containers get the same via compose. CI does not need it.
image: ## Build the API image locally
	docker build --network host -t dispute-engine-api:local --build-arg VERSION=$$(git rev-parse --short HEAD) $(BACKEND)

pr: ## Open a PR: make pr ARGS='--summary ... --why ... --testing ...'
	scripts/create-pr $(ARGS)

ci-logs: ## Download failing CI logs for the current PR into notes/ (ARGS='--pr N --wait')
	scripts/fetch-ci-logs $(ARGS)

stack: ## Stacked PRs: make stack ARGS='show|restack --push|retarget'
	scripts/stack $(ARGS)

pr-comments: ## Export review comments for the current PR into notes/ (ARGS='--pr N')
	scripts/fetch-pr-comments $(ARGS)

ci: versions generate-check fmt vet lint test tidy ## Everything CI runs, locally (race detector and integration tests need extras; see test-race, test-integration)
