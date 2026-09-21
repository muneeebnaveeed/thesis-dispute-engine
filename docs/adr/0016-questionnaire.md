# 0016: The questionnaire: a reason selects the questions, the questions are snapshotted, answers are typed facts

**Status:** accepted, 2026-09-22

## Context

The state machine has had `SEND_QUESTIONNAIRE` and `RECEIVE_QUESTIONNAIRE` since ADR 0002 but nothing
behind them: no questions, no answers, no reason to ask one set rather than another. The proposal
wants structured claim information per dispute type feeding the fraud scoring, reminders for
incomplete questionnaires, and submission moving the state machine. A dispute also had no reason
at all, which made every dispute an "unauthorised transaction" by omission.

## Decision

- A dispute carries a `reason` from opening: `UNAUTHORISED` (the default, and what the tenant
  systems already send), `NOT_RECEIVED`, `DUPLICATE`, `AMOUNT_DIFFERS`. The reason is immutable
  and selects the question set.
- Question sets are code in the domain (`domain/questionnaire.go`): each question has an id, the
  text, a type (`YES_NO`, `DATE`, `TEXT`, `AMOUNT`) and whether it is required. Sending snapshots
  the set onto the dispute (`questionnaires.questions`), so answers always match what was asked
  even after the set changes; sending again replaces the snapshot and clears the answers.
- Answers travel in the `RECEIVE_QUESTIONNAIRE` payload as `answers`, a map from question id to a
  string, and are validated against the snapshot: required ones present, each well-typed, none
  for questions not asked. A refusal is a 422 `invalid-answers` with one field error per question,
  and the transition is rolled back. Accepted answers are stored on the questionnaire and, like
  every payload, verbatim on the event.
- The domain also names contradictions between answers (`Inconsistencies`), returned with the
  questionnaire and later a fraud signal (ADR 0018). Reminders for unanswered questionnaires belong
  to communications (ADR 0017) and are not built here.

## Consequences

- Easier: a new dispute type is a reason and a question list; the workbench renders any set from
  its types; the fraud scoring reads typed answers rather than free text; an auditor sees exactly
  the questions the customer was shown.
- Harder: question text is code and not per-tenant or per-language (a tenant wanting its own
  wording needs a settings-driven set, not built); the event payload remains open, so `answers`
  on any other event is stored but ignored; `RECEIVE_QUESTIONNAIRE` from the actions list would
  send an empty answer set, so the workbench routes it through the questionnaire form only.
