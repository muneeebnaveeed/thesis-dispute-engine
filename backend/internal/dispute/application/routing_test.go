package application_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application/apptest"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
)

func TestRouteFirstMatchThenDefault(t *testing.T) {
	card, unauthorised, big := domain.RailCard, domain.Reason("UNAUTHORISED"), decimal.RequireFromString("1000")
	a := application.Access{DefaultTeam: "general", Routing: []application.RoutingRule{
		{Reason: &unauthorised, Team: "fraud"},
		{Rail: &card, MinAmount: &big, Team: "chargebacks"},
	}}
	cases := []struct {
		rail   domain.Rail
		reason domain.Reason
		amount string
		want   string
	}{
		{domain.RailCard, "UNAUTHORISED", "5", "fraud"},
		{domain.RailCard, "NOT_RECEIVED", "1500", "chargebacks"},
		{domain.RailCard, "NOT_RECEIVED", "1000", "chargebacks"},
		{domain.RailCard, "NOT_RECEIVED", "10", "general"},
		{domain.RailSEPADD, "NOT_RECEIVED", "5000", "general"},
	}
	for _, c := range cases {
		if got := application.Route(a, c.rail, c.reason, "", decimal.RequireFromString(c.amount)); got != c.want {
			t.Errorf("%v/%v/%s = %s, want %s", c.rail, c.reason, c.amount, got, c.want)
		}
	}
}

func TestCreateIntoAnotherTeamIsForbidden(t *testing.T) {
	svc, store := newService(t)
	unauthorised := domain.Reason("UNAUTHORISED")
	access := twoTeams()
	access.Routing = []application.RoutingRule{{Reason: &unauthorised, Team: "fraud"}}
	store.Access = map[uuid.UUID]*apptest.MemAccess{apptest.TenantA: access}
	txn := store.AddTransaction(domain.RailCard, "EUR", "EUR", "40.00")
	_, err := svc.CreateDispute(apptest.Ctx(), application.CreateDisputeInput{TransactionID: txn, Reason: "UNAUTHORISED"})
	if !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("err = %v", err)
	}
	if len(store.Disputes) != 0 {
		t.Fatal("a dispute was stored")
	}
	res, err := svc.CreateDispute(apptest.Ctx(), application.CreateDisputeInput{TransactionID: txn, Reason: "NOT_RECEIVED"})
	if err != nil || store.Disputes[res.View.ID].Team != "general" {
		t.Fatalf("unrouted create: %v, team %q", err, store.Disputes[res.View.ID].Team)
	}
}
