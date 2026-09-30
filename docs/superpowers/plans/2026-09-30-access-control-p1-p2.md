# Access Control P1 and P2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Every API operation is authorized by Cedar against a principal (analyst or tenant key), with teams owning disputes, a junior/senior/lead ladder, grants stored in Postgres, guardrails in code, and team scope enforced a second time by row-level security.

**Architecture:** A platform `principal` package carries who is calling. A pure `internal/dispute/authz` package compiles grant rows plus embedded guardrail policies into a Cedar policy set (cached per tenant and policy version) and decides requests. The application service loads the dispute and the caller's access data in the request transaction, asks an `Authorizer` port, and refuses with 403 naming the deciding policy. Postgres adds restrictive team-scope policies beside tenant isolation so a missed check still cannot leak another team's dispute.

**Tech Stack:** Go 1.27, `github.com/cedar-policy/cedar-go` v1.8.0, PostgreSQL 17 with RLS, sqlc, kin-openapi strict server, Keycloak realm template.

**Spec:** `docs/superpowers/specs/2026-09-30-access-control-design.md`

## Global Constraints

- Two PRs: PR A (Tasks 1 to 6, spec P1) and PR B (Tasks 7 to 12, spec P2). PR A must preserve today's behaviour except the actor change and separation of duties.
- Guardrails are code; grants are data. Cedar `forbid` overrides every `permit`; grants compile only to `permit` whose resource is `in Team::"<slug>"`.
- Roles are exactly `junior`, `senior`, `lead`; `lead` is a member of `senior`, `senior` of `junior`.
- Fail closed everywhere: no principal means deny; unparseable guardrails stop boot; an evaluation error denies.
- A dispute outside the caller's teams is 404 (RLS), not 403.
- The actor is always the principal; request bodies no longer carry `actor`.
- Terminology: "tenant keys", never "API keys".
- Writing: no em dashes, no emojis; comments one line, why only; commits `type(scope): summary`, no trailers.
- Spec first: HTTP changes start in `docs/api/openapi.yaml`, then `make api:generate`.
- Errors via `errs.New(kind, code, message)`; the 403 code is the existing `forbidden`.
- Run Go through the pinned toolchain: prefix commands with `mise exec --` (or have mise activated).
- Integration tests need `make db:up` first and run with `make api:test-integration`.

## Review Focus

1. **The same analyst opens an investigation and later issues the final credit** (the eval correctness suites and the browser suite do this): after PR A it is 403 `sod-final-credit`. Expected: suites updated to use a second analyst or a tenant key for the credit, never the guardrail relaxed. Pinned in Task 6.
2. **Background and test code that sets only a tenant** (`tenant.WithID`) now has no principal and is denied. Expected: tests move to `apptest.Ctx()`/`apptest.CtxFor`, which carry a lead principal; production has no such path today. Pinned in Task 4.
3. **An existing real user (for example `muneeb` in the OTP realm) after PR B:** no `team_members` row, so they see no disputes. Expected: a documented operator command adds them; the dev seed covers seeded users. Pinned in Task 11.
4. **Postgres ORs permissive policies:** adding team scope as a normal policy would silently widen tenant isolation. Expected: team-scope policies are `AS RESTRICTIVE`, and a test proves a cross-tenant row stays invisible. Pinned in Task 8.
5. **An analyst creates a dispute that routes to a team they are not in:** expected 403 before any insert, not a created-then-invisible dispute. Pinned in Task 9.

---

## PR A: authorization core (spec P1)

Create the branch from `main`: `authz/access-control-core`.

### Task 1: The principal package, set for both credential kinds

**Files:**
- Create: `backend/internal/platform/principal/principal.go`
- Create: `backend/internal/platform/principal/principal_test.go`
- Modify: `backend/internal/platform/auth/auth.go` (Resolver, Bearer, WithPrincipal)
- Modify: `backend/internal/dispute/infrastructure/postgres/keys.go` (`TenantForKeyHash` becomes `KeyForHash`)
- Modify: `backend/internal/dispute/infrastructure/postgres/queries/*.sql` (the `GetTenantByTenantKeyHash` query, line 84)
- Test: `backend/internal/platform/auth/auth_test.go`

**Interfaces:**
- Produces:
  ```go
  package principal
  type Kind string
  const (Analyst Kind = "analyst"; Key Kind = "key")
  type Principal struct { Kind Kind; ID string; Display string; TenantAdmin bool }
  func With(ctx context.Context, p Principal) context.Context
  func From(ctx context.Context) (Principal, bool)
  ```
  and in `auth`: `type KeyIdentity struct { Tenant, ID uuid.UUID; Prefix string }`, `Resolver.KeyForHash(ctx, hash []byte) (KeyIdentity, error)`.

- [ ] **Step 1: Write the failing test for the package**

```go
package principal_test

import (
	"context"
	"testing"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/principal"
)

func TestRoundTrip(t *testing.T) {
	if _, ok := principal.From(context.Background()); ok {
		t.Fatal("empty context carried a principal")
	}
	p := principal.Principal{Kind: principal.Analyst, ID: "sub-1", Display: "a@x", TenantAdmin: true}
	got, ok := principal.From(principal.With(context.Background(), p))
	if !ok || got != p {
		t.Fatalf("got %+v %v", got, ok)
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `cd backend && mise exec -- go test ./internal/platform/principal/`
Expected: FAIL, package does not exist.

- [ ] **Step 3: Implement**

```go
// Package principal carries who is calling, for both credential kinds, so authorization and audit never branch on
// how the caller authenticated.
package principal

import "context"

// Kind is how the caller authenticated.
type Kind string

const (
	Analyst Kind = "analyst"
	Key     Kind = "key"
)

// Principal is the authenticated caller. ID is stable (the OIDC subject or the tenant key id) and is what policies and
// the audit trail compare; Display is for people reading the log.
type Principal struct {
	Kind        Kind
	ID          string
	Display     string
	TenantAdmin bool
}

type ctxKey struct{}

// With returns ctx carrying p.
func With(ctx context.Context, p Principal) context.Context { return context.WithValue(ctx, ctxKey{}, p) }

// From returns the caller, if the request authenticated.
func From(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(ctxKey{}).(Principal)
	return p, ok
}
```

- [ ] **Step 4: Run it and see it pass**

Run: `cd backend && mise exec -- go test ./internal/platform/principal/`
Expected: PASS.

- [ ] **Step 5: Write the failing auth test**

Add to `backend/internal/platform/auth/auth_test.go` (reuse its existing fake resolver; change the fake to implement `KeyForHash` returning `auth.KeyIdentity{Tenant: tid, ID: kid, Prefix: "tk_abcdefghi"}`):

```go
func TestBearerSetsKeyPrincipal(t *testing.T) {
	var got principal.Principal
	h := auth.Bearer(fakeKeys, nil)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got, _ = principal.From(r.Context())
	}))
	req := httptest.NewRequest(http.MethodGet, "/disputes", nil)
	req.Header.Set("Authorization", "Bearer tk_valid")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if got.Kind != principal.Key || got.ID != kid.String() || got.Display != "key:tk_abcdefghi" || got.TenantAdmin {
		t.Fatalf("principal = %+v", got)
	}
}
```

Also extend the existing OIDC principal test (the one that calls `auth.WithPrincipal`) to assert `principal.From` returns `{Kind: Analyst, ID: <subject>, Display: <email>, TenantAdmin: <has tenant-admin>}`.

- [ ] **Step 6: Run and see it fail**

Run: `cd backend && mise exec -- go test ./internal/platform/auth/`
Expected: compile failure (`KeyForHash`, `principal` unknown).

- [ ] **Step 7: Implement in auth and the key store**

In `auth.go`:

```go
// KeyIdentity is a live tenant key.
type KeyIdentity struct {
	Tenant uuid.UUID
	ID     uuid.UUID
	Prefix string
}

// Resolver maps a key hash to its key; application.ErrNotFound for unknown or revoked keys.
type Resolver interface {
	KeyForHash(ctx context.Context, hash []byte) (KeyIdentity, error)
}
```

`WithPrincipal` also sets the platform principal:

```go
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	display := p.Email
	if display == "" {
		display = p.Subject
	}
	ctx = principal.With(ctx, principal.Principal{Kind: principal.Analyst, ID: p.Subject, Display: display, TenantAdmin: p.HasRole(RoleTenantAdmin)})
	return tenant.WithID(context.WithValue(ctx, principalKey{}, p), p.Tenant)
}
```

In `Bearer`, replace the key branch:

```go
} else {
	var k KeyIdentity
	if k, err = keys.KeyForHash(authCtx, HashKey(cred)); err == nil {
		id = k.Tenant
		ctx = principal.With(ctx, principal.Principal{Kind: principal.Key, ID: k.ID.String(), Display: "key:" + k.Prefix})
	}
}
```

In the queries file, make the lookup return the prefix:

```sql
-- name: GetTenantByTenantKeyHash :one
SELECT k.id, k.tenant_id, k.prefix FROM tenant_keys k
JOIN tenants t ON t.id = k.tenant_id AND t.disabled_at IS NULL
WHERE k.key_hash = $1 AND k.revoked_at IS NULL AND (k.expires_at IS NULL OR k.expires_at > now());
```

In `keys.go`, rename and return the identity:

```go
// KeyForHash returns a live key; unknown, revoked and expired keys are all ErrNotFound.
func (k *KeyStore) KeyForHash(ctx context.Context, hash []byte) (auth.KeyIdentity, error) {
	row, err := sqlcgen.New(k.pool).GetTenantByTenantKeyHash(ctx, hash)
	if err != nil {
		return auth.KeyIdentity{}, mapErr(err)
	}
	k.touch(ctx, row.ID)
	return auth.KeyIdentity{Tenant: row.TenantID, ID: row.ID, Prefix: row.Prefix}, nil
}
```

Run `make api:generate`, then fix every other caller of `TenantForKeyHash` (`grep -rn TenantForKeyHash backend`), including `keys_test.go`.

- [ ] **Step 8: Run the unit and integration tests**

Run: `make api:test && make api:test-integration`
Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add backend
git commit -m "feat(auth): one principal for analysts and tenant keys, carried on the request"
```

### Task 2: The actor comes from the principal

**Files:**
- Create: `backend/migrations/0022_event_actor_id.sql`
- Modify: `docs/api/openapi.yaml:966` and `:980` (drop `actor` from the create and apply request bodies)
- Modify: `backend/internal/dispute/application/ports.go` (`EventRecord.ActorID`)
- Modify: `backend/internal/dispute/application/service.go:292-310,352,417` (remove `Actor` from inputs; set from principal)
- Modify: `backend/internal/dispute/ports/http/handler.go:350,755` (stop passing actor)
- Modify: `backend/internal/dispute/infrastructure/postgres/queries/*.sql` (`AppendEvent`, `ListEvents` carry `actor_id`), `store.go`, `apptest/memstore.go`
- Modify: frontend server functions that send `actor` (`grep -rn "actor" frontend/src/server/functions`), then `make workbench:generate`
- Test: `backend/internal/dispute/application/service_test.go`, `backend/internal/dispute/ports/http/handler_test.go`

**Interfaces:**
- Consumes: `principal.From`.
- Produces: `EventRecord.ActorID string`; `application.actorOf(ctx) (display, id string)`.

- [ ] **Step 1: Write the failing service test**

```go
func TestActorComesFromPrincipal(t *testing.T) {
	store := apptest.NewMemStore()
	svc := apptest.NewService(t, store)
	ctx := apptest.CtxAs(principal.Principal{Kind: principal.Analyst, ID: "sub-7", Display: "ana@bank.example"})
	txn := store.AddTransaction(domain.RailCard, "EUR", "EUR", "40.00")
	res, err := svc.CreateDispute(ctx, application.CreateDisputeInput{TransactionID: txn})
	if err != nil {
		t.Fatal(err)
	}
	ev := store.Events[res.View.ID][0]
	if ev.Actor != "ana@bank.example" || ev.ActorID != "sub-7" {
		t.Fatalf("actor = %q/%q", ev.Actor, ev.ActorID)
	}
}
```

`apptest.NewService`, `apptest.CtxAs` are introduced in Task 4; for this task add them now in `apptest/memstore.go` in their simple form:

