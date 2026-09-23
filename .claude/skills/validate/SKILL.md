---
name: validate
description: Check that a Go change compiles, passes tests and lint. Use after editing backend code and before commit or PR.
---

Scope checks to the packages that changed; escalate to the whole module only before a PR.

## While editing

From `backend/`:

- Compile a package and its tests without running them: `go test -run=NOOP ./internal/<pkg>/...`
- Run its tests: `go test ./internal/<pkg>/...` (no `-v`; add `-run <Name>` to focus)

## Before commit or PR

From the repo root: `make ci` (versions, fmt, vet, lint, test, tidy). Fix formatting with
`make api:fmt-fix`. `make api:test-race` only works where cgo is available (CI does this).

## Interpreting failures

- `golangci-lint` findings: fix the code, not the config. Config changes need a one-line
  reason in `backend/.golangci.yml`.
- `go mod tidy` diff: run it and commit the result.
- `check-runtime-versions`: the message names the file that disagrees with `mise.toml`.
