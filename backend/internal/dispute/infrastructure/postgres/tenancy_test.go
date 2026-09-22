//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application/apptest"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
	disputepg "github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/infrastructure/postgres"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/postgres/pgtest"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/tenant"
)

// Everything here runs as dispute_api, the role row-level security applies to; the owner pool only seeds.
func TestTenantIsolationUnderRLS(t *testing.T) {
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
	txnB := seedFor(t, owner, apptest.TenantB, domain.RailCard, "EUR")

	body := []byte(`{"transactionId":"` + txnA.String() + `"}`)
	created, err := svc.CreateDispute(ctxA, application.CreateDisputeInput{TransactionID: txnA, Idempotency: application.Idempotency{Key: "shared-key", RequestBody: body}})
	if err != nil {
		t.Fatal(err)
	}

	// B cannot see, advance, or even learn the existence of A's dispute or transaction.
	if _, err := svc.GetDispute(ctxB, created.View.ID); !errors.Is(err, application.ErrNotFound) {
		t.Errorf("cross-tenant read: err = %v", err)
	}
	if _, err := svc.ApplyEvent(ctxB, application.ApplyEventInput{DisputeID: created.View.ID, Event: domain.EventOpenInvestigation}); !errors.Is(err, application.ErrNotFound) {
		t.Errorf("cross-tenant write: err = %v", err)
	}
	if _, err := svc.CreateDispute(ctxB, application.CreateDisputeInput{TransactionID: txnA}); !errors.Is(err, application.ErrNotFound) {
		t.Errorf("dispute on another tenant's transaction: err = %v", err)
	}

	// The same client-chosen idempotency key is a fresh request for B, not a replay of A's response.
	bodyB := []byte(`{"transactionId":"` + txnB.String() + `"}`)
	res, err := svc.CreateDispute(ctxB, application.CreateDisputeInput{TransactionID: txnB, Idempotency: application.Idempotency{Key: "shared-key", RequestBody: bodyB}})
	if err != nil || res.Replayed || res.View.ID == created.View.ID {
		t.Fatalf("B with A's key: %+v %v", res, err)
	}

	// Without a tenant the store refuses; a raw query on the app connection with no setting sees nothing (fail closed).
	if err := store.WithTx(context.Background(), func(application.Tx) error { return nil }); !errors.Is(err, tenant.ErrMissing) {
		t.Errorf("no tenant: err = %v", err)
	}
	var visible int
	if err := app.QueryRow(context.Background(), `SELECT count(*) FROM disputes`).Scan(&visible); err != nil || visible != 0 {
		t.Errorf("unscoped app connection sees %d rows (err %v), want 0", visible, err)
	}
	var total int
	_ = owner.QueryRow(context.Background(), `SELECT count(*) FROM disputes`).Scan(&total)
	if total != 2 {
		t.Errorf("owner sees %d disputes, want 2", total)
	}

	// Writes cannot smuggle a row into another tenant: the policy's WITH CHECK rejects a mismatched tenant_id.
	_, err = app.Exec(ctxA, `SELECT set_config('app.tenant_id', $1, false)`, apptest.TenantA.String())
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.Exec(ctxA, `INSERT INTO accounts (id, tenant_id, holder_name, currency) VALUES (gen_random_uuid(), $1, 'x', 'EUR')`, apptest.TenantB)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
		t.Errorf("cross-tenant insert: want 42501, got %v", err)
	}

	// Cross-tenant reads the API is allowed: the by-state gauge and the purge, through owner-defined objects.
	counts, err := store.CountByState(context.Background())
	if err != nil || len(counts) != 2 {
		t.Fatalf("CountByState across tenants = %+v, %v", counts, err)
	}
	if _, err := owner.Exec(context.Background(), `UPDATE idempotency_keys SET created_at = now() - interval '2 days'`); err != nil {
		t.Fatal(err)
	}
	if n, err := store.PurgeIdempotencyKeys(context.Background(), time.Now().Add(-time.Hour)); err != nil || n != 2 {
		t.Fatalf("purge across tenants = %d, %v", n, err)
	}
}
