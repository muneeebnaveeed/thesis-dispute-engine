#!/usr/bin/env bash
# Toolchains are declared once, in mise.toml. Everything else must agree:
#   - backend/go.mod's `go` directive is the same major.minor
#   - backend/Dockerfile's default GO_VERSION is the exact same version
#   - frontend/package.json's packageManager and frontend/Dockerfile's defaults match node and pnpm
# CI reads both versions from mise.toml at run time, so it cannot drift.
# Exits non-zero naming each drift so the fix is obvious from the log.
set -euo pipefail
cd "$(dirname "$0")/.."

mise_go=$(sed -n 's/^go = "\(.*\)"$/\1/p' mise.toml)
mise_lint=$(sed -n 's/^golangci-lint = "\(.*\)"$/\1/p' mise.toml)
gomod_go=$(sed -n 's/^go \(.*\)$/\1/p' backend/go.mod)
docker_go=$(sed -n 's/^ARG GO_VERSION=\(.*\)$/\1/p' backend/Dockerfile)
mise_node=$(sed -n 's/^node = "\(.*\)"$/\1/p' mise.toml)
mise_pnpm=$(sed -n 's/^pnpm = "\(.*\)"$/\1/p' mise.toml)
pkg_pnpm=$(sed -n 's/^ *"packageManager": "pnpm@\([^"]*\)".*$/\1/p' frontend/package.json)
docker_node=$(sed -n 's/^ARG NODE_VERSION=\(.*\)$/\1/p' frontend/Dockerfile)
docker_pnpm=$(sed -n 's/^ARG PNPM_VERSION=\(.*\)$/\1/p' frontend/Dockerfile)

problems=()
[[ -n "$mise_go" ]] || problems+=("mise.toml has no go version")
[[ "$mise_go" == "$gomod_go"* ]] || problems+=("backend/go.mod says go $gomod_go, mise.toml pins $mise_go")
[[ "$docker_go" == "$mise_go" ]] || problems+=("backend/Dockerfile ARG GO_VERSION=$docker_go, mise.toml pins $mise_go")
[[ -n "$mise_lint" ]] || problems+=("mise.toml has no golangci-lint version")
[[ -n "$mise_node" && -n "$mise_pnpm" ]] || problems+=("mise.toml has no node or pnpm version")
[[ "$pkg_pnpm" == "$mise_pnpm" ]] || problems+=("frontend/package.json packageManager pnpm@$pkg_pnpm, mise.toml pins $mise_pnpm")
[[ "$docker_node" == "$mise_node" ]] || problems+=("frontend/Dockerfile ARG NODE_VERSION=$docker_node, mise.toml pins $mise_node")
[[ "$docker_pnpm" == "$mise_pnpm" ]] || problems+=("frontend/Dockerfile ARG PNPM_VERSION=$docker_pnpm, mise.toml pins $mise_pnpm")

if ((${#problems[@]})); then
  echo "runtime-versions: drift found:" >&2
  printf '  %s\n' "${problems[@]}" >&2
  exit 1
fi
echo "runtime-versions: go $mise_go, node $mise_node, pnpm $mise_pnpm agree across mise.toml, go.mod, package.json and Dockerfiles; golangci-lint $mise_lint pinned"
