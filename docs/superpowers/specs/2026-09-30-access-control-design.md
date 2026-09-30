# Access control by team, action and data type

Status: design, awaiting review
Date: 2026-09-30

## Purpose

A thesis contribution first: adopt a recognised policy language, justify the model, and evaluate it for
correctness, cost and the threats it closes. The implementation is the evidence.

Today authorization stops at the tenant boundary. Row-level security isolates tenants, but inside a
tenant any analyst or tenant key can apply every dispute event, read every section of a dispute
(cardholder PII, ledger, risk signals, questionnaire answers, communications, attachments), and the
event actor is taken from the request body. Only `tenant-admin` is checked, ad hoc, in seven handlers.

## Decisions

| Question | Decision |
|---|---|
| Model | Teams own disputes; a fixed role ladder inside each team; conditions on dispute attributes |
| Policy authorship | Policy as code, versioned in the repo; membership from Keycloak groups |
| Data-type access | Per data type and role: `full`, `masked` or `none` |
| Tenant keys | Principals in the same model, each with a named profile |
| Separation of duties | Static, history-aware rule; no approval workflow |
| Engine | Cedar, embedded through `cedar-go`; team scope mirrored in Postgres RLS |
| Denied section in UI | Hidden; no "hidden by policy" state |

Rejected: OPA/Rego (weaker formal story for the thesis), a custom DSL (must defend inventing one),
self-service policy editing (UI cost with little thesis value), maker-checker approvals (doubles scope).

## Model

### Principals

- `Analyst`: Keycloak subject. Memberships come from Keycloak groups named `/teams/<team>/<role>`,
  delivered in a `groups` claim by a new mapper on the `dispute-api` client scope.
- `TenantKey`: one `profile`, stored in a new `tenant_keys.profile` column, set at issue time.
- `tenant-admin` stays a realm role; tenant administration becomes Cedar actions on `Tenant`, and the
  scattered `RequireRole` calls go away.

Roles are fixed across tenants: `junior`, `senior`, `lead`. Teams are declared per tenant. A group
naming an undeclared team is ignored with a warning.

Membership encoding: three cumulative sets of team names on the principal, following the fixed
ladder: `memberOf` (any role), `seniorOf` (senior or lead), `leadOf` (lead). Rules read as
`principal.seniorOf.contains(resource.team)`. Cedar has neither dynamic record indexing nor string
concatenation, so the ladder is resolved when the principal is built, not in policy.

```cedar
@id("sod-final-credit")
forbid (principal is Analyst, action == Action::"ISSUE_FINAL_CREDIT", resource is Dispute)
when { resource.investigationOpenedBy == principal };

@id("final-credit-threshold")
permit (principal is Analyst, action == Action::"ISSUE_FINAL_CREDIT", resource is Dispute)
when { principal.seniorOf.contains(resource.team)
       || (principal.memberOf.contains(resource.team)
           && resource.amount.lessThanOrEqual(decimal("500.00"))) };
```

### Resources

- `Dispute`: `team`, `amount` (decimal), `currency`, `rail`, `riskTier`, `state`,
  `investigationOpenedBy` (derived from the event log).
- `Tenant`: for administration actions.

### Actions

- Events: one per state-machine event (13). The state machine decides legality; Cedar decides
  permission.
- Operations: `list`, `create`, `compose-email`, `upload-attachment`, `resend-notice`, `suggest`,
  `reassign`, and the administration actions (tenant keys, templates, logo).
- Reads per data type: `read:<type>:full` and `read:<type>:masked` for `pii`, `financial`, `risk`,
  `questionnaire`, `communications`, `attachments`. The effective level is the highest allowed, else
  `none`.

### Routing

A dispute's team is set at creation by a first-match routing table (rail, reason, risk tier, amount,
then a default team) and stored in `disputes.team`. Re-routing is the audited `reassign` action,
allowed to leads.

### Files

