---
name: fix-ci
description: Triage a failing CI run for the current branch's PR (or --pr N), download the failing logs, and propose a fix. Use when asked to fix CI or a build failure.
---

Investigate first, then confirm the plan with the user before editing source.

1. Target: the current branch's open PR, or `--pr <number>` if given. On the default branch
   without `--pr`, stop and ask.
2. Fetch: `scripts/fetch-ci-logs [--pr N] [--wait]`. It writes to `notes/pr-<n>/`:
   `ci-triage-index.txt` (run, status, non-successful jobs), `run-<id>-jobs.json`,
   `run-<id>-logs.txt` (failed steps only).
3. Read the index, then the log. Each CI job is one check (`backend lint`, `backend test`,
   `backend image`, ...), so the job name already says which `make` target reproduces it.
4. Reproduce locally with that target (`make lint`, `make test`, `make image`, ...) before
   proposing anything; the race detector is the one check that cannot run here.
5. Present: failing job, root cause, the minimal fix, and what else it touches. Wait for a go.
6. After fixing: `make ci`, commit with the `commit` skill, push, `scripts/fetch-ci-logs --wait`.

`notes/` is gitignored; never commit it.
