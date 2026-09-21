# 0002: One dispute state machine, parameterised by a regulatory regime

**Status:** accepted, 2026-09-08 (recorded 2026-09-21)

## Context

The engine must model dispute lifecycles under regimes with different rules: EU PSD2
card disputes (refund by end of next business day, €50 liability cap, merchant may
contest via the card network), SEPA Direct Debit (eight-week no-questions-asked refund,
no adjudication, not contestable), and, as a comparison, US Regulation E
(provisional credit within 10 business days, 45-day investigation) and Regulation Z
(collection suspended, no credit issued). The system is issuer-side: it receives the
cardholder's complaint and files outward through the network.

Two designs were considered: a state machine per regime, or one machine whose
reachable states, deadlines, and terminal shapes are configuration.

## Decision

One state machine. The lifecycle shape (initiate → investigate/evidence → resolve →
settle, with appeal re-entering at investigation under a capped retry count) is shared.
What differs per regime is *which states are reachable, which deadlines apply, and
whether evidence/adjudication happens at all*; that is a `DisputeRegime` configuration
value carried by every dispute from creation, not a separate machine.

EU regimes are first-class and the implementation focus. US regimes exist as
configuration and test fixtures to demonstrate that the model generalises and to
support the comparative-regulation discussion in the thesis.

Regime-gated states (a regime enables a subset):

```
INITIATED → INVESTIGATING → [QUESTIONNAIRE_SENT → QUESTIONNAIRE_RECEIVED]
  → [PROVISIONAL_CREDIT_ISSUED | FAST_REFUND_ISSUED | SEPA_NQA_REFUND_ISSUED]
  → [CHARGEBACK_FILED → CHARGEBACK_ACKNOWLEDGED → EVIDENCE_SUBMITTED
     → CHARGEBACK_WON | CHARGEBACK_LOST]
  → [FINAL_CREDIT_ISSUED | PROVISIONAL_CREDIT_REVERSED] → CLOSED
APPEAL re-enters at INVESTIGATING, capped.
```

| Regime | Provisional credit | Resolution SLA | Evidence/adjudication | Contestable | Liability cap |
|---|---|---|---|---|---|
| `EU_SEPA_DIRECT_DEBIT` | no (direct NQA refund) | 8 weeks (13 months with evidence) | no | no | n/a |
| `EU_PSD2_CARD` | no (fast refund) | end of next business day | yes | yes | €50 |
| `US_REG_E` | yes | 10 business days; 45 days total | yes | yes | $50 |
| `US_REG_Z` | no (collection suspended) | 30 days ack, 90 days resolve | yes | yes | n/a |

## Consequences

- Easier: one transition table to test; adding a regime is a row plus fixtures; the
  provisional-credit, chargeback, and banking-core components take the regime as an
  input from day one instead of being retrofitted.
- Harder: the transition table must express gating clearly or it becomes a thicket of
  conditionals; keep the gate a property of the regime (`HasProvisionalCredit`,
  `HasAdjudication`), never an `if regime == …` in a handler.
- The regime is derived from the transaction's payment rail (SEPA DD vs card) and the
  dataset the demo targets; it is immutable for the life of a dispute.
