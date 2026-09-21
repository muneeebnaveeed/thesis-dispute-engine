#!/usr/bin/env bash
# The Go toolchain is declared once, in mise.toml. Everything else must agree:
#   - backend/go.mod's `go` directive is the same major.minor
#   - backend/Dockerfile's default GO_VERSION is the exact same version
# CI reads both versions from mise.toml at run time, so it cannot drift.
# Exits non-zero naming each drift so the fix is obvious from the log.
set -euo pipefail
cd "$(dirname "$0")/.."

mise_go=$(sed -n 's/^go = "\(.*\)"$/\1/p' mise.toml)
mise_lint=$(sed -n 's/^golangci-lint = "\(.*\)"$/\1/p' mise.toml)
gomod_go=$(sed -n 's/^go \(.*\)$/\1/p' backend/go.mod)
docker_go=$(sed -n 's/^ARG GO_VERSION=\(.*\)$/\1/p' backend/Dockerfile)

problems=()
[[ -n "$mise_go" ]] || problems+=("mise.toml has no go version")
[[ "$mise_go" == "$gomod_go"* ]] || problems+=("backend/go.mod says go $gomod_go, mise.toml pins $mise_go")
[[ "$docker_go" == "$mise_go" ]] || problems+=("backend/Dockerfile ARG GO_VERSION=$docker_go, mise.toml pins $mise_go")
[[ -n "$mise_lint" ]] || problems+=("mise.toml has no golangci-lint version")

if ((${#problems[@]})); then
  echo "runtime-versions: drift found:" >&2
  printf '  %s\n' "${problems[@]}" >&2
  exit 1
fi
echo "runtime-versions: go $mise_go agrees across mise.toml, go.mod and Dockerfile; golangci-lint $mise_lint pinned"