```go
// CtxAs returns a TenantA context carrying p.
func CtxAs(p principal.Principal) context.Context {
	return principal.With(tenant.WithID(context.Background(), TenantA), p)
}

// NewService builds a service over store with the given options.
func NewService(t testing.TB, store application.Store, opts ...application.Option) *application.Service {
	t.Helper()
	svc, err := application.NewService(store, nil, opts...)
	if err != nil {
		t.Fatal(err)
	}
	return svc
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `cd backend && mise exec -- go test ./internal/dispute/application/ -run TestActorComesFromPrincipal`
Expected: compile failure (`ActorID` unknown).

- [ ] **Step 3: Migration**

`backend/migrations/0022_event_actor_id.sql`:

```sql
-- The actor used to be whatever the request body said. It is now the authenticated principal: actor stays the
-- readable name, actor_id is the stable identity policies compare (the OIDC subject or the tenant key id).
ALTER TABLE dispute_events ADD COLUMN actor_id text;
```

- [ ] **Step 4: Implement**

In `ports.go` add `ActorID string` to `EventRecord` after `Actor`. Remove `Actor` from `CreateDisputeInput` and `ApplyEventInput`. In `service.go` add:

```go
// actorOf is who the audit trail names; a request without a principal is refused before it gets here.
func actorOf(ctx context.Context) (display, id string) {
	p, _ := principal.From(ctx)
	return p.Display, p.ID
}
```

and in both `EventRecord` literals replace `Actor: in.Actor,` with `Actor: display, ActorID: id,` after `display, id := actorOf(ctx)` at the top of the closure. Update the sqlc `AppendEvent` insert and `ListEvents` select to include `actor_id`, run `make api:generate`, map it in `store.go` (nil-safe: `ActorID: derefOr(row.ActorID, "")`, or the existing helper for nullable text), and store it in `memstore.go`'s `AppendEvent` (it already appends the record whole).

In `openapi.yaml` delete the two `actor:` properties at lines 966 and 980. In `handler.go` delete `Actor: orDefault(req.Body.Actor, "customer"),` and `Actor: orDefault(req.Body.Actor, "system"),`; remove `orDefault` if now unused. Run `make api:generate` and `make workbench:generate`; remove `actor` from any frontend request body the typecheck flags.

- [ ] **Step 5: Handler test: a forged actor is ignored**

Add to `handler_test.go` (it already builds a server over a MemStore; use its existing request helper and the analyst token or `CtxAs` it uses):

```go
func TestApplyEventRejectsActorField(t *testing.T) {
	// the body schema no longer has actor, and the validator refuses unknown properties
	rec := doJSON(t, srv, http.MethodPost, "/disputes/"+id+"/events", `{"event":"OPEN_INVESTIGATION","actor":"someone-else"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
}
```

If the schema allows additional properties (check `additionalProperties` on the request schema), assert instead that the stored event's `ActorID` equals the caller's ID, not `someone-else`.

- [ ] **Step 6: Run everything**

Run: `make api:test && make api:test-integration && make workbench:check`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add backend docs/api frontend
git commit -m "feat(disputes)!: the actor on an event is the authenticated caller, never the request body"
```

### Task 3: The Cedar engine

**Files:**
- Create: `backend/internal/dispute/authz/policy/schema.cedarschema`
- Create: `backend/internal/dispute/authz/policy/guardrails.cedar`
- Create: `backend/internal/dispute/authz/authz.go` (engine, cache, Decide)
- Create: `backend/internal/dispute/authz/compile.go` (grant rows to Cedar)
- Create: `backend/internal/dispute/authz/entities.go`
- Create: `backend/internal/dispute/application/access.go` (the port and its types)
- Test: `backend/internal/dispute/authz/authz_test.go`, `backend/internal/dispute/authz/compile_test.go`
- Modify: `backend/go.mod` (`go get github.com/cedar-policy/cedar-go@v1.8.0`)

**Interfaces:**
- Consumes: `principal.Principal`, `domain.Event`, `domain.Rail`, `domain.State`.
- Produces (in `application/access.go`):
  ```go
  type Role string
  const (RoleJunior Role = "junior"; RoleSenior Role = "senior"; RoleLead Role = "lead")
  type Action string
  const (
  	ActionCreate Action = "create"; ActionComposeEmail Action = "compose-email"
  	ActionUploadAttachment Action = "upload-attachment"; ActionResendNotice Action = "resend-notice"
  	ActionReassign Action = "reassign"; ActionManageKeys Action = "manage-keys"
  	ActionManageTemplates Action = "manage-templates"; ActionManageBranding Action = "manage-branding"
  )
  func EventAction(e domain.Event) Action
  type Grant struct { Team string; Role Role; Action Action; AmountLimit *decimal.Decimal }
  type Membership struct { Team string; Role Role }
  type RoutingRule struct { Rail *domain.Rail; Reason *domain.Reason; RiskTier *string; MinAmount *decimal.Decimal; Team string }
  type Access struct { Version int64; DefaultTeam string; Teams []string; Grants []Grant; Routing []RoutingRule; Members []Membership }
  type DisputeFacts struct { ID uuid.UUID; Team string; Amount decimal.Decimal; Currency string; Rail domain.Rail; State domain.State; InvestigationOpenedBy string }
  type Decision struct { Allowed bool; Policies []string }
  type Authorizer interface {
  	Decide(tenantID uuid.UUID, p principal.Principal, a Access, action Action, d *DisputeFacts) (Decision, error)
  }
  ```
  `d == nil` means the resource is the team named by `a`'s routing (for `create`, pass `&DisputeFacts{Team: routed}` with zero ID) or the tenant (for administration, pass nil).
- Produces (in `authz`): `func New(log *slog.Logger) (*Engine, error)`; `(*Engine).Decide` implementing `application.Authorizer`; `func Compile(tenantID uuid.UUID, a application.Access) (src string, skipped []string)` returning Cedar source and the grants it refused (a refused grant could only have added a permit, so skipping it fails closed).

- [ ] **Step 1: Write the schema and guardrails**

`policy/schema.cedarschema`:

```
entity Team;
entity TeamRole in [TeamRole];
entity Tenant;
entity Analyst in [TeamRole] { leadOf: Set<Team>, tenantAdmin: Bool };
entity TenantKey { profile: String };
entity Dispute in [Team] {
  team: Team,
  amount: decimal,
  currency: String,
  rail: String,
  state: String,
  investigationOpenedBy: String,
};

action "OPEN_INVESTIGATION", "SEND_QUESTIONNAIRE", "RECEIVE_QUESTIONNAIRE", "ISSUE_REFUND", "FILE_CHARGEBACK",
       "ACKNOWLEDGE_CHARGEBACK", "SUBMIT_EVIDENCE", "WIN_CHARGEBACK", "LOSE_CHARGEBACK", "ISSUE_FINAL_CREDIT",
       "REVERSE_PROVISIONAL_CREDIT", "CLOSE", "APPEAL", "compose-email", "upload-attachment", "resend-notice",
       "reassign"
  appliesTo { principal: [Analyst, TenantKey], resource: [Dispute] };
action "create" appliesTo { principal: [Analyst, TenantKey], resource: [Team] };
action "manage-keys", "manage-templates", "manage-branding" appliesTo { principal: [Analyst, TenantKey], resource: [Tenant] };
```

`investigationOpenedBy` is a principal ID string (empty when no investigation was opened), compared against the principal's ID attribute, so add `id: String` to `Analyst`:

```
entity Analyst in [TeamRole] { id: String, leadOf: Set<Team>, tenantAdmin: Bool };
```

`policy/guardrails.cedar`:

```cedar
// Separation of duties: whoever opened the investigation never issues its final credit.
@id("sod-final-credit")
forbid (principal is Analyst, action == Action::"ISSUE_FINAL_CREDIT", resource is Dispute)
when { resource.investigationOpenedBy == principal.id };

// Only a lead of the dispute's team may move it to another team.
@id("reassign-leads-only")
forbid (principal, action == Action::"reassign", resource is Dispute)
unless { principal is Analyst && principal.leadOf.contains(resource.team) };

// Tenant administration is the realm role, never a grant.
@id("tenant-admin")
permit (principal is Analyst,
        action in [Action::"manage-keys", Action::"manage-templates", Action::"manage-branding"],
        resource is Tenant)
when { principal.tenantAdmin };

// Tenant keys keep today's reach until profiles arrive (spec P4): any dispute action and create.
@id("keys-disputes")
permit (principal is TenantKey, action, resource is Dispute);

@id("keys-create")
permit (principal is TenantKey, action == Action::"create", resource is Team);

// Composing, attaching and resending are for people.
@id("keys-not-people")
forbid (principal is TenantKey,
        action in [Action::"compose-email", Action::"upload-attachment", Action::"resend-notice", Action::"reassign"],
        resource);
```

- [ ] **Step 2: Write the failing engine tests**

`authz_test.go` (package `authz_test`); the cases are the ones the design probe confirmed:

```go
package authz_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/authz"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/principal"
)

var tenantID = uuid.MustParse("00000000-0000-8000-8000-00000000a001")

func access(members ...application.Membership) application.Access {
	limit := decimal.RequireFromString("500.00")
	return application.Access{
		Version: 1, DefaultTeam: "cb", Teams: []string{"cb", "fraud"},
		Grants: []application.Grant{
			{Team: "cb", Role: application.RoleJunior, Action: application.EventAction(domain.EventIssueFinalCredit), AmountLimit: &limit},
			{Team: "cb", Role: application.RoleSenior, Action: application.EventAction(domain.EventIssueFinalCredit)},
			{Team: "cb", Role: application.RoleJunior, Action: application.ActionCreate},
			{Team: "cb", Role: application.RoleLead, Action: application.ActionReassign},
		},
		Members: members,
	}
}

func analyst(id string) principal.Principal {
	return principal.Principal{Kind: principal.Analyst, ID: id, Display: id}
}

func dispute(amount, openedBy string) *application.DisputeFacts {
	return &application.DisputeFacts{ID: uuid.New(), Team: "cb", Amount: decimal.RequireFromString(amount),
		Currency: "EUR", Rail: domain.RailCard, State: domain.StateInvestigating, InvestigationOpenedBy: openedBy}
}

func TestDecide(t *testing.T) {
	e, err := authz.New(slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	junior := []application.Membership{{Team: "cb", Role: application.RoleJunior}}
	lead := []application.Membership{{Team: "cb", Role: application.RoleLead}}
	credit := application.EventAction(domain.EventIssueFinalCredit)
	cases := []struct {
		name   string
		p      principal.Principal
		a      application.Access
		action application.Action
		d      *application.DisputeFacts
		allow  bool
		policy string
	}{
		{"junior under the limit", analyst("j"), access(junior...), credit, dispute("100.00", "other"), true, "grant:cb/junior/ISSUE_FINAL_CREDIT"},
		{"junior over the limit", analyst("j"), access(junior...), credit, dispute("900.00", "other"), false, ""},
		{"lead inherits senior", analyst("l"), access(lead...), credit, dispute("900.00", "other"), true, "grant:cb/senior/ISSUE_FINAL_CREDIT"},
		{"investigator blocked", analyst("l"), access(lead...), credit, dispute("100.00", "l"), false, "sod-final-credit"},
		{"lead reassigns", analyst("l"), access(lead...), application.ActionReassign, dispute("1.00", ""), true, "grant:cb/lead/reassign"},
		{"junior cannot reassign", analyst("j"), access(junior...), application.ActionReassign, dispute("1.00", ""), false, "reassign-leads-only"},
		{"no membership", analyst("x"), access(), credit, dispute("1.00", ""), false, ""},
		{"key applies events", principal.Principal{Kind: principal.Key, ID: "k"}, access(), credit, dispute("1.00", ""), true, "keys-disputes"},
		{"key cannot compose", principal.Principal{Kind: principal.Key, ID: "k"}, access(), application.ActionComposeEmail, dispute("1.00", ""), false, "keys-not-people"},
		{"admin manages keys", principal.Principal{Kind: principal.Analyst, ID: "a", TenantAdmin: true}, access(), application.ActionManageKeys, nil, true, "tenant-admin"},
		{"non-admin cannot", analyst("a"), access(), application.ActionManageKeys, nil, false, ""},
		{"junior creates in own team", analyst("j"), access(junior...), application.ActionCreate, &application.DisputeFacts{Team: "cb"}, true, "grant:cb/junior/create"},
		{"junior cannot create in another team", analyst("j"), access(junior...), application.ActionCreate, &application.DisputeFacts{Team: "fraud"}, false, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := e.Decide(tenantID, c.p, c.a, c.action, c.d)
			if err != nil {
				t.Fatal(err)
			}
			if got.Allowed != c.allow {
				t.Fatalf("allowed = %v, policies %v", got.Allowed, got.Policies)
			}
			if c.policy != "" && (len(got.Policies) == 0 || got.Policies[0] != c.policy) {
				t.Fatalf("policies = %v, want %s first", got.Policies, c.policy)
			}
		})
	}
}
```

`compile_test.go` pins the compiled text for one row with and one without a limit:

```go
func TestCompile(t *testing.T) {
	src, skipped := authz.Compile(tenantID, access())
	if len(skipped) > 0 {
		t.Fatalf("skipped %v", skipped)
	}
	for _, want := range []string{
		`@id("grant:cb/junior/ISSUE_FINAL_CREDIT")`,
		`permit (principal in TeamRole::"cb/junior", action == Action::"ISSUE_FINAL_CREDIT", resource in Team::"cb")`,
		`when { resource.amount.lessThanOrEqual(decimal("500.0000")) };`,
		`@id("grant:cb/senior/ISSUE_FINAL_CREDIT")`,
	} {
		if !strings.Contains(src, want) {
			t.Errorf("compiled source lacks %q\n%s", want, src)
		}
	}
}

func TestCompileSkipsBadRows(t *testing.T) {
	a := access()
	a.Grants = append(a.Grants,
		application.Grant{Team: "cb", Role: "owner", Action: application.ActionCreate},
		application.Grant{Team: `cb") permit(principal,action,resource`, Role: application.RoleJunior, Action: application.ActionCreate})
	src, skipped := authz.Compile(tenantID, a)
	if len(skipped) != 2 || strings.Contains(src, "owner") || strings.Contains(src, "permit(principal,action,resource") {
		t.Fatalf("skipped %v\n%s", skipped, src)
	}
}
```

- [ ] **Step 3: Run and see them fail**

Run: `cd backend && mise exec -- go test ./internal/dispute/authz/`
Expected: compile failure, package does not exist.

- [ ] **Step 4: Implement the application types**

`application/access.go`: the types exactly as listed under Interfaces, plus:

```go
// EventAction is the action that applying e requires.
func EventAction(e domain.Event) Action { return Action(e) }

// ErrForbidden is a caller whose grants or the guardrails refuse the operation.
var ErrForbidden = errs.New(errs.Forbidden, "forbidden", "your role does not allow this operation")

// Roles is the ladder, lowest first.
var Roles = []Role{RoleJunior, RoleSenior, RoleLead}
```

- [ ] **Step 5: Implement compile.go**

```go
package authz

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
)

