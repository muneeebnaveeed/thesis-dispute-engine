# Access control by team, action and data type

Status: design, approved 2026-09-30
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
| Policy authorship | Rules in code, grants in data: schema and guardrails in the repo; teams, members, grants and routing in Postgres, seeded now, editable by a later self-service UI |
| Data-type access | Per data type and role: `full`, `masked` or `none` |
| Tenant keys | Principals in the same model, each with a named profile |
| Separation of duties | Static, history-aware rule; no approval workflow |
| Engine | Cedar, embedded through `cedar-go`; team scope mirrored in Postgres RLS |
| Denied section in UI | Hidden; no "hidden by policy" state |

Rejected: OPA/Rego (weaker formal story for the thesis), a custom DSL (must defend inventing one),
policy as files only (a later UI would have to write Cedar text), membership in Keycloak groups (a
later UI would have to drive the Keycloak admin API), maker-checker approvals (doubles scope).

## Rules in code, grants in data

Two layers, kept honest by Cedar's semantics: any `forbid` overrides every `permit`.

- **Guardrails (code, `policy/`):** the Cedar schema and hand-written policies no tenant may change:
  separation of duties, tenant-key profiles, tenant administration, and bounds such as "only leads
  reassign". Written as `forbid` wherever they must hold.
- **Grants (data, Postgres):** what a tenant admin will one day edit, compiled into `permit`
  policies. Whatever a future UI grants, it cannot open what a guardrail forbids; this is the
  argument that self-service is safe.

### Tables (tenant-scoped, under the existing RLS)

- `teams (tenant_id, slug, name, is_default)`
- `team_members (tenant_id, team, subject, role)`, `role` in `junior | senior | lead`
- `role_grants (tenant_id, team, role, action, amount_limit null)`; `action` is an event, an
  operation, or a data-type read level such as `read:pii:masked`
- `routing_rules (tenant_id, position, rail null, reason null, risk_tier null, min_amount null, team)`
- `policy_versions (tenant_id, version)`, bumped by a trigger on any write to the tables above

Seeded per tenant from fixtures (`deploy/seed/access/<slug>.yaml`) by a seed command. No editing API
or UI in this work.

### Compilation

A tenant's policy set is the guardrails plus its grants compiled to Cedar, one policy per row:

```cedar
// row (chargebacks, senior, ISSUE_FINAL_CREDIT, null)
@id("grant:chargebacks/senior/ISSUE_FINAL_CREDIT")
permit (principal in TeamRole::"chargebacks/senior",
        action == Action::"ISSUE_FINAL_CREDIT",
        resource in Team::"chargebacks");

// row (chargebacks, junior, ISSUE_FINAL_CREDIT, 500.00)
@id("grant:chargebacks/junior/ISSUE_FINAL_CREDIT")
permit (principal in TeamRole::"chargebacks/junior",
        action == Action::"ISSUE_FINAL_CREDIT",
        resource in Team::"chargebacks")
when { resource.amount.lessThanOrEqual(decimal("500.00")) };
```

Compiled sets are cached per tenant keyed by `policy_versions.version`; a request reads the version in
its transaction and recompiles on mismatch, so a grant change takes effect on the next request.

## Model

### Principals

- `Analyst`: Keycloak subject. Parents are its `TeamRole` entities from `team_members`. Derived
  attribute `leadOf: Set<Team>` for guardrails that need "lead of this dispute's team". Attribute
  `tenantAdmin: Bool` from the Keycloak realm role.
- `TenantKey`: one `profile`, stored in a new `tenant_keys.profile` column, set at issue time.

`tenant-admin` stays a Keycloak realm role; tenant administration is guarded by `principal.tenantAdmin`
in the guardrails, and the scattered `RequireRole` calls go away. It is also the role that will edit
grants.

### The ladder as an entity hierarchy

`TeamRole::"<team>/lead"` is a member of `TeamRole::"<team>/senior"`, which is a member of
`TeamRole::"<team>/junior"`. A grant to junior reaches senior and lead through Cedar's `in`, with no
string building. Roles are fixed across tenants; teams are per tenant.

### Resources

- `Dispute`: parent `Team::"<team>"`; attributes `team` (entity), `amount` (decimal), `currency`,
  `rail`, `riskTier`, `state`, `investigationOpenedBy` (derived from the event log).
- `Team`, `Tenant`.

### Actions

- Events: one per state-machine event (13). The state machine decides legality; Cedar decides
  permission.
