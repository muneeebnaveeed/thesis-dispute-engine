# 0012: Client failure taxonomy: validate against the contract first, classify every other outcome

**Status:** accepted, 2026-09-21; extends ADR 0004 (RFC 9457 problems with a code registry) to the browser

## Context

The API answers every failure with a problem document carrying a stable `code`, `retryable` and,
for validation, per-field `errors`. The frontend was handing those documents to one generic banner
and letting field mistakes travel to the server before anyone saw them. Some outcomes are not
problems at all: the network is down, a proxy answers with an empty body, a server function throws.
The person at the screen needs the same treatment in each case: what went wrong, whether trying
again helps, and which field to fix.

## Decision

- The frontend has one closed set of failure kinds: `validation`, `conflict`, `not-found`,
  `signed-out`, `forbidden`, `rate-limited`, `unavailable`, `internal`, `unreachable`,
  `unexpected`. `classify` maps a problem's registry code to a kind, a thrown fetch error to
  `unreachable`, and anything else to `unexpected`; an unknown code lands on `internal`, so a new
  server code degrades to a generic message instead of a blank.
- Forms validate with the contract's own schemas (the TypeBox schemas generated from the OpenAPI
  document) before any request. Structural mistakes never leave the browser; a server-side field
  error, should one still arrive, is rendered in exactly the same place under the field. Semantic
  checks (the transaction exists, the transition is allowed) remain the server's job.
- Every direct browser call goes through `call`, which returns data or a failure, never throws,
  and repeats an idempotent call once when the failure is retryable and the advertised wait is
  short. Mutations carry an Idempotency-Key, so the repeat cannot double-apply.
- Failures render through one component: a title and a hint per kind, a countdown for rate
  limits, a retry for anything retryable, the request reference for anything server-side. A route
  that throws falls back to an error boundary that never shows a stack.

## Consequences

- Easier: one vocabulary for the whole client, tested once; the schemas are already generated, so
  validation follows the contract with no hand-written rules; new server codes need one line in
  `fromProblem`.
- Harder: the taxonomy is a second copy of the server's registry and can drift (the contract test
  on the server side and the unit tests on the client side are the guard); a bodiless 2xx must be
  recognised as success rather than as an empty answer.