// slugs and actions reach Cedar source as string literals; anything else is refused rather than escaped
var safe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// Compile turns a tenant's grants into Cedar permits, one per row, each resource-bounded to its team. Rows it cannot
// express safely are skipped and returned, never escaped.
func Compile(_ uuid.UUID, a application.Access) (string, []string) {
	var b strings.Builder
	var skipped []string
	for _, g := range a.Grants {
		if !slices.Contains(application.Roles, g.Role) || !safe.MatchString(g.Team) || !safe.MatchString(string(g.Action)) {
			skipped = append(skipped, fmt.Sprintf("%q/%q/%q", g.Team, g.Role, g.Action))
			continue
		}
		resource := fmt.Sprintf(`resource in Team::"%s"`, g.Team)
		fmt.Fprintf(&b, "@id(\"grant:%s/%s/%s\")\n", g.Team, g.Role, g.Action)
		fmt.Fprintf(&b, "permit (principal in TeamRole::\"%s/%s\", action == Action::\"%s\", %s)", g.Team, g.Role, g.Action, resource)
		if g.AmountLimit != nil {
			fmt.Fprintf(&b, "\nwhen { resource.amount.lessThanOrEqual(decimal(\"%s\")) }", g.AmountLimit.StringFixed(4))
		}
		b.WriteString(";\n\n")
	}
	return b.String(), skipped
}
```

Note: a `create` grant's resource is the `Team` itself; `Team::"cb" in Team::"cb"` holds because `in` is reflexive, so the same `resource in Team` form covers it (the probe confirmed `in` semantics for hierarchy).

- [ ] **Step 6: Implement entities.go**

```go
package authz

import (
	"github.com/cedar-policy/cedar-go"
	"github.com/cedar-policy/cedar-go/types"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/principal"
)

func uid(typ, id string) cedar.EntityUID { return cedar.NewEntityUID(cedar.EntityType(typ), cedar.String(id)) }

func teamRole(team string, r application.Role) cedar.EntityUID { return uid("TeamRole", team+"/"+string(r)) }

// entities builds the request's entity store: the ladder for every team, the caller, and the resource.
func entities(p principal.Principal, a application.Access, d *application.DisputeFacts, tenant cedar.EntityUID) (cedar.EntityMap, cedar.EntityUID, error) {
	m := cedar.EntityMap{}
	for _, t := range a.Teams {
		m[teamRole(t, application.RoleLead)] = cedar.Entity{UID: teamRole(t, application.RoleLead), Parents: cedar.NewEntityUIDSet(teamRole(t, application.RoleSenior))}
		m[teamRole(t, application.RoleSenior)] = cedar.Entity{UID: teamRole(t, application.RoleSenior), Parents: cedar.NewEntityUIDSet(teamRole(t, application.RoleJunior))}
		m[teamRole(t, application.RoleJunior)] = cedar.Entity{UID: teamRole(t, application.RoleJunior)}
		m[uid("Team", t)] = cedar.Entity{UID: uid("Team", t)}
	}
	var who cedar.EntityUID
	switch p.Kind {
	case principal.Analyst:
		who = uid("Analyst", p.ID)
		var parents []cedar.EntityUID
		var leadOf []types.Value
		for _, mem := range a.Members {
			parents = append(parents, teamRole(mem.Team, mem.Role))
			if mem.Role == application.RoleLead {
				leadOf = append(leadOf, uid("Team", mem.Team))
			}
		}
		m[who] = cedar.Entity{UID: who, Parents: cedar.NewEntityUIDSet(parents...), Attributes: cedar.NewRecord(cedar.RecordMap{
			"id": cedar.String(p.ID), "leadOf": cedar.NewSet(leadOf...), "tenantAdmin": cedar.Boolean(p.TenantAdmin)})}
	case principal.Key:
		who = uid("TenantKey", p.ID)
		m[who] = cedar.Entity{UID: who, Attributes: cedar.NewRecord(cedar.RecordMap{"profile": cedar.String("")})}
	}
	m[tenant] = cedar.Entity{UID: tenant}
	if d == nil {
		return m, tenant, nil
	}
	if d.ID == [16]byte{} {
		return m, uid("Team", d.Team), nil
	}
	amount, err := types.ParseDecimal(d.Amount.StringFixed(4))
	if err != nil {
		return nil, cedar.EntityUID{}, err
	}
	res := uid("Dispute", d.ID.String())
	m[res] = cedar.Entity{UID: res, Parents: cedar.NewEntityUIDSet(uid("Team", d.Team)), Attributes: cedar.NewRecord(cedar.RecordMap{
		"team": uid("Team", d.Team), "amount": amount, "currency": cedar.String(d.Currency), "rail": cedar.String(string(d.Rail)),
		"state": cedar.String(string(d.State)), "investigationOpenedBy": cedar.String(d.InvestigationOpenedBy)})}
	return m, res, nil
}
```

If `cedar.Boolean` is not re-exported, use `types.Boolean`; confirm with `mise exec -- go doc github.com/cedar-policy/cedar-go Boolean`.

- [ ] **Step 7: Implement authz.go**

```go
// Package authz decides whether a principal may act on a dispute, team or tenant. Guardrails are embedded Cedar no
// tenant can change; grants are a tenant's rows compiled to permits. Any forbid overrides every permit (ADR 0026).
package authz

import (
	"embed"
	"fmt"
	"log/slog"
	"sync"

	"github.com/cedar-policy/cedar-go"
	"github.com/google/uuid"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/principal"
)

//go:embed policy/guardrails.cedar
var policies embed.FS

// Engine caches each tenant's compiled policy set by policy version.
type Engine struct {
	log        *slog.Logger
	guardrails cedar.PolicyList
	mu         sync.Mutex
	sets       map[uuid.UUID]cached
}

type cached struct {
	version int64
	set     *cedar.PolicySet
}

var _ application.Authorizer = (*Engine)(nil)

// New parses the guardrails; a parse failure is a build defect, so the API refuses to start.
func New(log *slog.Logger) (*Engine, error) {
	src, err := policies.ReadFile("policy/guardrails.cedar")
	if err != nil {
		return nil, err
	}
	list, err := cedar.NewPolicyListFromBytes("guardrails.cedar", src)
	if err != nil {
		return nil, fmt.Errorf("authz: guardrails: %w", err)
	}
	for _, p := range list {
		if _, ok := p.Annotations()["id"]; !ok {
			return nil, fmt.Errorf("authz: a guardrail has no @id")
		}
	}
	return &Engine{log: log, guardrails: list, sets: map[uuid.UUID]cached{}}, nil
}

func (e *Engine) set(tenantID uuid.UUID, a application.Access) (*cedar.PolicySet, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if c, ok := e.sets[tenantID]; ok && c.version == a.Version {
		return c.set, nil
	}
	src, skipped := Compile(tenantID, a)
	if len(skipped) > 0 {
		e.log.Warn("authz: grants skipped", "tenant", tenantID, "version", a.Version, "grants", skipped)
	}
	grants, err := cedar.NewPolicyListFromBytes("grants.cedar", []byte(src))
	if err != nil {
		return nil, fmt.Errorf("authz: compiled grants: %w", err)
	}
	ps := cedar.NewPolicySet()
	for _, p := range append(append(cedar.PolicyList{}, e.guardrails...), grants...) {
		ps.Add(cedar.PolicyID(p.Annotations()["id"]), p)
	}
	e.sets[tenantID] = cached{version: a.Version, set: ps}
	return ps, nil
}

