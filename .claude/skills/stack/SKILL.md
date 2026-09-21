---
name: stack
description: Create and maintain stacked pull requests (child PRs that target a parent PR's branch). Use when a change depends on an unmerged PR, or when asked to restack or retarget.
---

A stack is a chain of branches where each PR targets the branch below it instead of `main`.
`scripts/stack` walks that chain from the PRs themselves; there is no extra metadata.

- **Create a child:** branch from the parent branch, commit, then
  `scripts/create-pr --base <parent-branch> --summary ... --why ... --testing ...`.
  Say in `--notes` that it is stacked on `#<parent>`.
- **See the chain:** `scripts/stack show` (default branch at the top, current branch at the bottom).
- **Parent changed (new commits, force-push):** `scripts/stack restack --push` rebases every
  branch in the chain onto its updated parent and force-pushes with lease. Run without
  `--push` first to see whether anything conflicts.
- **Parent merged (squash):** `scripts/stack retarget` on the child. It drops the parent's
  now-duplicated commits, rebases onto the parent's base, pushes, and edits the PR base.
  Do this before deleting the parent branch; deleting a child's base first closes the child.
- Conflicts stop the script with the exact commands to finish by hand; never reset a branch
  to escape a rebase.
- Merge order is bottom-up: never merge a child before its parent.
- Merging: `gh pr merge <n> --squash --auto` and let GitHub wait for green checks; never merge
  behind a `gh pr checks --watch` whose exit code you did not gate on.
