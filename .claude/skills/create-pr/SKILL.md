---
name: create-pr
description: Open a pull request for the current branch with scripts/create-pr. Use when asked to open, create or submit a PR.
---

Never call `gh pr create` directly; use the script so the template and title rules hold. A CI
check (`pr template`) rejects PRs that do not follow them however they were opened.

```sh
scripts/create-pr --description "..." [--type feat] [--tests yes|no] [--rollout "..."] [--adr "..."] \
                  [--title "type(scope): subject"] [--draft] [--dry-run]
```

- Run `make ci` first; the script refuses an unclean tree.
- Title: conventional-commit shaped, `type(scope): subject`. The component is the scope, never
  the type (`fix(ci): ...`, not `ci: ...`). With one commit it defaults to that subject; with
  several, pass `--title` describing the whole branch.
- `--description`: two sentences, three at most, stating the scope and impact of the change and
  what it aims to achieve. Never a summary of what changed. Do not name files, functions, flags,
  packages or tables; those are in the diff. The script rejects backticks, filenames, `name()`
  and `--flag` tokens, and more than three sentences. Write it for someone reading the squash
  commit on main a year from now.
- `--type` defaults to the title prefix. `--tests` defaults to whether the diff contains test
  files. `--adr` defaults to links to ADR files in the diff, else `N/A`. `--rollout` defaults to
  `direct deploy`; when the diff contains a migration the script insists you state it (for
  example `migration + backfill, run /migrate before rolling the API`).
- Use `--dry-run` to show the user the title and body before creating.
- The script pushes the branch, so do not push separately.
- The PR title and body become the squash commit on main verbatim (repo setting). Detail that
  a reviewer needs but the description should not carry (verification steps, numbers, tricky
  decisions) goes in a PR comment after creation, not in the body.
- Report the PR URL as a markdown link. Then `scripts/fetch-ci-logs --wait` if asked to watch CI.
