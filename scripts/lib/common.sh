#!/usr/bin/env bash
# Shared helpers for the scripts in this directory. Source, do not execute.

die() { printf 'error: %s\n' "$*" >&2; exit 1; }
info() { printf '%s\n' "$*" >&2; }

repo_root() { git rev-parse --show-toplevel; }
current_branch() { git rev-parse --abbrev-ref HEAD; }
default_branch() { gh repo view --json defaultBranchRef --jq .defaultBranchRef.name; }

require_clean_tree() {
  if [[ -n "$(git status --porcelain --untracked-files=no)" ]]; then
    git status --short --untracked-files=no >&2
    die "uncommitted changes; commit or stash them first"
  fi
}

# gh holds several accounts on the dev machine; PRs and comments must come from the repo owner.
require_gh_account() {
  local owner active
  owner=$(gh repo view --json owner --jq .owner.login)
  active=$(gh api user --jq .login)
  [[ "$active" == "$owner" ]] || die "gh is authenticated as $active, repo owner is $owner; run: gh auth switch --user $owner"
}

require_feature_branch() {
  local b; b=$(current_branch)
  [[ "$b" != "$(default_branch)" && "$b" != "HEAD" ]] || die "on $b; check out a feature branch or pass --pr <number>"
}

# Resolves a PR number from --pr or from the current branch's open PR.
resolve_pr() {
  local pr="$1"
  if [[ -n "$pr" ]]; then printf '%s' "$pr"; return; fi
  require_feature_branch
  pr=$(gh pr view --json number --jq .number 2>/dev/null) || die "no open PR for $(current_branch); pass --pr <number>"
  printf '%s' "$pr"
}

notes_dir() { local d; d="$(repo_root)/notes/pr-$1"; mkdir -p "$d"; printf '%s' "$d"; }
