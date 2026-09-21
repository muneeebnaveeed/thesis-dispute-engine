# Root task runner. Backend targets delegate into backend/; frontend targets
# arrive with Phase 4 (pnpm + Node in frontend/).
.DEFAULT_GOAL := help
SHELL := /bin/bash

BACKEND := backend
GO      := cd $(BACKEND) && go

.PHONY: help run dev build test test-race lint vet fmt fmt-fix tidy vuln versions db-up db-down db-logs up down otel-up otel-down image ci

help: ## List targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

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

up: ## PostgreSQL + the API built from backend/Dockerfile
	docker compose -f deploy/compose.yml --profile app up -d --build

down: ## Stop everything started by up/otel-up (keeps the DB volume)
	docker compose -f deploy/compose.yml --profile app --profile otel down

otel-up: ## PostgreSQL + API + Jaeger (UI at http://localhost:16686); API exports traces to it
	docker compose -f deploy/compose.yml --env-file deploy/otel.env --profile otel up -d --build

otel-down: ## Stop the otel stack
	docker compose -f deploy/compose.yml --profile otel down

# --network host for the build stage: on the primary dev machine the VPN drops
# traffic on Docker's bridge, so `go mod download` inside the build would time
# out. Runtime containers get the same via compose. CI does not need it.
image: ## Build the API image locally
	docker build --network host -t dispute-engine-api:local --build-arg VERSION=$$(git rev-parse --short HEAD) $(BACKEND)

ci: versions fmt vet lint test tidy ## Everything CI runs, locally (race detector needs cgo; see test-race)