// Decide implements application.Authorizer. Errors from evaluation deny; the caller records them.
func (e *Engine) Decide(tenantID uuid.UUID, p principal.Principal, a application.Access, action application.Action, d *application.DisputeFacts) (application.Decision, error) {
	ps, err := e.set(tenantID, a)
	if err != nil {
		return application.Decision{}, err
	}
	ents, resource, err := entities(p, a, d, uid("Tenant", tenantID.String()))
	if err != nil {
		return application.Decision{}, err
	}
	who := uid("Analyst", p.ID)
	if p.Kind == principal.Key {
		who = uid("TenantKey", p.ID)
	}
	decision, diag := cedar.Authorize(ps, ents, cedar.Request{Principal: who, Action: uid("Action", string(action)), Resource: resource, Context: cedar.NewRecord(nil)})
	out := application.Decision{Allowed: decision == cedar.Allow}
	for _, r := range diag.Reasons {
		out.Policies = append(out.Policies, string(r.PolicyID))
	}
	if len(diag.Errors) > 0 {
		return application.Decision{Policies: out.Policies}, fmt.Errorf("authz: evaluation: %v", diag.Errors)
	}
	return out, nil
}
```

Embedding the schema file is not needed at runtime; it is read by the CI validation in Task 12.

- [ ] **Step 8: Run and see the tests pass**

Run: `cd backend && mise exec -- go get github.com/cedar-policy/cedar-go@v1.8.0 && mise exec -- go test ./internal/dispute/authz/`
Expected: PASS for every case in `TestDecide` and both compile tests.

- [ ] **Step 9: Commit**

```bash
git add backend
git commit -m "feat(authz): a Cedar engine with embedded guardrails and grants compiled from rows"
```

### Task 4: The service authorizes every dispute operation

**Files:**
- Modify: `backend/internal/dispute/application/service.go` (Option, NewService, `authorize`, CreateDispute, ApplyEvent, view)
- Modify: `backend/internal/dispute/application/compose.go` (ComposeEmail, Resend, template settings), `attachments.go` (Upload), `identity.go` (PutTenantLogo)
- Modify: `backend/internal/dispute/application/ports.go` (`Tx.Access`)
- Create: `backend/internal/dispute/application/default_access.go`
- Modify: `backend/internal/dispute/infrastructure/postgres/store.go` and `apptest/memstore.go` (`Access`)
- Modify: `backend/internal/dispute/application/apptest/memstore.go` (`Ctx`, `CtxFor`, `NewService` inject the engine)
- Modify: every test calling `application.NewService(` or building a context with only `tenant.WithID(` (`grep -rln "application.NewService(\|tenant.WithID(" backend --include=*_test.go`)
- Modify: `backend/cmd/api/main.go:91`, `backend/cmd/eval` (any `NewService` call)
- Modify: `backend/internal/dispute/ports/http/handler.go` (remove `tenantAdmin`'s and the other `RequireRole` calls)
- Test: `backend/internal/dispute/application/authz_test.go`

**Interfaces:**
- Consumes: `application.Authorizer`, `authz.New`, `principal.From`.
- Produces: `application.WithAuthorizer(a Authorizer) Option`; `NewService` returns an error when no authorizer is given; `Tx.Access(ctx context.Context, p principal.Principal) (Access, error)`; `application.DefaultAccess(p principal.Principal) Access`; `apptest.Ctx()` carries a lead analyst `apptest.Lead` (`ID "analyst-lead"`, `TenantAdmin true`); `apptest.CtxFor(tenantID uuid.UUID) context.Context`.

- [ ] **Step 1: Write the failing tests**

`application/authz_test.go`:

```go
package application_test

func TestNoPrincipalIsRefused(t *testing.T) {
	store := apptest.NewMemStore()
	svc := apptest.NewService(t, store)
	txn := store.AddTransaction(domain.RailCard, "EUR", "EUR", "40.00")
	_, err := svc.CreateDispute(tenant.WithID(context.Background(), apptest.TenantA), application.CreateDisputeInput{TransactionID: txn})
	if !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("err = %v", err)
	}
}

func TestSeparationOfDuties(t *testing.T) {
	store := apptest.NewMemStore()
	svc := apptest.NewService(t, store)
	txn := store.AddTransaction(domain.RailCard, "EUR", "EUR", "40.00")
	ctx := apptest.Ctx()
	created, err := svc.CreateDispute(ctx, application.CreateDisputeInput{TransactionID: txn})
	if err != nil {
		t.Fatal(err)
	}
	walkToFinalCredit(t, svc, ctx, created.View.ID) // helper: OPEN_INVESTIGATION and the steps the card regime needs before ISSUE_FINAL_CREDIT
	_, err = svc.ApplyEvent(ctx, application.ApplyEventInput{DisputeID: created.View.ID, Event: domain.EventIssueFinalCredit})
	if !errors.Is(err, application.ErrForbidden) || !strings.Contains(err.Error(), "sod-final-credit") {
		t.Fatalf("investigator issued final credit: %v", err)
	}
	other := apptest.CtxAs(principal.Principal{Kind: principal.Analyst, ID: "analyst-two", Display: "two@x"})
	if _, err := svc.ApplyEvent(other, application.ApplyEventInput{DisputeID: created.View.ID, Event: domain.EventIssueFinalCredit}); err != nil {
		t.Fatalf("second analyst: %v", err)
	}
}

func TestAllowedEventsExcludeForbidden(t *testing.T) {
	// same setup as TestSeparationOfDuties up to the final-credit state; the investigator's view omits ISSUE_FINAL_CREDIT
	view, err := svc.GetDispute(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(view.AllowedEvents, domain.EventIssueFinalCredit) {
		t.Fatalf("allowed = %v", view.AllowedEvents)
	}
}
```

For `walkToFinalCredit`, copy the event sequence an existing test already uses to reach a state where `ISSUE_FINAL_CREDIT` is allowed (`grep -rn EventIssueFinalCredit backend/internal/dispute/application/*_test.go`).

- [ ] **Step 2: Run and see them fail**

Run: `cd backend && mise exec -- go test ./internal/dispute/application/ -run 'NoPrincipal|SeparationOfDuties|AllowedEvents'`
Expected: FAIL (no authorization yet).

- [ ] **Step 3: DefaultAccess keeps today's behaviour**

`application/default_access.go`:

```go
package application

import (
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/principal"
)

// DefaultTeam is every tenant's team until its own teams are seeded (spec P2).
const DefaultTeam = "general"

// DefaultAccess grants every dispute action to every analyst as a lead of the one default team, so enabling
// authorization changes nothing but the guardrails.
func DefaultAccess(p principal.Principal) Access {
	grants := []Grant{
		{Team: DefaultTeam, Role: RoleJunior, Action: ActionCreate},
		{Team: DefaultTeam, Role: RoleJunior, Action: ActionComposeEmail},
		{Team: DefaultTeam, Role: RoleJunior, Action: ActionUploadAttachment},
		{Team: DefaultTeam, Role: RoleJunior, Action: ActionResendNotice},
		{Team: DefaultTeam, Role: RoleLead, Action: ActionReassign},
	}
	for _, e := range domain.AllEvents() {
		grants = append(grants, Grant{Team: DefaultTeam, Role: RoleJunior, Action: EventAction(e)})
	}
	var members []Membership
	if p.Kind == principal.Analyst {
		members = []Membership{{Team: DefaultTeam, Role: RoleLead}}
	}
	return Access{Version: 0, DefaultTeam: DefaultTeam, Teams: []string{DefaultTeam}, Grants: grants, Members: members}
}
```

If `domain.AllEvents()` does not exist, add it next to the event constants in `domain/state.go` returning the 13 events in declaration order, with a test asserting its length is 13.

Add to the `Tx` interface in `ports.go`:

```go
	// Access is the caller's teams, grants and routing, read in this transaction.
	Access(ctx context.Context, p principal.Principal) (Access, error)
```

and implement it in both `store.go`'s `txn` and `memTx` as `return application.DefaultAccess(p), nil` (PR B replaces both).

- [ ] **Step 4: The service hook**

In `service.go`: add the fields `authz Authorizer` and `decisions metric.Int64Counter` (created in `NewService` next to `transitions`, named `authz.decisions`, unit `{decision}`), `func WithAuthorizer(a Authorizer) Option { return func(s *Service) { s.authz = a } }`, and at the end of `NewService` return `errors.New("application: an Authorizer is required")` when `s.authz == nil`. Then:

```go
// authorize decides action for the caller in tx; d nil is the tenant, d with a zero ID is the team a create routes to.
func (s *Service) authorize(ctx context.Context, tx Tx, action Action, d *DisputeFacts) (Decision, error) {
	p, ok := principal.From(ctx)
	if !ok {
		return Decision{}, ErrForbidden
	}
	tid, _ := tenant.IDFrom(ctx)
	a, err := tx.Access(ctx, p)
	if err != nil {
		return Decision{}, err
	}
	dec, err := s.authz.Decide(tid, p, a, action, d)
	s.decisions.Add(ctx, 1, metric.WithAttributes(tenantAttr(ctx), attribute.String("action", string(action)), attribute.Bool("allowed", dec.Allowed && err == nil)))
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attribute.String("authz.action", string(action)), attribute.Bool("authz.allowed", dec.Allowed),
		attribute.StringSlice("authz.policies", dec.Policies), attribute.Int64("authz.policy_version", a.Version))
	if err != nil {
		span.RecordError(err)
		return Decision{}, ErrForbidden
	}
	if !dec.Allowed {
		if len(dec.Policies) > 0 {
			return dec, errs.Wrap(ErrForbidden, "denied by %s", dec.Policies[0])
		}
		return dec, errs.Wrap(ErrForbidden, "no grant allows %s", action)
	}
	return dec, nil
}

// facts is what policies may read about a dispute, including who opened its investigation.
func facts(ctx context.Context, tx Tx, rec DisputeRecord) (DisputeFacts, error) {
	events, err := tx.ListEvents(ctx, rec.ID)
	if err != nil {
		return DisputeFacts{}, err
	}
	var opener string
	for _, e := range events {
		if e.Event == domain.EventOpenInvestigation {
			opener = e.ActorID
		}
	}
	return DisputeFacts{ID: rec.ID, Team: rec.Team, Amount: rec.DisputedAmount, Currency: rec.Currency,
		Rail: rec.Regime.Rail(), State: rec.State, InvestigationOpenedBy: opener}, nil
}
```

`DisputeRecord` gains `Team string`; in PR A both stores set it to `application.DefaultTeam` on read (memstore: when empty; postgres: a constant until the column exists in Task 8). If `Regime` has no `Rail()` accessor, pass the transaction's rail: `CreateDispute` already has `txn.Rail`; in `ApplyEvent` load it with `tx.GetTransaction(ctx, rec.TransactionID)` only if a grant needs it; for PR A use `domain.Rail("")` and add `Rail` to `DisputeRecord` in Task 8 where routing needs it.

Call sites (each inside the existing `WithTx` closure, right after the record is loaded):

- `CreateDispute`, before `InsertDispute`: `if _, err := s.authorize(ctx, tx, ActionCreate, &DisputeFacts{Team: DefaultTeam}); err != nil { return DisputeView{}, err }` (Task 9 replaces `DefaultTeam` with the routed team).
- `ApplyEvent`, after `GetDispute` and before `Lifecycle().Apply`: build `f, err := facts(ctx, tx, rec)`, then `dec, err := s.authorize(ctx, tx, EventAction(in.Event), &f)`; after the payload default, record the deciding policies: `payload, err = withAuthorizedBy(payload, dec.Policies)`.
- `ComposeEmail`, `Resend`, `Upload`: the same with `ActionComposeEmail`, `ActionResendNotice`, `ActionUploadAttachment` on the dispute's facts.
- `PutTemplateSetting`, `DeleteTemplateSetting`, `ListTemplateSettings`: `ActionManageTemplates` with `nil`. `PutTenantLogo`: `ActionManageBranding` with `nil`.

```go
// withAuthorizedBy records which policies allowed the event, so the log answers "why was this permitted".
func withAuthorizedBy(payload json.RawMessage, policies []string) (json.RawMessage, error) {
	m := map[string]any{}
	if err := json.Unmarshal(payload, &m); err != nil {
		return nil, err
	}
	m["authorizedBy"] = policies
	return json.Marshal(m)
}
```

`view` narrows `allowed`:

```go
	allowed := []domain.Event{}
	f, err := facts(ctx, tx, rec)
	if err != nil {
		return DisputeView{}, err
	}
	for _, e := range rec.Lifecycle().Allowed() {
		if _, err := s.authorize(ctx, tx, EventAction(e), &f); err == nil {
			allowed = append(allowed, e)
		}
	}
```

`authorize` is called once per candidate event; the compiled set is cached, so this is entity construction plus evaluation, measured in Task 12.

- [ ] **Step 5: Tenant keys self-service moves off RequireRole**

The key operations (`ListTenantKeys`, `CreateTenantKey`, `RevokeTenantKey`) live in the handler and never enter the service. Add a service method and call it from `tenantAdmin` instead of `auth.RequireRole`:

```go
// Authorize checks a tenant-level action outside a dispute transaction, for handlers that own their storage.
func (s *Service) Authorize(ctx context.Context, action Action) error {
	return s.store.WithTx(ctx, func(tx Tx) error {
		_, err := s.authorize(ctx, tx, action, nil)
		return err
	})
}
```

In `handler.go`: `tenantAdmin` calls `h.svc.Authorize(ctx, application.ActionManageKeys)`; delete the `RequireRole` lines in `ListTenantTemplates`, `PutTenantTemplate`, `DeleteTenantTemplate`, `PutTenantLogo` (the service now checks). Keep `RequireRole` exported only if something else still uses it; otherwise delete it and `ErrForbidden` from `auth`.

- [ ] **Step 6: Test helpers and the migration of existing tests**

In `apptest/memstore.go`:

```go
// Lead is the analyst test contexts act as: a lead of every team and a tenant admin.
var Lead = principal.Principal{Kind: principal.Analyst, ID: "analyst-lead", Display: "lead@example.test", TenantAdmin: true}

func Ctx() context.Context { return CtxFor(TenantA) }

// CtxFor is a tenant context acting as Lead.
func CtxFor(tenantID uuid.UUID) context.Context {
	return principal.With(tenant.WithID(context.Background(), tenantID), Lead)
}
```

`apptest.NewService` appends `application.WithAuthorizer(engine)` where `engine, err := authz.New(slog.Default())` (fatal on error). Replace `application.NewService(` in `_test.go` files with `apptest.NewService(t, ` (drop the `err` handling), and `tenant.WithID(context.Background(), X)` with `apptest.CtxFor(X)`, except in `TestNoPrincipalIsRefused`. Wire `cmd/api/main.go` and `cmd/eval`: `engine, err := authz.New(logger)` with the process logger; pass `application.WithAuthorizer(engine)`.

- [ ] **Step 7: Run everything**

Run: `make api:test && make api:test-integration`
Expected: PASS, including the three new tests.

- [ ] **Step 8: Commit**

```bash
git add backend
git commit -m "feat(disputes): every dispute and tenant operation is decided by the authorization engine"
```

### Task 5: Boot refuses an operation without an authorization class

**Files:**
- Create: `backend/internal/dispute/ports/http/operations.go`
- Modify: `backend/internal/dispute/ports/http/handler.go` (`Mount` calls the check)
- Test: `backend/internal/dispute/ports/http/operations_test.go`

**Interfaces:**
- Produces: `var operationClass map[string]opClass` keyed by OpenAPI `operationId`; `func checkCoverage(spec *openapi3.T) error`.

- [ ] **Step 1: Write the failing test**

```go
func TestEveryOperationIsClassified(t *testing.T) {
	spec, err := oapi.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	if err := checkCoverage(spec); err != nil {
		t.Fatal(err)
	}
}

func TestUnclassifiedOperationFails(t *testing.T) {
	spec, _ := oapi.GetSpec()
	spec.Paths.Find("/healthz").Get.OperationID = "brandNew"
	if err := checkCoverage(spec); err == nil || !strings.Contains(err.Error(), "brandNew") {
		t.Fatalf("err = %v", err)
	}
}
```

(This test file is `package disputehttp`, internal, to reach `checkCoverage`; check the package name at the top of `handler.go`.)

- [ ] **Step 2: Run and see it fail**

Run: `cd backend && mise exec -- go test ./internal/dispute/ports/http/ -run Operation`
Expected: compile failure.

- [ ] **Step 3: Implement**

```go
package disputehttp

import (
	"fmt"
	"sort"

	"github.com/getkin/kin-openapi/openapi3"
)

type opClass string

const (
	public   opClass = "public"   // no credential
	internal opClass = "internal" // service key, internal network only
	tenantWide opClass = "tenant" // any authenticated caller; rows are scoped by RLS
	decided  opClass = "decided"  // the service asks the authorization engine
)

// operationClass says how each operation is authorized; Mount refuses to start if the spec grows one it lacks.
var operationClass = map[string]opClass{
	"getHealthz": public, "getReadyz": public,
	"listDisputes": tenantWide, "getDispute": tenantWide, "getAttachment": tenantWide, "getNotice": tenantWide,
	"listEmailTemplates": tenantWide, "getTenant": tenantWide, "getTenantLogo": tenantWide,
	"getMyAvatar": tenantWide, "putMyAvatar": tenantWide,
	"suggestQuestionnaireAnswers": tenantWide, "suggestSearchFilters": tenantWide, "suggestDisputeReason": tenantWide,
	"createDispute": decided, "applyDisputeEvent": decided, "composeEmail": decided, "uploadAttachment": decided,
	"resendNotice": decided, "listTenantTemplates": decided, "putTenantTemplate": decided,
	"deleteTenantTemplate": decided, "putTenantLogo": decided,
	"listTenantKeys": decided, "createTenantKey": decided, "revokeTenantKey": decided,
	"putSession": internal, "getSession": internal, "deleteSession": internal, "deleteSessions": internal, "listTenants": internal,
}

func checkCoverage(spec *openapi3.T) error {
	var missing []string
	for _, item := range spec.Paths.Map() {
		for _, op := range item.Operations() {
			if _, ok := operationClass[op.OperationID]; !ok {
				missing = append(missing, op.OperationID)
			}
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("disputehttp: operations without an authorization class: %v", missing)
	}
	return nil
}
```

Take the exact operation IDs from `docs/api/openapi.yaml` (`grep -n operationId docs/api/openapi.yaml`); the list above follows the handler names and must match the spec, not the other way round. In `Mount`, after `spec.Servers = nil`: `if err := checkCoverage(spec); err != nil { return err }`.

- [ ] **Step 4: Run and see it pass**

Run: `cd backend && mise exec -- go test ./internal/dispute/ports/http/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend
git commit -m "feat(api): refuse to start when an operation has no authorization class"
```

### Task 6: ADR 0026, thesis section, suites, PR A

**Files:**
- Create: `docs/adr/0026-authorization-with-cedar.md`
- Modify: `docs/thesis/latex/chapters/04-design.tex` (new section after the multi-tenancy stub at line 249)
- Modify: `docs/authentication.md:52-54` (the authorization paragraph)
- Modify: eval and browser suites that apply `ISSUE_FINAL_CREDIT` as the investigator (`grep -rn "ISSUE_FINAL_CREDIT" eval frontend/e2e backend/cmd/eval`)

- [ ] **Step 1: Fix suites hit by separation of duties**

For each hit, make the final credit come from a different principal than the investigation: a tenant key where the suite already has one, otherwise a second seeded analyst. Do not change the guardrail. Run `make thesis:eval` if the eval harness is affected, and `make workbench:e2e` for the browser suite (informative in CI, run locally).

- [ ] **Step 2: Write ADR 0026**

Follow the format of `docs/adr/0025-validation-behind-standard-schema.md` (Status, Context, Decision, Consequences). Content: why Cedar (formal semantics, deny by default, forbid overrides permit), rules in code and grants in data, the principal for both credential kinds, actor from the principal, fail-closed behaviour, the coverage check, and that PR A keeps behaviour with `DefaultAccess` apart from separation of duties.

- [ ] **Step 3: Write the thesis section**

In `04-design.tex`, a `\section{Authorization}` of four to six paragraphs covering the same points as the ADR in thesis prose, citing ADR 0026. Run `grep -i supervisor docs/thesis` and `grep -nP '\x{2014}' docs/thesis/latex/chapters/04-design.tex`; both must print nothing.

- [ ] **Step 4: Full verification**

Run: `make ci && make api:test-integration`
Expected: PASS.

- [ ] **Step 5: Commit and open PR A**

```bash
git add docs eval frontend backend
git commit -m "docs(authz): ADR 0026 and the authorization section"
git push -u origin HEAD
scripts/create-pr --title "feat(authz): Cedar authorization for every operation, actor from the caller" \
  --description "Every operation that changes a dispute or the tenant is now decided by one policy engine, with fixed guardrails such as separation of duties that no grant can override. The event actor is the authenticated caller rather than a request field, which the guardrails depend on." \
  --adr "0026"
```

Merge with `scripts/merge-when-green <number>` after review.

---

## PR B: teams and grants in data (spec P2)

Create the branch from `main` after PR A merges: `authz/teams-and-grants`.

### Task 7: Access tables, policy versions, disputes.team

**Files:**
- Create: `backend/migrations/0023_access.sql`
- Create: `backend/internal/dispute/infrastructure/postgres/queries/access.sql`
- Modify: `backend/internal/dispute/infrastructure/postgres/store.go` (`txn.Access`, `Team` on dispute reads)
- Test: `backend/internal/dispute/infrastructure/postgres/access_test.go` (integration tag)

**Interfaces:**
- Consumes: `application.Access`, `application.Grant`, `application.Membership`, `application.RoutingRule`.
- Produces: postgres `txn.Access(ctx, p)` reading the tables; SQL functions `current_subject()`, `current_principal_kind()`.

- [ ] **Step 1: Write the failing integration test**

```go
//go:build integration

func TestAccessReadsGrantsAndBumpsVersion(t *testing.T) {
	owner, schema := pgtest.PoolWithSchema(t)
	app := pgtest.AppPool(t, schema)
	store := disputepg.NewStore(app)
	ctx := apptest.CtxFor(apptest.TenantA)
	mustExec(t, owner, `INSERT INTO teams (tenant_id, slug, name) VALUES ($1, 'cb', 'Chargebacks')`, apptest.TenantA)
	mustExec(t, owner, `INSERT INTO team_members (tenant_id, team, subject, role) VALUES ($1, 'cb', $2, 'senior')`, apptest.TenantA, apptest.Lead.ID)
	mustExec(t, owner, `INSERT INTO role_grants (tenant_id, team, role, action, amount_limit) VALUES ($1, 'cb', 'junior', 'ISSUE_FINAL_CREDIT', 500)`, apptest.TenantA)

	var before application.Access
	if err := store.WithTx(ctx, func(tx application.Tx) (err error) { before, err = tx.Access(ctx, apptest.Lead); return }); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(before.Teams, "cb") || !slices.Contains(before.Teams, "general") || len(before.Members) != 1 || before.Members[0] != (application.Membership{Team: "cb", Role: application.RoleSenior}) {
		t.Fatalf("access = %+v", before)
	}
	mustExec(t, owner, `DELETE FROM role_grants WHERE tenant_id = $1 AND team = 'cb'`, apptest.TenantA)
	var after application.Access
	_ = store.WithTx(ctx, func(tx application.Tx) (err error) { after, err = tx.Access(ctx, apptest.Lead); return })
	if after.Version <= before.Version {
		t.Fatalf("version %d -> %d", before.Version, after.Version)
	}
}
```

Add `mustExec(t, pool, sql, args...)` to the test file if no such helper exists in the package (check `seedFor` in `tenancy_test.go` for the style).

- [ ] **Step 2: Run and see it fail**

Run: `make api:test-integration`
Expected: FAIL, relation `teams` does not exist.

- [ ] **Step 3: Migration**

`backend/migrations/0023_access.sql`:

```sql
-- Who may do what inside a tenant is data, so a tenant admin can later edit it; the rules no tenant may change are
-- Cedar in the binary (ADR 0027). Every tenant starts with one default team that owns all existing disputes.
CREATE TABLE teams (
    tenant_id  uuid NOT NULL REFERENCES tenants (id) DEFAULT current_tenant_id(),
    slug       text NOT NULL CHECK (slug ~ '^[a-z][a-z0-9-]{0,31}$'),
    name       text NOT NULL,
    is_default boolean NOT NULL DEFAULT false,
    PRIMARY KEY (tenant_id, slug)
);
CREATE UNIQUE INDEX teams_one_default ON teams (tenant_id) WHERE is_default;

CREATE TABLE team_members (
    tenant_id uuid NOT NULL DEFAULT current_tenant_id(),
    team      text NOT NULL,
    subject   text NOT NULL,
    role      text NOT NULL CHECK (role IN ('junior', 'senior', 'lead')),
    PRIMARY KEY (tenant_id, team, subject),
    FOREIGN KEY (tenant_id, team) REFERENCES teams (tenant_id, slug) ON DELETE CASCADE
);
CREATE INDEX team_members_subject_idx ON team_members (tenant_id, subject);

CREATE TABLE role_grants (
    tenant_id    uuid NOT NULL DEFAULT current_tenant_id(),
    team         text NOT NULL,
    role         text NOT NULL CHECK (role IN ('junior', 'senior', 'lead')),
    action       text NOT NULL CHECK (action ~ '^[A-Za-z0-9_-]{1,64}$'),
    amount_limit numeric(19, 4) CHECK (amount_limit IS NULL OR amount_limit >= 0),
    PRIMARY KEY (tenant_id, team, role, action),
    FOREIGN KEY (tenant_id, team) REFERENCES teams (tenant_id, slug) ON DELETE CASCADE
);

CREATE TABLE routing_rules (
    tenant_id  uuid NOT NULL DEFAULT current_tenant_id(),
    position   int NOT NULL,
    rail       text,
    reason     text,
    risk_tier  text,
    min_amount numeric(19, 4),
    team       text NOT NULL,
    PRIMARY KEY (tenant_id, position),
    FOREIGN KEY (tenant_id, team) REFERENCES teams (tenant_id, slug) ON DELETE CASCADE
);

CREATE TABLE policy_versions (
    tenant_id uuid PRIMARY KEY REFERENCES tenants (id) DEFAULT current_tenant_id(),
    version   bigint NOT NULL DEFAULT 1
);

-- any change to a tenant's access data invalidates its compiled policy set on the next request
CREATE FUNCTION bump_policy_version() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE t uuid := COALESCE(NEW.tenant_id, OLD.tenant_id);
BEGIN
    INSERT INTO policy_versions (tenant_id, version) VALUES (t, 1)
    ON CONFLICT (tenant_id) DO UPDATE SET version = policy_versions.version + 1;
    RETURN NULL;
END $$;
CREATE TRIGGER teams_version AFTER INSERT OR UPDATE OR DELETE ON teams FOR EACH ROW EXECUTE FUNCTION bump_policy_version();
CREATE TRIGGER team_members_version AFTER INSERT OR UPDATE OR DELETE ON team_members FOR EACH ROW EXECUTE FUNCTION bump_policy_version();
CREATE TRIGGER role_grants_version AFTER INSERT OR UPDATE OR DELETE ON role_grants FOR EACH ROW EXECUTE FUNCTION bump_policy_version();
CREATE TRIGGER routing_rules_version AFTER INSERT OR UPDATE OR DELETE ON routing_rules FOR EACH ROW EXECUTE FUNCTION bump_policy_version();

-- every tenant, existing and future, has the default team and today's grants on it
CREATE FUNCTION default_access(t uuid) RETURNS void LANGUAGE sql AS $$
    INSERT INTO teams (tenant_id, slug, name, is_default) VALUES (t, 'general', 'General', true);
    INSERT INTO role_grants (tenant_id, team, role, action)
    SELECT t, 'general', 'junior', a FROM unnest(ARRAY[
        'create', 'compose-email', 'upload-attachment', 'resend-notice',
        'OPEN_INVESTIGATION', 'SEND_QUESTIONNAIRE', 'RECEIVE_QUESTIONNAIRE', 'ISSUE_REFUND', 'FILE_CHARGEBACK',
        'ACKNOWLEDGE_CHARGEBACK', 'SUBMIT_EVIDENCE', 'WIN_CHARGEBACK', 'LOSE_CHARGEBACK', 'ISSUE_FINAL_CREDIT',
        'REVERSE_PROVISIONAL_CREDIT', 'CLOSE', 'APPEAL']) AS a;
    INSERT INTO role_grants (tenant_id, team, role, action) VALUES (t, 'general', 'lead', 'reassign');
$$;
SELECT default_access(id) FROM tenants;
CREATE FUNCTION tenants_default_access() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN PERFORM default_access(NEW.id); RETURN NULL; END $$;
CREATE TRIGGER tenants_default_access AFTER INSERT ON tenants FOR EACH ROW EXECUTE FUNCTION tenants_default_access();

ALTER TABLE disputes ADD COLUMN team text;
UPDATE disputes SET team = 'general';
ALTER TABLE disputes ALTER COLUMN team SET NOT NULL;
ALTER TABLE disputes ADD FOREIGN KEY (tenant_id, team) REFERENCES teams (tenant_id, slug);
CREATE INDEX disputes_team_idx ON disputes (tenant_id, team, opened_at DESC);

ALTER TABLE teams           ENABLE ROW LEVEL SECURITY;
ALTER TABLE team_members    ENABLE ROW LEVEL SECURITY;
ALTER TABLE role_grants     ENABLE ROW LEVEL SECURITY;
ALTER TABLE routing_rules   ENABLE ROW LEVEL SECURITY;
ALTER TABLE policy_versions ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON teams           USING (tenant_id = current_tenant_id()) WITH CHECK (tenant_id = current_tenant_id());
CREATE POLICY tenant_isolation ON team_members    USING (tenant_id = current_tenant_id()) WITH CHECK (tenant_id = current_tenant_id());
CREATE POLICY tenant_isolation ON role_grants     USING (tenant_id = current_tenant_id()) WITH CHECK (tenant_id = current_tenant_id());
CREATE POLICY tenant_isolation ON routing_rules   USING (tenant_id = current_tenant_id()) WITH CHECK (tenant_id = current_tenant_id());
CREATE POLICY tenant_isolation ON policy_versions USING (tenant_id = current_tenant_id()) WITH CHECK (tenant_id = current_tenant_id());
GRANT SELECT ON teams, team_members, role_grants, routing_rules, policy_versions TO dispute_app;
GRANT UPDATE (team) ON disputes TO dispute_app;
```

Check that `INSERT INTO disputes` grants in `0002_app_role.sql` cover the new column (a table-level INSERT grant does); if `UPDATE` is granted per column there, the last line is needed, otherwise drop it.

- [ ] **Step 4: Queries and the store**

`queries/access.sql`:

```sql
-- name: GetPolicyVersion :one
SELECT COALESCE((SELECT version FROM policy_versions), 0)::bigint AS version;

-- name: ListTeams :many
SELECT slug, is_default FROM teams ORDER BY slug;

-- name: ListRoleGrants :many
SELECT team, role, action, amount_limit FROM role_grants ORDER BY team, role, action;

-- name: ListMemberships :many
SELECT team, role FROM team_members WHERE subject = $1 ORDER BY team;

-- name: ListRoutingRules :many
SELECT position, rail, reason, risk_tier, min_amount, team FROM routing_rules ORDER BY position;
```

Run `make api:generate`. Replace `txn.Access`:

```go
// Access implements application.Tx; RLS already limits every table to the tenant.
func (t *txn) Access(ctx context.Context, p principal.Principal) (application.Access, error) {
	version, err := t.q.GetPolicyVersion(ctx)
	if err != nil {
		return application.Access{}, mapErr(err)
	}
	a := application.Access{Version: version}
	teams, err := t.q.ListTeams(ctx)
	if err != nil {
		return application.Access{}, mapErr(err)
	}
	for _, tm := range teams {
		a.Teams = append(a.Teams, tm.Slug)
		if tm.IsDefault {
			a.DefaultTeam = tm.Slug
		}
	}
	grants, err := t.q.ListRoleGrants(ctx)
	if err != nil {
		return application.Access{}, mapErr(err)
	}
	for _, g := range grants {
		a.Grants = append(a.Grants, application.Grant{Team: g.Team, Role: application.Role(g.Role), Action: application.Action(g.Action), AmountLimit: decimalPtr(g.AmountLimit)})
	}
	rules, err := t.q.ListRoutingRules(ctx)
	if err != nil {
		return application.Access{}, mapErr(err)
	}
	for _, r := range rules {
		a.Routing = append(a.Routing, routingRule(r))
	}
	if p.Kind == principal.Analyst {
		ms, err := t.q.ListMemberships(ctx, p.ID)
		if err != nil {
			return application.Access{}, mapErr(err)
		}
		for _, m := range ms {
			a.Members = append(a.Members, application.Membership{Team: m.Team, Role: application.Role(m.Role)})
		}
	}
	return a, nil
}
```

Write `decimalPtr` and `routingRule` against the generated sqlc types (nullable numeric via the shopspring pgx adapter already used for `disputed_amount`; nullable text as `*string`). Add `team` to the `GetDispute`, `ListDisputes` and `InsertDispute` queries and map it to `DisputeRecord.Team`; `InsertDispute` passes `d.Team`.

- [ ] **Step 5: Run and see it pass**

Run: `make api:test-integration`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend
git commit -m "feat(authz): teams, members, grants and routing as tenant data with a policy version"
```

### Task 8: Team scope in row-level security

**Files:**
- Create: `backend/migrations/0024_team_scope.sql`
- Modify: `backend/internal/dispute/infrastructure/postgres/store.go` (`WithTx` sets `app.subject`, `app.principal_kind`)
- Modify: `backend/internal/dispute/application/apptest/memstore.go` (membership filtering on dispute reads)
- Test: `backend/internal/dispute/infrastructure/postgres/team_scope_test.go` (integration), `backend/internal/dispute/application/team_scope_test.go`

- [ ] **Step 1: Write the failing integration test**

```go
//go:build integration

func TestTeamScopeUnderRLS(t *testing.T) {
	owner, schema := pgtest.PoolWithSchema(t)
	app := pgtest.AppPool(t, schema)
	store := disputepg.NewStore(app)
	svc := apptest.NewService(t, store)
	mustExec(t, owner, `INSERT INTO teams (tenant_id, slug, name) VALUES ($1, 'fraud', 'Fraud')`, apptest.TenantA)
	mustExec(t, owner, `INSERT INTO team_members (tenant_id, team, subject, role) VALUES ($1, 'general', $2, 'lead')`, apptest.TenantA, apptest.Lead.ID)
	txn := seedFor(t, owner, apptest.TenantA, domain.RailCard, "EUR")
	created, err := svc.CreateDispute(apptest.Ctx(), application.CreateDisputeInput{TransactionID: txn})
	if err != nil {
		t.Fatal(err)
	}
	outsider := apptest.CtxAs(principal.Principal{Kind: principal.Analyst, ID: "fraud-only", Display: "f@x"})
	mustExec(t, owner, `INSERT INTO team_members (tenant_id, team, subject, role) VALUES ($1, 'fraud', 'fraud-only', 'lead')`, apptest.TenantA)

	if _, err := svc.GetDispute(outsider, created.View.ID); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("other team read: %v", err)
	}
	page, err := svc.ListDisputes(outsider, application.ListQuery{Limit: 50})
	if err != nil || len(page.Items) != 0 {
		t.Fatalf("other team list: %d items, %v", len(page.Items), err)
	}
	key := apptest.CtxAs(principal.Principal{Kind: principal.Key, ID: uuid.NewString(), Display: "key:tk_x"})
	if _, err := svc.GetDispute(key, created.View.ID); err != nil {
		t.Fatalf("key read: %v", err)
	}
	// restrictive, not permissive: team scope must never widen tenant isolation
	otherTenant := principal.With(tenant.WithID(context.Background(), apptest.TenantB), apptest.Lead)
	if _, err := svc.GetDispute(otherTenant, created.View.ID); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("cross-tenant read: %v", err)
	}
}
```

Adjust `ListQuery` and `Page` field names to the real ones in `service.go:828`.

- [ ] **Step 2: Run and see it fail**

Run: `make api:test-integration`
Expected: FAIL at "other team read" (the dispute is visible).

- [ ] **Step 3: Migration**

`backend/migrations/0024_team_scope.sql`:

```sql
-- A second, independent line behind the Cedar checks: an analyst sees only disputes of teams they belong to. These
-- policies are RESTRICTIVE because permissive policies are ORed with tenant_isolation and would widen it.
CREATE FUNCTION current_subject() RETURNS text LANGUAGE sql STABLE AS $$
    SELECT NULLIF(current_setting('app.subject', true), '')
$$;
CREATE FUNCTION current_principal_kind() RETURNS text LANGUAGE sql STABLE AS $$
    SELECT NULLIF(current_setting('app.principal_kind', true), '')
$$;
CREATE FUNCTION in_team(t uuid, team text) RETURNS boolean LANGUAGE sql STABLE AS $$
    SELECT current_principal_kind() = 'key'
        OR EXISTS (SELECT 1 FROM team_members m WHERE m.tenant_id = t AND m.team = in_team.team AND m.subject = current_subject())
$$;

CREATE POLICY team_scope_read   ON disputes AS RESTRICTIVE FOR SELECT USING (in_team(tenant_id, team));
CREATE POLICY team_scope_insert ON disputes AS RESTRICTIVE FOR INSERT WITH CHECK (in_team(tenant_id, team));
-- reassign moves a dispute out of the caller's teams; Cedar decides it, so the new row is not checked here
CREATE POLICY team_scope_update ON disputes AS RESTRICTIVE FOR UPDATE USING (in_team(tenant_id, team)) WITH CHECK (true);

CREATE POLICY team_scope ON dispute_events    AS RESTRICTIVE USING (EXISTS (SELECT 1 FROM disputes d WHERE d.id = dispute_id));
CREATE POLICY team_scope ON dispute_deadlines AS RESTRICTIVE USING (EXISTS (SELECT 1 FROM disputes d WHERE d.id = dispute_id));
CREATE POLICY team_scope ON ledger_entries    AS RESTRICTIVE USING (EXISTS (SELECT 1 FROM disputes d WHERE d.id = dispute_id));
CREATE POLICY team_scope ON questionnaires    AS RESTRICTIVE USING (EXISTS (SELECT 1 FROM disputes d WHERE d.id = dispute_id));
CREATE POLICY team_scope ON notices           AS RESTRICTIVE USING (EXISTS (SELECT 1 FROM disputes d WHERE d.id = dispute_id));
CREATE POLICY team_scope ON risk_assessments  AS RESTRICTIVE USING (EXISTS (SELECT 1 FROM disputes d WHERE d.id = dispute_id));
CREATE POLICY team_scope ON attachments       AS RESTRICTIVE USING (dispute_id IS NULL OR EXISTS (SELECT 1 FROM disputes d WHERE d.id = dispute_id));
```

Check each child table's column name for the dispute reference (`grep -n "dispute_id" backend/migrations/*.sql`) and whether `attachments.dispute_id` is nullable for drafts; adjust the last line to match. The owner-defined functions and views that bypass RLS (`claim_notices`, `outbox_backlog`, `disputes_by_state`) are unaffected.

- [ ] **Step 4: Existing integration tests keep their principal's reach**

Once the Postgres store reads real memberships, `apptest.Lead` has none and every existing integration test would see no disputes. Wherever the integration helpers create a test schema's tenants (`pgtest.PoolWithSchema` or the package's `seedFor`), also run as the owner, per tenant: `INSERT INTO team_members (tenant_id, team, subject, role) VALUES ($1, 'general', 'analyst-lead', 'lead') ON CONFLICT DO NOTHING`. Then run `make api:test-integration` and confirm the pre-existing tests still pass before adding the policies.

- [ ] **Step 5: WithTx binds the principal**

```go
func (s *Store) WithTx(ctx context.Context, fn func(application.Tx) error) error {
	id, ok := tenant.IDFrom(ctx)
	if !ok {
		return tenant.ErrMissing
	}
	p, _ := principal.From(ctx)
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		// an absent principal binds empty settings, which the team-scope policies treat as no teams
		if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true), set_config('app.subject', $2, true), set_config('app.principal_kind', $3, true)`,
			id.String(), p.ID, string(p.Kind)); err != nil {
			return mapErr(err)
		}
		return fn(&txn{q: sqlcgen.New(tx)})
	})
}
```

- [ ] **Step 6: MemStore parity, with a unit test**

Add to `MemStore`: `Access map[uuid.UUID]*MemAccess` where

```go
// MemAccess is one tenant's access data; a tenant without one gets application.DefaultAccess with Lead as a member.
type MemAccess struct {
	Version int64
	Teams   []string
	Default string
	Grants  []application.Grant
	Routing []application.RoutingRule
	Members map[string][]application.Membership // by subject
}
```

`memTx.Access` builds `application.Access` from it (or `DefaultAccess(p)` when absent). `memTx.GetDispute` returns `application.ErrNotFound` and `ListDisputes` skips records when `!t.visible(rec)`:

```go
// visible mirrors the team_scope policies: keys see the tenant, analysts their teams.
func (t *memTx) visible(rec application.DisputeRecord) bool {
	if t.principal.Kind == principal.Key {
		return true
	}
	a, _ := t.Access(context.Background(), t.principal)
	for _, m := range a.Members {
		if m.Team == rec.Team {
			return true
		}
	}
	return false
}
```

`memTx` gains a `principal principal.Principal` field set in `WithTx` from `principal.From(ctx)`. Child reads (`ListEvents`, `ListLedger`, ...) go through the dispute lookup already in the service, so filtering the two dispute reads is enough for parity.

Unit test `application/team_scope_test.go`: the same three assertions as the integration test (other team 404, other team list empty, key sees it) over `apptest.NewMemStore()` with a `MemAccess` of two teams.

- [ ] **Step 7: Run both suites**

Run: `make api:test && make api:test-integration`
Expected: PASS, including the cross-tenant assertion.

- [ ] **Step 8: Commit**

```bash
git add backend
git commit -m "feat(authz): row-level team scope beside tenant isolation, restrictive so it can only narrow"
```

### Task 9: Routing on create

**Files:**
- Create: `backend/internal/dispute/application/routing.go`
- Modify: `backend/internal/dispute/application/service.go` (`CreateDispute`)
- Test: `backend/internal/dispute/application/routing_test.go`

**Interfaces:**
- Produces: `func Route(a Access, rail domain.Rail, reason domain.Reason, tier string, amount decimal.Decimal) string`.

- [ ] **Step 1: Write the failing test**

```go
func TestRouteFirstMatchThenDefault(t *testing.T) {
	card, fraudReason, big := domain.RailCard, domain.Reason("FRAUD"), decimal.RequireFromString("1000")
	a := application.Access{DefaultTeam: "general", Routing: []application.RoutingRule{
		{Reason: &fraudReason, Team: "fraud"},
		{Rail: &card, MinAmount: &big, Team: "chargebacks"},
	}}
	cases := []struct {
		rail   domain.Rail
		reason domain.Reason
		amount string
		want   string
	}{
		{domain.RailCard, "FRAUD", "5", "fraud"},
		{domain.RailCard, "UNAUTHORISED", "1500", "chargebacks"},
		{domain.RailCard, "UNAUTHORISED", "10", "general"},
		{domain.RailSEPA, "UNAUTHORISED", "5000", "general"},
	}
	for _, c := range cases {
		if got := application.Route(a, c.rail, c.reason, "", decimal.RequireFromString(c.amount)); got != c.want {
			t.Errorf("%v/%v/%s = %s, want %s", c.rail, c.reason, c.amount, got, c.want)
		}
	}
}
```

Use the real reason and rail constants from `domain` (`grep -n "Reason\b\|Rail.* = " backend/internal/dispute/domain/*.go`).

- [ ] **Step 2: Run and see it fail**

Run: `cd backend && mise exec -- go test ./internal/dispute/application/ -run TestRoute`
Expected: compile failure.

- [ ] **Step 3: Implement**

```go
package application

// Route picks a new dispute's team: the first rule whose set fields all match, else the tenant's default team.
func Route(a Access, rail domain.Rail, reason domain.Reason, tier string, amount decimal.Decimal) string {
	for _, r := range a.Routing {
		if r.Rail != nil && *r.Rail != rail {
			continue
		}
		if r.Reason != nil && *r.Reason != reason {
			continue
		}
		if r.RiskTier != nil && *r.RiskTier != tier {
			continue
		}
		if r.MinAmount != nil && amount.LessThan(*r.MinAmount) {
			continue
		}
		return r.Team
	}
	return a.DefaultTeam
}
```

In `CreateDispute`, after parsing the reason: `p, _ := principal.From(ctx); a, err := tx.Access(ctx, p)`; `team := Route(a, txn.Rail, reason, "", txn.Amount)` (risk is assessed after insert, so creation routes without a tier; the tier field serves `reassign` tooling and later re-routing); authorize `ActionCreate` on `&DisputeFacts{Team: team}` instead of `DefaultTeam`; set `rec.Team = team`.

- [ ] **Step 4: Test the Review Focus case: routed to a team the creator is not in**

```go
func TestCreateIntoAnotherTeamIsForbidden(t *testing.T) {
	store := apptest.NewMemStore()
	fraud := domain.Reason("FRAUD")
	store.Access = map[uuid.UUID]*apptest.MemAccess{apptest.TenantA: {
		Version: 1, Teams: []string{"general", "fraud"}, Default: "general",
		Grants:  []application.Grant{{Team: "general", Role: application.RoleJunior, Action: application.ActionCreate}, {Team: "fraud", Role: application.RoleJunior, Action: application.ActionCreate}},
		Routing: []application.RoutingRule{{Reason: &fraud, Team: "fraud"}},
		Members: map[string][]application.Membership{apptest.Lead.ID: {{Team: "general", Role: application.RoleLead}}},
	}}
	svc := apptest.NewService(t, store)
	txn := store.AddTransaction(domain.RailCard, "EUR", "EUR", "40.00")
	_, err := svc.CreateDispute(apptest.Ctx(), application.CreateDisputeInput{TransactionID: txn, Reason: "FRAUD"})
	if !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("err = %v", err)
	}
	if len(store.Disputes) != 0 {
		t.Fatal("a dispute was stored")
	}
}
```

- [ ] **Step 5: Run and see it pass**

Run: `make api:test && make api:test-integration`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend
git commit -m "feat(disputes): route a new dispute to a team, and refuse creators outside it"
```

### Task 10: Reassign

**Files:**
- Modify: `docs/api/openapi.yaml` (new `POST /disputes/{disputeId}/team`, operationId `reassignDispute`, body `{ team: string }`, responses 204, 403, 404, 422)
- Modify: `backend/internal/dispute/application/service.go` (new `Reassign`), `ports.go` (`Tx.UpdateDisputeTeam`), `store.go`, `memstore.go`, `queries/disputes.sql`
- Modify: `backend/internal/dispute/ports/http/handler.go`, `operations.go` (`"reassignDispute": decided`)
- Test: `backend/internal/dispute/application/reassign_test.go`

**Interfaces:**
- Produces: `func (s *Service) Reassign(ctx context.Context, disputeID uuid.UUID, team string) error`; `Tx.UpdateDisputeTeam(ctx, id uuid.UUID, expected int64, team string) error`; `ErrUnknownTeam = errs.New(errs.Unprocessable, "unknown-team", "no such team in this tenant")` (add `unknown-team` to the `ErrorCode` enum).

- [ ] **Step 1: Write the failing tests**

```go
func TestReassign(t *testing.T) {
	store := apptest.NewMemStore()
	store.Access = map[uuid.UUID]*apptest.MemAccess{apptest.TenantA: twoTeams()} // general and fraud; Lead is lead of general only
	svc := apptest.NewService(t, store)
	txn := store.AddTransaction(domain.RailCard, "EUR", "EUR", "40.00")
	created, err := svc.CreateDispute(apptest.Ctx(), application.CreateDisputeInput{TransactionID: txn})
	if err != nil {
		t.Fatal(err)
	}
	junior := apptest.CtxAs(principal.Principal{Kind: principal.Analyst, ID: "junior-general", Display: "j@x"})
	if err := svc.Reassign(junior, created.View.ID, "fraud"); !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("junior reassign: %v", err)
	}
	if err := svc.Reassign(apptest.Ctx(), created.View.ID, "nope"); !errors.Is(err, application.ErrUnknownTeam) {
		t.Fatalf("unknown team: %v", err)
	}
	if err := svc.Reassign(apptest.Ctx(), created.View.ID, "fraud"); err != nil {
		t.Fatal(err)
	}
	if store.Disputes[created.View.ID].Team != "fraud" {
		t.Fatal("team unchanged")
	}
	if _, err := svc.GetDispute(apptest.Ctx(), created.View.ID); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("lead of general still sees it: %v", err)
	}
	evs := store.Events[created.View.ID]
	if last := evs[len(evs)-1]; last.Event != "REASSIGNED" || !strings.Contains(string(last.Payload), `"to":"fraud"`) {
		t.Fatalf("audit event = %+v", last)
	}
}
```

`twoTeams()` returns a `MemAccess` with teams `general` (default) and `fraud`, the `DefaultAccess` grants on `general` plus `create` on `fraud`, `Lead` as lead of `general`, and `junior-general` as junior of `general`.

- [ ] **Step 2: Run and see it fail**

Run: `cd backend && mise exec -- go test ./internal/dispute/application/ -run TestReassign`
Expected: compile failure.

- [ ] **Step 3: Implement**

```go
// Reassign moves a dispute to another team; the move is an event, so the log shows who moved it and why it was allowed.
func (s *Service) Reassign(ctx context.Context, disputeID uuid.UUID, team string) error {
	return s.store.WithTx(ctx, func(tx Tx) error {
		rec, err := tx.GetDispute(ctx, disputeID)
		if err != nil {
			return err
		}
		p, _ := principal.From(ctx)
		a, err := tx.Access(ctx, p)
		if err != nil {
			return err
		}
		if !slices.Contains(a.Teams, team) {
			return ErrUnknownTeam
		}
		f, err := facts(ctx, tx, rec)
		if err != nil {
			return err
		}
		dec, err := s.authorize(ctx, tx, ActionReassign, &f)
		if err != nil {
			return err
		}
		if err := tx.UpdateDisputeTeam(ctx, rec.ID, rec.Version, team); err != nil {
			return err
		}
		payload, err := json.Marshal(map[string]any{"from": rec.Team, "to": team, "authorizedBy": dec.Policies})
		if err != nil {
			return err
		}
		display, id := actorOf(ctx)
		return tx.AppendEvent(ctx, rec.ID, EventRecord{Seq: int(rec.Version) + 1, Event: "REASSIGNED", FromState: rec.State, ToState: rec.State,
			Actor: display, ActorID: id, Payload: payload, TraceID: traceIDPtr(ctx), OccurredAt: s.now()})
	})
}
```

`UpdateDisputeTeam` bumps `version` the way `UpdateDisputeState` does (copy its query and its `ErrConflict` on zero rows). Check that `REASSIGNED` is accepted by any `CHECK` on `dispute_events.event` (`grep -n "event" backend/migrations/0001*.sql`); if events are constrained, extend the constraint in migration 0024. The handler maps `nil` to `oapi.ReassignDispute204Response{}` and the error through `h.problem` like `ApplyDisputeEvent`.

- [ ] **Step 4: Run and see it pass**

Run: `make api:generate && make api:test && make api:test-integration`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend docs/api
git commit -m "feat(disputes): leads reassign a dispute to another team, recorded as an event"
```

### Task 11: Seeds, realm users, operator command

**Files:**
- Create: `deploy/seed/access/otp.yaml`, `deploy/seed/access/erste.yaml`, `deploy/seed/access/raiff.yaml`
- Modify: `backend/cmd/seed/main.go` (load access fixtures)
- Modify: `deploy/keycloak/realm.template.json` (fixed user ids, more users) and the renderer that fills it (`make api:generate` regenerates `deploy/keycloak/import`)
- Modify: `backend/cmd/tenant` (a `member add` subcommand) and `scripts/tenant`
- Modify: `docs/authentication.md`
- Test: `backend/cmd/seed/access_test.go`

- [ ] **Step 1: Fixture format and a failing parse test**

`deploy/seed/access/otp.yaml`:

```yaml
teams:
  - { slug: general, name: General, default: true }
  - { slug: chargebacks, name: Chargebacks }
  - { slug: fraud, name: Fraud }
members:
  - { subject: 00000000-0000-4000-8000-0000000000a1, team: general, role: lead }      # analyst (demo, eval)
  - { subject: 00000000-0000-4000-8000-0000000000a1, team: chargebacks, role: lead }
  - { subject: 00000000-0000-4000-8000-0000000000a1, team: fraud, role: lead }
  - { subject: 00000000-0000-4000-8000-0000000000a2, team: chargebacks, role: junior } # junior
  - { subject: 00000000-0000-4000-8000-0000000000a3, team: fraud, role: senior }      # fraud
grants:
  - { team: chargebacks, role: junior, actions: [OPEN_INVESTIGATION, SEND_QUESTIONNAIRE, RECEIVE_QUESTIONNAIRE, FILE_CHARGEBACK, ACKNOWLEDGE_CHARGEBACK, SUBMIT_EVIDENCE, compose-email, upload-attachment, resend-notice, create] }
  - { team: chargebacks, role: junior, actions: [ISSUE_FINAL_CREDIT, ISSUE_REFUND], amount_limit: "500.00" }
  - { team: chargebacks, role: senior, actions: [ISSUE_FINAL_CREDIT, ISSUE_REFUND, WIN_CHARGEBACK, LOSE_CHARGEBACK, REVERSE_PROVISIONAL_CREDIT, CLOSE, APPEAL] }
  - { team: chargebacks, role: lead, actions: [reassign] }
  - { team: fraud, role: junior, actions: [OPEN_INVESTIGATION, SEND_QUESTIONNAIRE, RECEIVE_QUESTIONNAIRE, compose-email, upload-attachment, create] }
  - { team: fraud, role: senior, actions: [ISSUE_REFUND, CLOSE, REVERSE_PROVISIONAL_CREDIT] }
  - { team: fraud, role: lead, actions: [reassign] }
routing:
  - { reason: FRAUD, team: fraud }
  - { rail: CARD, min_amount: "1000.00", team: chargebacks }
```

`erste.yaml` and `raiff.yaml` use the same shape with the same three subjects (different realms, same fixed ids). The `general` team and its grants already exist from the migration; the seeder upserts teams and replaces members, grants and routing for the tenant.

`access_test.go`:

```go
func TestFixturesParseAndCompile(t *testing.T) {
	for _, slug := range []string{"otp", "erste", "raiff"} {
		a, err := loadAccessFixture(filepath.Join("..", "..", "..", "deploy", "seed", "access", slug+".yaml"))
		if err != nil {
			t.Fatalf("%s: %v", slug, err)
		}
		if _, skipped := authz.Compile(uuid.Nil, a); len(skipped) > 0 {
			t.Fatalf("%s skipped grants: %v", slug, skipped)
		}
	}
}
```

- [ ] **Step 2: Run and see it fail**

Run: `cd backend && mise exec -- go test ./cmd/seed/`
Expected: compile failure (`loadAccessFixture`).

- [ ] **Step 3: Implement the loader and the seeding**

`loadAccessFixture(path string) (application.Access, error)` parses YAML (use `gopkg.in/yaml.v3`, or `sigs.k8s.io/yaml` if already in `go.mod`), expands each `actions` list into one `Grant` per action, and returns `Access` with `Members` holding every membership (the seeder needs them all; `Members` here is not per-caller). Seeding runs as the owner in one transaction per tenant: upsert the fixture's teams (`INSERT INTO teams ... ON CONFLICT (tenant_id, slug) DO UPDATE SET name = EXCLUDED.name, is_default = EXCLUDED.is_default`); for every team the fixture names, delete that team's `team_members` and `role_grants`; delete the tenant's `routing_rules`; insert the fixture's members, grants and routing. Teams the fixture does not name keep their rows, so rerunning gives the same result.

- [ ] **Step 4: Fixed Keycloak ids and users**

In `realm.template.json`, give the existing `analyst` user `"id": "00000000-0000-4000-8000-0000000000a1"` and add `junior` (`...a2`, realm role `analyst`) and `fraud` (`...a3`, realm role `analyst`) with passwords equal to their usernames, as the existing user has. Run `make api:generate` and commit the regenerated `deploy/keycloak/import/*.json`. Keycloak keeps the subject equal to the user id, which is what `team_members.subject` stores.

- [ ] **Step 5: Operator command for real users**

Add `tenant member add <slug> <subject> <team> <role>` to `backend/cmd/tenant` (following its existing subcommands; it runs as the owner) and document in `docs/authentication.md` how to find a user's subject in the Keycloak admin console and add them, for example the user `muneeb` in the OTP realm. Mention in the PR B description that existing real users need this once.

- [ ] **Step 6: Run the seed end to end**

Run: `make db:seed && make api:test && make api:test-integration`
Expected: PASS; `psql` as the owner shows three teams for each seeded tenant.

- [ ] **Step 7: Commit**

```bash
git add deploy backend docs
git commit -m "feat(seed): teams, members, grants and routing for the dev tenants, and an operator command for members"
```

### Task 12: Verification, ADR 0027, thesis, PR B

**Files:**
- Create: `backend/internal/dispute/authz/testdata/*.golden` and `backend/internal/dispute/authz/golden_test.go`
- Create: `backend/internal/dispute/authz/property_test.go`
- Modify: `.github/workflows/ci.yml` (a named step validating policies with the Cedar CLI)
- Create: `scripts/validate-policies`
- Create: `docs/adr/0027-teams-and-grants-as-data.md`
- Modify: `docs/thesis/latex/chapters/04-design.tex`

- [ ] **Step 1: Goldens for compiled policies and decisions**

`golden_test.go` loads each seed fixture (reuse `loadAccessFixture` by moving it to `authz` as `authz.LoadFixture` if the import direction allows; otherwise duplicate the tiny YAML shape in the test), writes `Compile` output to `testdata/<slug>.cedar.golden`, and a decision table to `testdata/<slug>.decisions.golden`: for each role in the ladder, each relation (own team, other team, none), each action in the schema, and two disputes (amount `100.00` and `900.00`), one line `role relation action amount -> allow|deny policy`. Use the repo's golden helper (`backend/internal/platform/golden`) and its update flag, as `notice/golden_test.go` does.

- [ ] **Step 2: Property tests**

`property_test.go`, using `pgregory.net/rapid` if present in `go.mod`, otherwise add it:

```go
// No generated grant set lets an investigator issue their own final credit.
func TestNoGrantBypassesSeparationOfDuties(t *testing.T) {
	e, _ := authz.New(slog.Default())
	rapid.Check(t, func(t *rapid.T) {
		a := genAccess(t) // random teams, grants over every action and role, random limits, the caller in random roles
		p := principal.Principal{Kind: principal.Analyst, ID: "investigator"}
		d := &application.DisputeFacts{ID: uuid.New(), Team: rapid.SampledFrom(a.Teams).Draw(t, "team"),
			Amount: decimal.NewFromInt(rapid.Int64Range(0, 100000).Draw(t, "amount")), InvestigationOpenedBy: "investigator"}
		dec, err := e.Decide(uuid.Nil, p, a, application.EventAction(domain.EventIssueFinalCredit), d)
		if err != nil || dec.Allowed {
			t.Fatalf("allowed=%v err=%v grants=%v", dec.Allowed, err, a.Grants)
		}
	})
}

// A grant never reaches a dispute outside its team: an analyst with no membership in the dispute's team is denied every dispute action.
func TestGrantsStayInTheirTeam(t *testing.T) { /* same generator; caller's memberships exclude d.Team; every Decide over dispute actions denies */ }
```

Write `genAccess` in the same file: 1 to 4 team slugs from `[a-z]{3,8}`, each grant a random team, role and action from the schema's action list, `AmountLimit` nil or a random decimal; members a random subset. The second property's body: draw `a`, choose `d.Team` among teams the caller is not a member of (skip with `t.Skip` if none), and assert every `Decide` over the 13 events plus `compose-email`, `upload-attachment`, `resend-notice`, `reassign` is denied.

- [ ] **Step 3: CI validation with the reference CLI**

`scripts/validate-policies`: compiles each fixture to a temp file with a tiny Go command (`go run ./backend/cmd/policy-dump <fixture>` that prints guardrails plus `Compile` output; create it) and runs `cedar validate --schema backend/internal/dispute/authz/policy/schema.cedarschema --policies <file>` for each. In `ci.yml`, in the backend checks job, add a named step "Validate policies" that installs the pinned CLI (`cargo install cedar-policy-cli --version <pin> --locked`, cached) and runs the script. Pin the version in `mise.toml` if the repo pins tools there, and add it to the `versions` check the Makefile runs.

- [ ] **Step 4: Benchmarks for the evaluation chapter**

Add `BenchmarkDecide` in `authz_test.go` over the OTP fixture (cached set) and `BenchmarkDecideCold` (new version each iteration), and record `make thesis:eval` p50 and p99 for `GetDispute` and `ApplyDisputeEvent` before and after PR B in the thesis section.

- [ ] **Step 5: ADR 0027 and the thesis**

ADR 0027: grants as data and guardrails as code; the forbid-overrides-permit argument for safe self-service; restrictive RLS policies and why permissive ones would widen isolation; policy versions for cache invalidation; subjects from Keycloak and the operator command. Extend the thesis `Authorization` section with teams, routing, the two enforcement layers, the properties proved, and the measured cost. Run the supervisor and em dash greps from Task 6.

- [ ] **Step 6: Full verification**

Run: `make ci && make api:test-integration && scripts/validate-policies`
Expected: PASS.

- [ ] **Step 7: Commit and open PR B**

```bash
git add .
git commit -m "test(authz): goldens, properties and reference validation for every seeded policy set"
git push -u origin HEAD
scripts/create-pr --title "feat(authz): teams own disputes, grants live in data, RLS scopes rows by team" \
  --description "Disputes now belong to teams, and what each role may do is data a tenant admin can later edit, bounded by guardrails that no grant can override. Row-level security narrows every read to the caller's teams as a second, independent line. Existing real users need a one-time membership, documented in the authentication guide." \
  --rollout "migrations 0023 and 0024; existing disputes move to each tenant's general team" --adr "0027"
```

Merge with `scripts/merge-when-green <number>` after review.
