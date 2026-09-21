# 0015: Banking core behind a port: ISO 8583 semantics, per-tenant configuration, called inside the transition

**Status:** accepted, 2026-09-22; completes ADR 0014

## Context

The ledger records what the engine instructs; something has to move the money on the customer's
account. Every tenant has its own core banking system, reached differently, answering differently
and failing differently. The proposal asked for a simulated integration "modelled on ISO 8583
field semantics". The design question is not the simulator but the port: what the engine sends,
what it expects back, when it calls, and what happens when the answer is no, or does not come.

## Decision

- One port, `BankingCore.Post(config, instruction) -> receipt`. An instruction is one movement on
  the customer's account: the posting's reference, the account, the amount and currency, credit or
  debit, the dispute and original transaction. A receipt is the core's answer in ISO 8583 terms:
  a retrieval reference number (DE37), a response code (DE39), approved or not, and whether the
  core recognised the reference as one it had already carried out (94, duplicate). Only postings
  that touch the customer account cross the port; suspense, recovery and loss are the bank's own
  books.
- Each tenant names its core in `tenants.settings.core` (`kind` plus adapter settings); a router
  picks the adapter by kind and a tenant with none gets book entries only. The only adapter in this
  build is the simulator, which speaks in the same fields a real switch would (0200/0210 message
  types, processing code 20 for a credit and 01 for a debit, amount in minor units, STAN, RRN
  derived from the reference, ISO 4217 numeric currency), keeps what it has seen for duplicate
  detection, and is configured per tenant to decline credits above a limit (61), refuse debits on
  named accounts (51), answer slowly, or be down. The two seed banks are configured differently on
  purpose.
- The call happens inside the transition's database transaction, with a bounded timeout. A
  decline rolls the transition back and answers 422 `core-declined` with the response code; no
  event, no posting. Silence rolls it back and answers 503; the client repeats with the same
  Idempotency-Key, the engine issues the same posting reference (the event sequence is unchanged),
  and a core that did the work answers duplicate rather than paying twice. Approved receipts are
  stored on the ledger row.

## Consequences

- Easier: the transition and the money agree by construction, since neither is committed without
  the other; the retry story needs no outbox or reconciliation job because the reference is
  deterministic; a new core is an adapter and a `kind`, and a tenant's choice is data.
- Harder: the dispute row is locked for the length of a core round trip, so a slow core slows that
  dispute (not others); a core that answers approved after the engine gave up leaves money moved
  and the transition unwritten until the retry, which the duplicate answer resolves but which is
  visible in the interval; the simulator's memory of references does not survive a restart, so
  duplicate detection in the demo is per process (the ledger's unique reference still stops the
  engine from posting twice). Declines are not persisted; they are in the request log, the trace
  and the `dispute.core_messages` counter.
