package authz_test

import (
	"testing"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/authz"
)

const fixture = `
teams:
  - { slug: general, name: General, default: true }
  - { slug: fraud, name: Fraud }
members:
  - { subject: s1, team: fraud, role: senior }
grants:
  - { team: fraud, role: junior, actions: [OPEN_INVESTIGATION, create] }
  - { team: fraud, role: senior, actions: [ISSUE_REFUND], amount_limit: "500.00" }
routing:
  - { reason: UNAUTHORISED, team: fraud }
  - { rail: CARD, min_amount: "1000.00", team: general }
`

func TestParseFixture(t *testing.T) {
	f, err := authz.ParseFixture([]byte(fixture))
	if err != nil {
		t.Fatal(err)
	}
	a := f.Access()
	if a.DefaultTeam != "general" || len(a.Teams) != 2 || len(a.Grants) != 3 || len(a.Routing) != 2 {
		t.Fatalf("access = %+v", a)
	}
	if g := a.Grants[2]; g.Action != "ISSUE_REFUND" || g.AmountLimit == nil || g.AmountLimit.String() != "500" {
		t.Fatalf("limited grant = %+v", g)
	}
	if r := a.Routing[1]; r.Rail == nil || *r.Rail != "CARD" || r.MinAmount == nil || r.Team != "general" {
		t.Fatalf("routing = %+v", r)
	}
	if len(f.Members) != 1 || f.Members[0].Subject != "s1" || f.Members[0].Role != application.RoleSenior {
		t.Fatalf("members = %+v", f.Members)
	}
	if _, skipped := authz.Compile(tenantID, a); len(skipped) > 0 {
		t.Fatalf("skipped %v", skipped)
	}
}

func TestParseFixtureRefusesUnknownFieldsAndTeams(t *testing.T) {
	for name, doc := range map[string]string{
		"unknown field":   "teams: [{ slug: general, name: G, default: true, colour: red }]",
		"undeclared team": "teams: [{ slug: general, name: G, default: true }]\ngrants: [{ team: nope, role: junior, actions: [create] }]",
		"no default":      "teams: [{ slug: general, name: G }]",
		"bad role":        "teams: [{ slug: general, name: G, default: true }]\nmembers: [{ subject: s, team: general, role: owner }]",
	} {
		if _, err := authz.ParseFixture([]byte(doc)); err == nil {
			t.Errorf("%s: parsed", name)
		}
	}
}
