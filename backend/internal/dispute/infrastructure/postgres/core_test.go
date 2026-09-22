//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application/apptest"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/infrastructure/mockcore"
	disputepg "github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/infrastructure/postgres"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/postgres/pgtest"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/tenant"
)

// The tenant's core comes from its settings row, the receipt lands on the ledger row, and a decline leaves no row.
func TestCoreReceiptsThroughPostgres(t *testing.T) {
	owner, schema := pgtest.PoolWithSchema(t)
	app := pgtest.AppPool(t, schema)
	core := mockcore.New(slog.New(slog.NewTextHandler(io.Discard, nil)))
	svc, err := application.NewService(disputepg.NewStore(app), nil,
		application.WithCore(application.CoreRouter{Adapters: map[string]application.BankingCore{mockcore.Kind: core}}))
	if err != nil {
		t.Fatal(err)
	}
	ctx := tenant.WithID(context.Background(), apptest.TenantA)
	txn := seedFor(t, owner, apptest.TenantA, domain.RailCard, "EUR")
	if _, err := owner.Exec(context.Background(),
		`UPDATE tenants SET settings = settings || '{"core":{"kind":"mock","declineAbove":"100"}}' WHERE id = $1`, apptest.TenantA); err != nil {
		t.Fatal(err)
	}

	created, _ := svc.CreateDispute(ctx, application.CreateDisputeInput{TransactionID: txn})
	id := created.View.ID
	if _, err := svc.ApplyEvent(ctx, application.ApplyEventInput{DisputeID: id, Event: domain.EventOpenInvestigation}); err != nil {
		t.Fatal(err)
	}
	// 125.40 is over this tenant's limit: declined, nothing written.
	if _, err := svc.ApplyEvent(ctx, application.ApplyEventInput{DisputeID: id, Event: domain.EventIssueRefund}); !errors.Is(err, application.ErrCoreDeclined) {
		t.Fatalf("over the limit: %v", err)
	}
	var rows int
	_ = owner.QueryRow(context.Background(), `SELECT count(*) FROM ledger_entries WHERE dispute_id = $1`, id).Scan(&rows)
	if rows != 0 {
		t.Errorf("ledger rows after a decline = %d", rows)
	}

	// Raise the limit; the credit posts and the receipt survives a fresh read.
	if _, err := owner.Exec(context.Background(),
		`UPDATE tenants SET settings = settings || '{"core":{"kind":"mock"}}' WHERE id = $1`, apptest.TenantA); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ApplyEvent(ctx, application.ApplyEventInput{DisputeID: id, Event: domain.EventIssueRefund}); err != nil {
		t.Fatal(err)
	}
	view, _ := svc.GetDispute(ctx, id)
	if len(view.Ledger) != 1 || view.Ledger[0].Core == nil || view.Ledger[0].Core.ResponseCode != "00" || len(view.Ledger[0].Core.RRN) != 12 {
		t.Errorf("ledger = %+v", view.Ledger)
	}
}
