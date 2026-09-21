# 0017: Communications: notices composed in the transition, letters ready at once, emails through an outbox

**Status:** accepted, 2026-09-22

## Context

The regimes oblige the bank to tell the customer things at particular moments: that the dispute was
received (Regulation Z wants a written acknowledgement within thirty days), that a provisional
credit was posted and may be reversed (12 CFR 1005.11(c)(2)(iv)), the date and amount of a reversal
before it happens (1005.11(d)(2)), and the outcome with the right to see the documents relied on
(1005.11(d)(1), 1026.13(e) and (f)). Two of the four regimes want these in writing. Sending is a
side effect that can fail, and a transition must not be undone because a mail relay was down.

## Decision

- What is owed is domain knowledge: `NoticesFor(rules, enteredState)` names the kinds a transition
  triggers (acknowledgement at opening, the questionnaire, a credit or refund notice, a reversal
  notice, the resolution) and `ChannelsFor(rules, kind)` names the channels, email always and a
  letter as well where the regime's notices must be written and the notice carries a legal
  consequence. Gated on state and one rule property, never on a regime's name.
- A notice is composed once, in the transition's transaction, from facts the engine already holds
  (the account, the transaction, the clocks, the ledger, the questionnaire), as structured
  paragraphs with the provision it satisfies, and stored as a row. Letters are complete the moment
  they exist: the workbench renders them on a printable page from the paragraphs, never from
  markup. Emails are an outbox: a dispatcher claims unsent rows across tenants through owner-defined
  functions (the same pattern as the purge), renders text and HTML from the same paragraphs, hands
  them to the relay, and records sent or the failure; a nudge after each commit keeps latency low
  and a timer keeps nothing lost; backoff grows per attempt and SKIP LOCKED lets replicas share.
- The acknowledgement clock (ADR 0013) is met by the acknowledgement notice, not by any state:
  what the regulation asks for is the acknowledgement.
- The relay is configuration (`DISPUTE_SMTP_ADDR`; Mailpit in development and CI); without one
  the mailer logs, so a deployment without mail still records what it would have sent.
  Recipients come from the account (`accounts.email`, `accounts.postal_address`); a missing
  address skips that channel and the other still goes out.

## Consequences

- Easier: the customer's file is complete and auditable from the dispute alone; a new obligation
  is a kind, a channel rule and a paragraph, and it can never be forgotten by a code path because
  the transition writes it; the browser suite proves the mail arrives.
- Harder: the letter is a page, not a PDF, and printing is the tenant's business; templates are one
  language and one house style; the dispatcher is at-least-once, so a relay that accepted but
  failed to acknowledge can produce a duplicate email (the letter, being a row, cannot duplicate);
  reminders for unanswered questionnaires are the natural next step and are not built.
