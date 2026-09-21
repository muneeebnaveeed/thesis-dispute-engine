package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application/apptest"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
	disputepg "github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/infrastructure/postgres"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/postgres/pgtest"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/tenant"
)

// As dispute_api: postings are rows the tenant owns, cannot be changed, and stay invisible to another tenant;
// the suspense view sums across tenants for the gauge.
func TestLedgerThroughPostgres(t *testing.T) {
	owner, schema := pgtest.PoolWithSchema(t)
	app := pgtest.AppPool(t, schema)
	store := disputepg.NewStore(app)
	svc, err := application.NewService(store, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctxA := tenant.WithID(context.Background(), apptest.TenantA)
	ctxB := tenant.WithID(context.Background(), apptest.TenantB)
	txnA := seedFor(t, owner, apptest.TenantA, domain.RailCard, "EUR")
	seedFor(t, owner, apptest.TenantB, domain.RailCard, "EUR")

	created, err := svc.CreateDispute(ctxA, application.CreateDisputeInput{TransactionID: txnA})
	if err != nil {
		t.Fatal(err)
	}
	id := created.View.ID
	if _, err := svc.ApplyEvent(ctxA, application.ApplyEventInput{DisputeID: id, Event: domain.EventOpenInvestigation}); err != nil {
		t.Fatal(err)
	}
	res, err := svc.ApplyEvent(ctxA, application.ApplyEventInput{DisputeID: id, Event: domain.EventIssueRefund,
		Payload: json.RawMessage(`{"liability":"25.40"}`)})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.View.Ledger) != 1 || res.View.Ledger[0].Amount.String() != "100" || res.View.Balances.Suspense.String() != "100" {
		t.Fatalf("ledger = %+v balances %+v", res.View.Ledger, res.View.Balances)
	}

	// Refused facts roll the whole transition back: no event, no posting, state unchanged.
	if _, err := svc.ApplyEvent(ctxA, application.ApplyEventInput{DisputeID: id, Event: domain.EventClose,
		Payload: json.RawMessage(`{"settlement":"NOPE"}`)}); !errors.Is(err, domain.ErrInvalidSettlement) {
		t.Fatalf("bad settlement: %v", err)
	}
	view, _ := svc.GetDispute(ctxA, id)
	if view.State != domain.StateFastRefundIssued || len(view.Events) != 3 || len(view.Ledger) != 1 {
		t.Errorf("after a refused close: %s, %d events, %d postings", view.State, len(view.Events), len(view.Ledger))
	}

	// Tamper attempts fail at the database, as the owner, exactly like the event log.
	for _, stmt := range []string{
		`UPDATE ledger_entries SET amount = 1 WHERE dispute_id = $1`,
		`DELETE FROM ledger_entries WHERE dispute_id = $1`,
	} {
		if _, err := owner.Exec(context.Background(), stmt, id); err == nil {
			t.Errorf("%q succeeded; the ledger must be append-only", stmt)
		}
	}

	// Tenant B sees no ledger for A's dispute (nor the dispute), and the gauge view sums per tenant.
	if _, err := svc.GetDispute(ctxB, id); !errors.Is(err, application.ErrNotFound) {
		t.Errorf("cross-tenant read: %v", err)
	}
	balances, err := store.SuspenseBalances(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(balances) != 1 || balances[0].TenantID != apptest.TenantA || balances[0].Balance.String() != "100" || balances[0].Currency != "EUR" {
		t.Errorf("suspense by regime = %+v", balances)
	}

	closed, err := svc.ApplyEvent(ctxA, application.ApplyEventInput{DisputeID: id, Event: domain.EventClose,
		Payload: json.RawMessage(`{"settlement":"RECOVERED"}`)})
	if err != nil {
		t.Fatal(err)
	}
	if !closed.View.Balances.Suspense.IsZero() || closed.View.Balances.Recovery.String() != "100" {
		t.Errorf("after close: %+v", closed.View.Balances)
	}
}
