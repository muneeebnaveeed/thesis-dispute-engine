# 0018: Fraud scoring: fixed explainable rules, reassessed as facts arrive, a hold rather than a verdict

**Status:** accepted, 2026-09-22

## Context

The proposal listed a scoring engine with weighted signals (dispute frequency, transaction age,
questionnaire inconsistency, merchant category, chargeback history, account age, amount) and
three tiers. The supervisor named it as the first candidate to cut. The question was what form
would earn its place: a model needs data and evaluation the thesis does not have, and a score
nobody can argue with is a liability in a regulated process where the customer has the right to
know why they were refused.

## Decision

- Rules, not a model. Each signal contributes a fixed number of points from a fixed weight and
  reports a sentence saying why, whether or not it fired (`domain/risk.go`). The score is the
  sum; LOW below 20, MEDIUM from 20, HIGH from 45. Every assessment is stored with its signals,
  keyed to the event that prompted it.
- Assessed when the dispute opens, from what the engine already knows about the account (other
  disputes in twelve months, earlier disputes lost at the network), the transaction (age,
  amount, merchant category code) and the account (age); assessed again when the questionnaire
  arrives, which is when the consistency signal becomes known. Earlier assessments stay as
  history, so the analyst sees the picture change.
- The only enforcement is a hold: a credit on a dispute whose latest tier is HIGH is refused (422
  `risk-hold`) unless the event carries the analyst's justification, which is stored on the event.
  MEDIUM is a flag; LOW is nothing. The score never denies a dispute and never moves a state.
- The list shows the tier so a queue can be worked by risk; the dispute page shows every signal.

## Consequences

- Easier: an analyst, an auditor and the thesis can all read why a dispute scored what it did; a
  new signal is a case in one function and a test; the override leaves a human decision on the
  record instead of a silent bypass.
- Harder: the weights are judgement, not fit to data, and the thesis must say so; the merchant
  category list is fixed; a score that reads the account's history reflects the tenant's own
  disputes only, never a network-wide view; the hold is per dispute, so a HIGH tier does not
  stop a separate dispute on the same account from being credited by another analyst.
