#!/usr/bin/env bash
# Fails if regenerating changes the committed src/api/*.gen.ts; compares before and after so a dirty tree is fine.
set -euo pipefail
cd "$(dirname "$0")/.."
before=$(cat src/api/*.gen.ts | sha256sum)
pnpm generate >/dev/null
after=$(cat src/api/*.gen.ts | sha256sum)
if [[ "$before" != "$after" ]]; then
  echo "src/api/*.gen.ts are out of date; run pnpm generate and commit the output" >&2
  git --no-pager diff --stat -- src/api
  exit 1
fi
echo "generated API files are up to date"
