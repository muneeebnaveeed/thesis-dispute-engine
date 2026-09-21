---
name: review-comments
description: Fetch PR review comments and build a plan to address them. Use when asked to address, fix or respond to review feedback.
---

1. `scripts/fetch-pr-comments [--pr N]` writes `notes/pr-<n>/pr-comments.md` (threads marked
   OPEN or resolved, with file and line) and `pr-comments.json`.
2. Read the Markdown. For each OPEN thread decide: change the code, answer with a reason, or
   ask the user. Outdated threads may already be addressed; check the current code before
   acting.
3. Write the plan to `notes/pr-<n>/review-plan.md`: one line per thread with the decision and
   the file it touches. Show it to the user and wait for a go.
4. Make the changes in small commits (`commit` skill), one review theme per commit where
   possible, so the reviewer can match commits to threads.
5. For threads that need a reply rather than a change, draft the reply text for the user to
   post; do not post on their behalf unless asked.
6. Push, then report which threads were addressed by which commit and which still need a
   human answer.
