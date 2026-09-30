package authz_test

import (
	"log/slog"
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
	key := principal.Principal{Kind: principal.Key, ID: "k"}
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
		{"key applies events", key, access(), credit, dispute("1.00", ""), true, "keys-disputes"},
		{"key cannot compose", key, access(), application.ActionComposeEmail, dispute("1.00", ""), false, "keys-not-people"},
		{"admin manages keys", principal.Principal{Kind: principal.Analyst, ID: "a", TenantAdmin: true}, access(), application.ActionManageKeys, nil, true, "tenant-admin"},
		{"non-admin cannot", analyst("a"), access(), application.ActionManageKeys, nil, false, ""},
		{"key cannot manage keys", key, access(), application.ActionManageKeys, nil, false, ""},
		{"junior creates in own team", analyst("j"), access(junior...), application.ActionCreate, &application.DisputeFacts{Team: "cb"}, true, "grant:cb/junior/create"},
		{"junior cannot create in another team", analyst("j"), access(junior...), application.ActionCreate, &application.DisputeFacts{Team: "fraud"}, false, ""},
		{"key creates anywhere", key, access(), application.ActionCreate, &application.DisputeFacts{Team: "fraud"}, true, "keys-create"},
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

// Cedar decimals stop at 922337203685477.5807; a limit beyond that must not turn one row into a denial for everyone,
// and an amount beyond it must still be decided.
func TestAmountsBeyondCedarRange(t *testing.T) {
	e, err := authz.New(slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	huge := decimal.RequireFromString("999999999999999.9999")
	a := access([]application.Membership{{Team: "cb", Role: application.RoleLead}}...)
	a.Grants = append(a.Grants, application.Grant{Team: "cb", Role: application.RoleJunior, Action: application.EventAction(domain.EventIssueRefund), AmountLimit: &huge})
	if _, skipped := authz.Compile(tenantID, a); len(skipped) != 1 {
		t.Fatalf("out-of-range limit compiled: skipped %v", skipped)
	}
	credit := application.EventAction(domain.EventIssueFinalCredit)
	if dec, err := e.Decide(tenantID, analyst("l"), a, credit, dispute("900.00", "other")); err != nil || !dec.Allowed {
		t.Fatalf("lead with an unconditional grant: %+v %v", dec, err)
	}
	big := dispute("1.00", "other")
	big.Amount = huge
	if dec, err := e.Decide(tenantID, analyst("l"), a, credit, big); err != nil || !dec.Allowed {
		t.Fatalf("huge dispute, unconditional grant: %+v %v", dec, err)
	}
	a.Members = []application.Membership{{Team: "cb", Role: application.RoleJunior}}
	if dec, err := e.Decide(tenantID, analyst("j"), a, credit, big); err != nil || dec.Allowed {
		t.Fatalf("huge dispute, limited grant: %+v %v", dec, err)
	}
}
