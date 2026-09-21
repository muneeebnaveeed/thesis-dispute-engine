---
name: create-pr
description: Open a pull request for the current branch with scripts/create-pr. Use when asked to open, create or submit a PR.
---

Never call `gh pr create` directly; use the script so the template and title rules hold.

```sh
scripts/create-pr --summary "..." --why "..." --testing "..." [--notes "..."] \
                  [--title "type(scope): subject"] [--draft] [--dry-run]
```

- Run `make ci` first; the script refuses an unclean tree.
- Title: conventional-commit shaped. With one commit it defaults to that subject; with
  several, pass `--title` describing the whole branch.
- `--summary`: what changed, one or two sentences. `--why`: motivation and impact, not a diff
  summary. `--testing`: what was run and what was checked by hand. `--notes`: follow-ups,
  deferred decisions, ADR links.
- Use `--dry-run` to show the user the title and body before creating.
- The script pushes the branch, so do not push separately.
- The PR title and body become the squash commit on main verbatim (repo setting), so write
  them as the permanent record; the branch's own commits stay only in the PR.
- Report the PR URL. Then `scripts/fetch-ci-logs --wait` if asked to watch CI.
