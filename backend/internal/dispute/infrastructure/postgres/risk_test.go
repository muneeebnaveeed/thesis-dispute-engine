package postgres_test

import (
	"context"
	"testing"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application/apptest"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
	disputepg "github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/infrastructure/postgres"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/postgres/pgtest"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/tenant"
)

// Account history is counted within the tenant, the MCC comes from the transaction, and assessments persist.
func TestRiskThroughPostgres(t *testing.T) {
	owner, schema := pgtest.PoolWithSchema(t)
	app := pgtest.AppPool(t, schema)
	svc, err := application.NewService(disputepg.NewStore(app), nil)
	if err != nil {
		t.Fatal(err)
	}
	ctxA := tenant.WithID(context.Background(), apptest.TenantA)
	txn := seedFor(t, owner, apptest.TenantA, domain.RailCard, "EUR")
	if _, err := owner.Exec(context.Background(), `UPDATE transactions SET mcc = '5815' WHERE id = $1`, txn); err != nil {
		t.Fatal(err)
	}
	first, err := svc.CreateDispute(ctxA, application.CreateDisputeInput{TransactionID: txn})
	if err != nil {
		t.Fatal(err)
	}
	if first.View.Risk == nil || first.View.Risk.Score != 15 { // merchant category (10) and a brand-new account (5)
		t.Fatalf("first risk = %+v", first.View.Risk)
	}
	for _, e := range []domain.Event{domain.EventOpenInvestigation, domain.EventIssueRefund, domain.EventFileChargeback, domain.EventAcknowledgeChargeback, domain.EventLoseChargeback} {
		if _, err := svc.ApplyEvent(ctxA, application.ApplyEventInput{DisputeID: first.View.ID, Event: e}); err != nil {
			t.Fatal(err)
		}
	}
	second, err := svc.CreateDispute(ctxA, application.CreateDisputeInput{TransactionID: txn})
	if err != nil {
		t.Fatal(err)
	}
	// one other dispute (10) + one lost chargeback (15) + merchant category (10) + new account (5)
	if second.View.Risk.Score != 40 || second.View.Risk.Tier != domain.RiskMedium {
		t.Errorf("second risk = %+v", second.View.Risk)
	}
	view, _ := svc.GetDispute(ctxA, second.View.ID)
	if view.Risk == nil || view.Risk.Score != 40 || len(view.Risk.Signals) != 7 {
		t.Errorf("persisted risk = %+v", view.Risk)
	}
	page, _ := svc.ListDisputes(ctxA, application.ListQuery{Limit: 5})
	for _, it := range page.Items {
		if it.Risk == nil {
			t.Errorf("list row without a tier: %s", it.ID)
		}
	}
}
