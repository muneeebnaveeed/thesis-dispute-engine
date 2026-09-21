#!/usr/bin/env bash
# Fails when generated or formatted output is not committed; CI runs it after fmt/tidy steps.
set -euo pipefail
changed=$(git diff --stat)
untracked=$(git ls-files --exclude-standard --others .)
if [[ -n "$changed$untracked" ]]; then
  echo "error: uncommitted changes after generation/formatting:" >&2
  [[ -n "$untracked" ]] && printf '  untracked:\n%s\n' "$(sed 's/^/    /' <<<"$untracked")" >&2
  [[ -n "$changed" ]] && printf '  changed:\n%s\n' "$(sed 's/^/   /' <<<"$changed")" >&2
  exit 1
fi
echo "ok: tree is clean"
