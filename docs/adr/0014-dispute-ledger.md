# 0014: A dispute ledger: double-entry postings written by transitions, suspense that must clear at close

**Status:** accepted, 2026-09-22; realises the provisional-credit and refund rows of ADR 0002

## Context

Three of the four regimes move the customer's money before the dispute is decided: a provisional
credit that may be taken back (Regulation E), a fast refund the bank absorbs unless a chargeback
recovers it (PSD2), a no-questions-asked refund the SEPA scheme recovers from the creditor (SEPA
Core). The wiki's original proposal had a "provisional credit engine" and a "final credit engine"
as separate components. The engine must know, for every dispute and in aggregate, what it has
advanced, what came back, and what was absorbed; and it must be impossible to close a dispute with
money unaccounted for. The banking core that actually moves funds is a later component (ADR 0015).

## Decision

- One ledger, four accounts, from the issuer's point of view: `CUSTOMER`, `SUSPENSE` (the
  dispute receivable: what the bank has advanced and not yet cleared), `RECOVERY` and `LOSS`.
  Every movement is a double-entry posting with a kind, and postings are derived from the state
  a transition enters, never from the regime by name: entering a credit state debits suspense and
  credits the customer; a reversal does the opposite; a won chargeback recovers; a lost chargeback
  writes off unless the regime has provisional credit, in which case the reversal that follows
  clears it; closing settles whatever is left, as recovered or written off, by the analyst's word
  or the regime's default (recovered for SEPA, written off otherwise).
- The invariant is that suspense is zero once a dispute is CLOSED. It is not enforced by a check
  at close; it holds by construction, and a test enumerates every path from INITIATED to CLOSED
  under every regime (43 at the time of writing) and asserts it, so a new state or regime that
  breaks it fails the build.
- The credit is the disputed amount less a liability the analyst may assign on the refund event,
  capped by the regime (50 EUR under PSD2 art. 74, 50 USD under Regulation E, none elsewhere) and
  never the whole claim. The liability and the close settlement travel in the event payload, are
  validated before anything is written, and a refusal is a 422 with the field named; the refusing
  request leaves no event and no posting.
- Postings are rows in `ledger_entries`: append-only at the database, under row-level security,
  keyed to the event that caused them, each with a reference unique per tenant that the banking
  core adapter will use as its idempotency key. The API returns the ledger and the per-account
  balances on every dispute; a gauge exposes suspense per tenant and regime and a counter the
  postings by kind.

## Consequences

- Easier: provisional credit, refund, reversal, final credit and recovery are one mechanism with
  one test; the banking core adapter has an exact list of instructions to carry out and a
  reference to make each one idempotent; an auditor can reconcile a dispute from its ledger alone.
- Harder: the "final credit" event moves no money (recovery already cleared suspense at the win),
  which reads oddly until one sees the ledger; multi-currency disputes are out of scope (the
  regime fixes the currency); the ledger records what the engine instructed, and until ADR 0015
  lands that is also taken to be what happened.
