# 0026: Every operation is decided by Cedar policies over one principal

**Status:** accepted, 2026-09-30; builds on ADR 0008, 0009 and 0010

## Context

Row-level security keeps tenants apart (ADR 0008), and nothing kept anyone apart inside a tenant. Any analyst or
tenant key could apply every dispute event, including refunds and final credits. The one role that was checked,
`tenant-admin`, was checked by hand in seven handlers. The actor recorded on each dispute event was whatever the
request body said, so the audit trail named whoever the caller claimed to be. Issuers expect the controls their own
operations use: teams that own work, seniority for larger amounts, and separation of duties, where the person who
investigated a dispute does not also pay it out.

## Decision

Every operation that changes a dispute or the tenant is decided by one policy engine, Cedar, over one principal
that stands for either kind of credential.

- **One principal.** Authentication puts a `principal.Principal` on every request: an analyst (the OIDC subject,
  their email for display, and whether the realm granted `tenant-admin`), or a tenant key (its id and prefix). The
  event actor is always this principal. `actor` is no longer part of either request body, and `dispute_events`
  gains `actor_id` for the stable identity policies compare.
- **Cedar, embedded.** Policies are Cedar, evaluated in process with `cedar-go`. Cedar denies by default, and any
  `forbid` overrides every `permit`. Its semantics are formally specified, and the reference implementation can
  validate a policy set against a schema, which CI uses because the Go validator is still experimental.
- **Rules in code, grants in data.** Guardrails are Cedar in the binary, which no tenant can change: separation of
  duties on the final credit, reassignment only by a lead of the dispute's team, tenant administration only through
  the realm role, and what tenant keys may do. Grants (which role in which team may take which action, optionally up
  to an amount) are rows compiled to `permit` policies, each bounded to its team. A row that cannot be expressed
  safely is skipped, never escaped, so a bad row can only take a permission away. Until ADR 0027 stores grants per
  tenant, every analyst is a lead of one default team holding every grant, so the only change in behaviour is the
  guardrails.
- **Where it runs.** The service decides inside the request's transaction, after loading the dispute, so policies
  see what was committed. A denial is 403 `forbidden` naming the policy that decided it. The allowed events on a
  dispute are the ones the state machine accepts *and* the caller may apply. An allowed event records the policies
  that allowed it in its payload (`authorizedBy`). Idempotency keys belong to the caller, so a replay cannot hand one
  principal a response only another was authorized to see.
- **Nothing unclassified.** The API refuses to start if an operation in the OpenAPI document is not classified as
  public, internal, tenant-wide, or decided by the engine.

## Consequences

- Easier: a new control is a policy and its tests, not a check scattered across handlers; the log answers why an
  event was allowed; tenant keys and people are held to the same rules; forgetting to authorize a new operation
  fails at start-up rather than in production.
- Harder: every dispute decision reads the event log and the transaction, which adds reads; an analyst who
  investigated a dispute can no longer finish it alone, so small teams need a second person or a tenant key for the
  final credit; tests must act as a principal, and a context with only a tenant is refused.
