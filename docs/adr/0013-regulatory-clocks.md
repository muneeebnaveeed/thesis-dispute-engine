# 0013: Regulatory clocks: per-regime deadlines as rows, due in the tenant's calendar, settled by transitions

**Status:** accepted, 2026-09-22; refines the SLA columns of ADR 0002

## Context

Every regime imposes deadlines, and the deadlines are the part of a dispute a regulator audits:
PSD2 wants the customer refunded by the end of the following business day (art. 73(1)) and a
final reply to a complaint within 15 business days (art. 101(2)); Regulation E wants provisional
credit within 10 business days and the investigation done in 45 days (12 CFR 1005.11(c));
Regulation Z wants a written acknowledgement within 30 days and a resolution within two billing
cycles (12 CFR 1026.13(c)). Some count business days, some calendar days; business days depend on
the jurisdiction's holidays; ADR 0002 carried them as two approximate durations per regime and no
code read them. Analysts need to see the clocks, the list needs to surface what is about to run
out, and the thesis needs to show compliance as a measured property.

## Decision

- A regime's `Rules` carry a list of `Clock`s: kind (`REFUND`, `ACKNOWLEDGE`, `RESOLUTION`),
  a count, a unit (business or calendar days) and the citation. Kinds are gates the state machine
  already expresses: a refund clock is met by entering any credit state, the acknowledgement clock
  by opening the investigation, the resolution clock by closing; a refund clock is voided when the
  dispute closes without a credit. An appeal restarts the resolution clock, numbered by the appeal.
- Due times are computed once, at opening, as the last instant of the Nth permitted day in the
  tenant's calendar (`tenants.settings.timezone` and `.holidays`, weekends implied; UTC and
  weekends only when unset) and stored as rows in `dispute_deadlines` under row-level security.
  A clock is a fact that was communicated, so a later change to a calendar or a rule does not move
  it; a dispute that predates the feature has none rather than an invented breach.
- Status is derived at read time from `due_at`, `met_at` and `voided_at`: RUNNING, MET, LATE,
  BREACHED, VOID. No job marks breaches; the list's `overdue` filter and the
  `dispute.deadlines_overdue` gauge ask the same question of the same rows, and
  `dispute.deadline_slack` records how early or late each clock was satisfied.
- The API returns every clock on the dispute and the earliest open one on each list row; the
  workbench shows both with the citation, so the legal basis travels with the number.

## Consequences

- Easier: compliance is a query, not a report; a new regime or a changed statutory period is a
  row in the rules table with its citation; holidays are tenant data, entered at onboarding
  (`scripts/tenant onboard --timezone --holidays`), never code.
- Harder: an idempotent replay returns the response as first stored, so its clock statuses are as
  of the original request; extensions the law allows (PSD2's 35 business days for a complex
  complaint, Regulation E's 90 days for new accounts) are not modelled and would need a second
  clock or an explicit extension event; the acknowledgement clock equates acknowledgement with
  opening the investigation until communications (letters) exist to satisfy it properly.
