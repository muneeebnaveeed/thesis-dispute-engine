# 0027: Teams own disputes; grants are data; row-level security scopes rows by team

**Status:** accepted, 2026-09-30; builds on ADR 0008 and 0026

## Context

ADR 0026 put every operation behind Cedar policies but gave every analyst every grant on one default team. Issuers
split disputes between teams (chargebacks, fraud, general handling), give juniors a ceiling on what they may pay
out, and expect a tenant administrator, not the vendor, to decide who is in which team. The policy model has to allow
that later editing without trusting what gets edited, and a policy check forgotten in one handler should not be
enough to show one team another team's disputes.

## Decision

A dispute belongs to one team. What each role in each team may do is tenant data; what no tenant may change stays
Cedar in the binary. Row-level security enforces team membership a second time, independently of the policy engine.

- **Data.** Tenant-scoped tables hold teams (one of them the default), memberships (Keycloak subject, team, role),
  grants (team, role, action, optional amount ceiling) and routing rules (first match on rail, reason and a minimum
  amount; a risk tier column is reserved, see Routing). The migration gives every tenant a `general` team holding today's grants, and new tenants get
  it from a trigger. Every change to these tables bumps a per-tenant policy version by trigger. The engine caches
  each tenant's compiled policy set by that version, so a change applies on the next request.
- **Why grants can be edited safely.** Grants compile only to `permit` policies whose resource is inside their own
  team. The rules that must always hold are `forbid` policies in the binary, and Cedar lets a `forbid` override any
  `permit`. So no set of rows can break separation of duties, reach another team or administer the tenant; property
  tests generate arbitrary grant sets to check this. Editing is left to an operator command for now
  (`scripts/tenant member add`); a tenant-admin interface is the next step and needs no change to the model.
- **The ladder.** `lead` is a member of `senior`, which is a member of `junior`, as Cedar entities. A grant to junior
  reaches every rung above it through Cedar's `in`, so there is no role logic in code.
- **Routing.** A new dispute goes to the first matching rule's team, else the default. A dispute is routed before
  its first risk assessment, so rules on risk tier are refused until routing can see one. Creating a dispute is
  authorized against that team, so an analyst cannot open a dispute that would land outside their teams. A lead
  moves a dispute with `POST /disputes/{id}/team`, logged as a `REASSIGNED` event.
- **Row-level security.** `WithTx` binds the principal next to the tenant, and restrictive policies on `disputes`
  and every child table admit only rows of the caller's teams (tenant keys see the whole tenant). They are
  restrictive because Postgres ORs permissive policies, and a permissive team policy would widen tenant isolation
  rather than narrow it. A dispute outside the caller's teams is therefore 404, as if it did not exist. Moving a
  dispute out of one's own teams cannot pass a row policy, so it goes through one owner-run function that still
  requires the caller's tenant, current membership and the version they read.
- **Checked by the reference implementation.** CI validates the guardrails and every seed fixture's compiled grants
  with the pinned Cedar CLI, and golden files pin each fixture's compiled policies and full decision table.

## Consequences

- Easier: a tenant's structure is rows a UI can edit; the decision table of any policy change is a diff; a missing
  check in code still cannot leak another team's disputes; the model is testable without a database.
- Harder: an analyst with no membership sees nothing, so every real user needs one (the seeded users get theirs from
  fixtures; others need the operator command once); every dispute read joins the membership table; reassignment is
  the one path that runs as the owner and must stay narrow.