- Operations: `list`, `create`, `compose-email`, `upload-attachment`, `resend-notice`, `suggest`,
  `reassign`, and the administration actions (tenant keys, templates, logo).
- Reads per data type: `read:<type>:full` and `read:<type>:masked` for `pii`, `financial`, `risk`,
  `questionnaire`, `communications`, `attachments`. The effective level is the highest allowed, else
  `none`.

### Guardrail examples

```cedar
@id("sod-final-credit")
forbid (principal is Analyst, action == Action::"ISSUE_FINAL_CREDIT", resource is Dispute)
when { resource.investigationOpenedBy == principal };

@id("reassign-leads-only")
forbid (principal, action == Action::"reassign", resource is Dispute)
unless { principal is Analyst && principal.leadOf.contains(resource.team) };
```

### Routing

A dispute's team is set at creation by the first matching row of `routing_rules`, falling back to the
tenant's default team, and stored in `disputes.team`. Re-routing is the audited `reassign` action.

## Enforcement

1. **Principal resolution** in `auth.Bearer`: a JWT becomes an `Analyst`, its `team_members` rows
   loaded in the request transaction; a key becomes a `TenantKey`. Every request carries exactly one
   principal.
2. **Action gate**: `authz.Check(ctx, action, resource)` at the top of each handler, after the
   resource is loaded in the same transaction. Denial is 403 in the ADR 0012 problem shape with a
   reason naming the deciding policy id. A dispute outside the caller's teams is 404, supplied by RLS.
   The client-supplied `actor` on `CreateDispute` and `ApplyDisputeEvent` is removed; the actor is
   always the principal. A startup check fails boot if any OpenAPI operation lacks an action mapping
   or an explicit public/internal marker.
3. **Row scope**: `Store.WithTx` also sets `app.subject` and `app.principal_kind`. A `team_scope`
   policy on `disputes` admits rows whose team has a `team_members` row for the current subject, or a
   key principal; child tables are scoped by joining to their dispute.
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
- Spans carry `authz.decision`, `authz.policies` and `authz.policy_version`; a counter by action and
  decision.
- Fail closed: unparseable guardrails stop boot; a grant row that fails to compile is skipped and
  reported (it could only have added a permit); evaluation errors deny and are recorded on the span.

## Verification

1. `cedar validate --schema` in CI over the guardrails and the compiled set of every seed fixture,
   with the pinned reference CLI (`cedar-go` schema validation is experimental).
2. Goldens: compiled Cedar per seed fixture, and a decision table (role x team relation x action x
   dispute fixtures) per fixture.
3. Property tests: Cedar team scope equals RLS row scope; no generated grant set bypasses a
   guardrail; masking never exceeds a granted level and `none` leaks nothing; keys never read PII
   beyond their profile.
4. Integration negatives over HTTP and Postgres: cross-team read is 404, forged actor is rejected,
   tenant key `ISSUE_REFUND` is 403, a grant change takes effect on the next request.

## Evaluation (thesis)

- Threat coverage table, before and after: leaked tenant key issuing refunds, forged actor,
  cross-team read, insider self-approval, PII to the wrong staff, an over-generous tenant admin.
- Cost: p50/p99 added on `GetDispute` and `ApplyDisputeEvent`, authorization off versus on, with the
  existing eval harness; `ListDisputes` cost of the team-scope RLS at the eval dataset size;
  recompilation time per tenant.
- Expressiveness limits: what the model cannot state (four-eyes approval, per-customer ownership).

## Delivery

| PR | Scope | ADR |
|---|---|---|
| P1 | `authz` package, `cedar-go`, schema, guardrails, principal model, actor removal, coverage check; a default grant set that preserves today's behaviour | 0026 |
| P2 | Access tables, seed fixtures, compiler with version cache, `disputes.team`, routing, team-scope RLS, `reassign` | 0027 |
| P3 | Projection, maskers, optional sections, UI hides absent sections | |
| P4 | Tenant key profiles: column, CLI, `/keys` UI, profile policies | |
| P5 | CI validation, goldens, property tests, benchmarks, thesis chapter | |
| later | Self-service: grants and membership API and UI for `tenant-admin` | |

Each PR carries its thesis section, per the write-as-you-build habit. The first implementation plan
covers P1 and P2.

## Out of scope

Approval workflows, the self-service editing API and UI (designed for, not built), per-customer
ownership, per-tenant role ladders.
