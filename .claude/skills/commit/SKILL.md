---
name: commit
description: Write and make a commit in this repo. Use when asked to commit, stage changes, or write a commit message.
---

Run `validate` first when Go code changed.

## Message

- Subject: `type(scope): summary`, imperative, lower case, no period, under 72 characters.
  Types: feat, fix, chore, refactor, docs, test, perf, build, ci, style. The scope is the
  area (`dispute`, `httpserver`, `ci`, `deploy`), never the type.
- Body only when the subject cannot carry the why: two or three sentences on motivation and
  impact (what changes for callers, what could break). Never restate the diff.
- No trailers of any kind. No em dashes, no emojis.

## Mechanics

- Stage only the files that belong to this change; check `git status` for strays such as
  `notes/` output or formatter side effects.
- The pre-commit hook formats and lints staged Go packages; if it rewrote files, they are
  re-staged automatically. If it fails, fix and commit again; never `--no-verify`.
- One logical change per commit so the branch bisects.
