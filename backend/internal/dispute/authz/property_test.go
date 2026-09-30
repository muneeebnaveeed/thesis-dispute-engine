package authz_test

import (
	"log/slog"
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"pgregory.net/rapid"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/authz"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/principal"
)

// genAccess is any tenant a self-service UI could produce: random teams, grants over every action and role with
// random limits, and the caller in random memberships.
func genAccess(t *rapid.T) application.Access {
	teams := rapid.SliceOfNDistinct(rapid.StringMatching(`[a-z]{3,8}`), 1, 4, rapid.ID[string]).Draw(t, "teams")
	actions := append(disputeActions(), application.ActionCreate)
	grants := rapid.SliceOfN(rapid.Custom(func(t *rapid.T) application.Grant {
		g := application.Grant{
			Team:   rapid.SampledFrom(teams).Draw(t, "team"),
			Role:   rapid.SampledFrom(application.Roles).Draw(t, "role"),
			Action: rapid.SampledFrom(actions).Draw(t, "action"),
		}
		if rapid.Bool().Draw(t, "limited") {
			d := decimal.NewFromInt(rapid.Int64Range(0, 100000).Draw(t, "limit"))
			g.AmountLimit = &d
		}
		return g
	}), 0, 40).Draw(t, "grants")
	members := rapid.SliceOfN(rapid.Custom(func(t *rapid.T) application.Membership {
		return application.Membership{Team: rapid.SampledFrom(teams).Draw(t, "team"), Role: rapid.SampledFrom(application.Roles).Draw(t, "role")}
	}), 0, 4).Draw(t, "members")
	return application.Access{Version: rapid.Int64().Draw(t, "version"), DefaultTeam: teams[0], Teams: teams, Grants: grants, Members: members}
}

func genDispute(t *rapid.T, team, openedBy string) *application.DisputeFacts {
	return &application.DisputeFacts{ID: uuid.New(), Team: team, Amount: decimal.NewFromInt(rapid.Int64Range(1, 100000).Draw(t, "amount")),
		Currency: "EUR", Rail: domain.RailCard, State: domain.StateChargebackWon, InvestigationOpenedBy: openedBy}
}

// No grant set lets an investigator issue their own final credit.
func TestNoGrantBypassesSeparationOfDuties(t *testing.T) {
	e, err := authz.New(slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	rapid.Check(t, func(t *rapid.T) {
		a := genAccess(t)
		p := principal.Principal{Kind: principal.Analyst, ID: "investigator"}
		d := genDispute(t, rapid.SampledFrom(a.Teams).Draw(t, "disputeTeam"), "investigator")
		dec, err := e.Decide(uuid.Nil, p, a, application.EventAction(domain.EventIssueFinalCredit), d)
		if err != nil || dec.Allowed {
			t.Fatalf("allowed=%v err=%v members=%v grants=%v", dec.Allowed, err, a.Members, a.Grants)
		}
	})
}

// A grant never reaches outside its team: an analyst with no membership in the dispute's team is denied every
// dispute action, whatever they hold elsewhere.
func TestGrantsStayInTheirTeam(t *testing.T) {
	e, err := authz.New(slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	rapid.Check(t, func(t *rapid.T) {
		a := genAccess(t)
		var outside []string
		for _, team := range a.Teams {
			if !slices.ContainsFunc(a.Members, func(m application.Membership) bool { return m.Team == team }) {
				outside = append(outside, team)
			}
		}
		if len(outside) == 0 {
			t.Skip("the caller is in every team")
		}
		p := principal.Principal{Kind: principal.Analyst, ID: "caller"}
		d := genDispute(t, rapid.SampledFrom(outside).Draw(t, "disputeTeam"), "someone-else")
		for _, action := range disputeActions() {
			if dec, err := e.Decide(uuid.Nil, p, a, action, d); err != nil || dec.Allowed {
				t.Fatalf("%s allowed=%v err=%v", action, dec.Allowed, err)
			}
		}
	})
}

// Tenant administration is never reachable through a grant, only through the realm role.
func TestGrantsNeverAdministerTheTenant(t *testing.T) {
	e, err := authz.New(slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	rapid.Check(t, func(t *rapid.T) {
		a := genAccess(t)
		for _, team := range a.Teams {
			for _, role := range application.Roles {
				for _, action := range []application.Action{application.ActionManageKeys, application.ActionManageTemplates, application.ActionManageBranding} {
					a.Grants = append(a.Grants, application.Grant{Team: team, Role: role, Action: action})
				}
			}
		}
		p := principal.Principal{Kind: principal.Analyst, ID: "caller"}
		for _, action := range []application.Action{application.ActionManageKeys, application.ActionManageTemplates, application.ActionManageBranding} {
			if dec, err := e.Decide(uuid.Nil, p, a, action, nil); err != nil || dec.Allowed {
				t.Fatalf("%s allowed=%v err=%v", action, dec.Allowed, err)
			}
		}
	})
}