```
policy/
  schema.cedarschema
  base.cedar             # shared: separation of duties, tenant-key profiles
  tenants/<slug>/
    teams.cedar          # team and role permits, thresholds
    routing.yaml
```

Embedded with `go:embed`; a tenant's policy set is `base` plus its own directory.

## Enforcement

1. **Principal resolution** in `auth.Bearer`: a JWT becomes an `Analyst`, a key becomes a `TenantKey`.
   Every request carries exactly one principal.
2. **Action gate**: `authz.Check(ctx, action, resource)` at the top of each handler, after the
   resource is loaded in the same transaction. Denial is 403 in the ADR 0012 problem shape with a
   reason (`role`, `separation-of-duties`, `profile`) naming the deciding policy id. A dispute outside
   the caller's teams is 404, supplied by RLS. The client-supplied `actor` on `CreateDispute` and
   `ApplyDisputeEvent` is removed; the actor is always the principal. A startup check fails boot if any
   OpenAPI operation lacks an action mapping or an explicit public/internal marker.
3. **Row scope**: `Store.WithTx` also sets `app.teams` and `app.principal_kind`. A `team_scope` policy
   on `disputes` admits `team = ANY(current_teams())` or a key principal; child tables are scoped by
   joining to their dispute.
4. **Data-type projection**: `authz.Levels(ctx, dispute)` decides all six levels in one batch;
   `project(aggregate, levels)` removes `none` sections (optional in OpenAPI) and applies a fixed
   masker per type for `masked`: email `j***@domain`, postal address to city and country, risk tier
   without signals, ledger balances without lines, attachment metadata without content.
   `allowedEvents` becomes the intersection of state-machine legality and Cedar permission.

Frontend holds no policy logic. It renders what the API returns, hides absent sections, offers only
`allowedEvents`, and reads a `capabilities` list from a new `GET /me` in place of
`roles.includes('tenant-admin')`.

## Audit, telemetry, failure

- Each allowed dispute event records `authorized_by` (deciding policy ids) in its payload.
- Spans carry `authz.decision` and `authz.policies`; a counter by action and decision.
- Fail closed: unparseable policy stops boot; evaluation errors deny and are recorded on the span.

## Verification

1. `cedar validate --schema` for every tenant policy set in CI, with the pinned reference CLI
   (`cedar-go` schema validation is experimental).
2. Decision-table golden: role x team relation x action x dispute fixtures; every policy change is a
   reviewable diff.
3. Property tests: Cedar team scope equals RLS row scope; no separation-of-duties bypass; masking
   never exceeds a granted level and `none` leaks nothing; keys never read PII beyond their profile.
4. Integration negatives over HTTP and Postgres: cross-team read is 404, forged actor is rejected,
   tenant key `ISSUE_REFUND` is 403.

## Evaluation (thesis)

- Threat coverage table, before and after: leaked tenant key issuing refunds, forged actor,
  cross-team read, insider self-approval, PII to the wrong staff.
- Cost: p50/p99 added on `GetDispute` and `ApplyDisputeEvent`, authorization off versus on, with the
  existing eval harness; `ListDisputes` cost of the team-scope RLS at the eval dataset size.
- Expressiveness limits: what the model cannot state (four-eyes approval, per-customer ownership).

## Delivery

| PR | Scope | ADR |
|---|---|---|
| P1 | `authz` package, `cedar-go`, schema, base policy with separation of duties, principal model, actor removal, coverage check; default policy grants everything | 0026 |
| P2 | Keycloak group mapper, `disputes.team`, routing, team-scope RLS, `reassign` | 0027 |
| P3 | Projection, maskers, optional sections, UI hides absent sections | |
| P4 | Tenant key profiles: column, CLI, `/keys` UI, profile policies | |
| P5 | CI validation, decision-table golden, property tests, benchmarks, thesis chapter | |

Each PR carries its thesis section, per the write-as-you-build habit.

## Out of scope

Approval workflows, self-service policy editing, per-customer ownership, per-tenant role ladders.
