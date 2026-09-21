package postgres_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application/apptest"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
	disputepg "github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/infrastructure/postgres"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/infrastructure/postgres/sqlcgen"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/postgres/pgtest"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/tenant"
)

func seed(t *testing.T, pool *pgxpool.Pool, rail domain.Rail, currency string) uuid.UUID {
	t.Helper()
	return seedFor(t, pool, apptest.TenantA, rail, currency)
}

// seedFor inserts as the schema owner, which row-level security does not constrain, so tenant_id is explicit.
func seedFor(t *testing.T, pool *pgxpool.Pool, tenantID uuid.UUID, rail domain.Rail, currency string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	q := sqlcgen.New(pool)
	if err := q.UpsertTenant(ctx, sqlcgen.UpsertTenantParams{ID: tenantID, Name: "test", Slug: strings.ReplaceAll(tenantID.String(), "-", "")}); err != nil {
		t.Fatal(err)
	}
	account := uuid.New()
	if err := q.InsertAccount(ctx, sqlcgen.InsertAccountParams{ID: account, TenantID: tenantID, HolderName: "Test Holder", Currency: currency}); err != nil {
		t.Fatal(err)
	}
	txn := uuid.New()
	if err := q.InsertTransaction(ctx, sqlcgen.InsertTransactionParams{
		ID: txn, TenantID: tenantID, AccountID: account, Rail: string(rail), Amount: decimal.RequireFromString("125.4"),
		Currency: currency, Merchant: "ACME", OccurredAt: time.Now().Add(-48 * time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	return txn
}

func newService(t *testing.T) (*application.Service, *pgxpool.Pool) {
	t.Helper()
	pool := pgtest.Pool(t)
	svc, err := application.NewService(disputepg.NewStore(pool), nil)
	if err != nil {
		t.Fatal(err)
	}
	return svc, pool
}

func TestLifecycleRoundTripThroughPostgres(t *testing.T) {
	svc, pool := newService(t)
	ctx := apptest.Ctx()
	txn := seed(t, pool, domain.RailCard, "EUR")

	created, err := svc.CreateDispute(ctx, application.CreateDisputeInput{TransactionID: txn, Actor: "customer"})
	if err != nil {
		t.Fatal(err)
	}
	if created.View.Regime != domain.RegimeEUPSD2Card || created.View.DisputedAmount.String() != "125.4" {
		t.Errorf("created = %+v", created.View)
	}
	for _, e := range []domain.Event{domain.EventOpenInvestigation, domain.EventIssueRefund, domain.EventFileChargeback} {
		if _, err := svc.ApplyEvent(ctx, application.ApplyEventInput{DisputeID: created.View.ID, Event: e, Actor: "analyst:1"}); err != nil {
			t.Fatalf("apply %s: %v", e, err)
		}
	}
	view, err := svc.GetDispute(ctx, created.View.ID)
	if err != nil {
		t.Fatal(err)
	}
	if view.State != domain.StateChargebackFiled || view.Version != 4 || len(view.Events) != 4 {
		t.Errorf("view = %+v", view)
	}
	for i, e := range view.Events {
		if e.Seq != i+1 {
			t.Errorf("event %d has seq %d", i, e.Seq)
		}
	}
}

func TestEventLogIsAppendOnlyAtTheDatabase(t *testing.T) {
	svc, pool := newService(t)
	ctx := apptest.Ctx()
	txn := seed(t, pool, domain.RailCard, "USD")
	created, _ := svc.CreateDispute(ctx, application.CreateDisputeInput{TransactionID: txn})

	for _, stmt := range []string{
		`UPDATE dispute_events SET actor = 'tamper' WHERE dispute_id = $1`,
		`DELETE FROM dispute_events WHERE dispute_id = $1`,
	} {
		if _, err := pool.Exec(ctx, stmt, created.View.ID); err == nil {
			t.Errorf("%q succeeded; the log must be append-only", stmt)
		}
	}
	if _, err := pool.Exec(ctx, `TRUNCATE dispute_events`); err == nil {
		t.Error("TRUNCATE succeeded")
	}
	var n int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM dispute_events WHERE dispute_id = $1`, created.View.ID).Scan(&n)
	if n != 1 {
		t.Errorf("events after tampering attempts = %d", n)
	}
}

func TestConcurrentEventsOnOneDisputeSerialise(t *testing.T) {
	svc, pool := newService(t)
	ctx := apptest.Ctx()
	txn := seed(t, pool, domain.RailCard, "USD")
	created, _ := svc.CreateDispute(ctx, application.CreateDisputeInput{TransactionID: txn})

	const writers = 8
	var wg sync.WaitGroup
	results := make(chan error, writers)
	start := make(chan struct{})
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := svc.ApplyEvent(ctx, application.ApplyEventInput{DisputeID: created.View.ID, Event: domain.EventOpenInvestigation})
			results <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	ok, conflicts := 0, 0
	for err := range results {
		switch {
		case err == nil:
			ok++
		case errors.Is(err, application.ErrConflict), errors.Is(err, domain.ErrInvalidTransition):
			conflicts++
		default:
			t.Errorf("unexpected error: %v", err)
		}
	}
	if ok != 1 || conflicts != writers-1 {
		t.Errorf("ok=%d conflicts=%d, want exactly one winner", ok, conflicts)
	}
	view, _ := svc.GetDispute(ctx, created.View.ID)
	if view.Version != 2 || len(view.Events) != 2 {
		t.Errorf("after race: version=%d events=%d", view.Version, len(view.Events))
	}
}

func TestIdempotentReplayAcrossConnections(t *testing.T) {
	svc, pool := newService(t)
	ctx := apptest.Ctx()
	txn := seed(t, pool, domain.RailSEPADD, "EUR")
	body := []byte(`{"transactionId":"` + txn.String() + `"}`)
	in := application.CreateDisputeInput{TransactionID: txn, Idempotency: application.Idempotency{Key: "k-42", RequestBody: body}}

	first, err := svc.CreateDispute(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.CreateDispute(ctx, in)
	if err != nil || !second.Replayed || second.View.ID != first.View.ID {
		t.Fatalf("replay: %+v %v", second, err)
	}
	var n int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM disputes`).Scan(&n)
	if n != 1 {
		t.Errorf("disputes = %d", n)
	}
	in.Idempotency.RequestBody = []byte(`{"transactionId":"different"}`)
	if _, err := svc.CreateDispute(ctx, in); !errors.Is(err, application.ErrIdempotencyReuse) {
		t.Errorf("mismatch: err = %v", err)
	}
}

func TestRejectedTransitionLeavesNoTrace(t *testing.T) {
	svc, pool := newService(t)
	ctx := apptest.Ctx()
	txn := seed(t, pool, domain.RailCreditCard, "USD")
	created, _ := svc.CreateDispute(ctx, application.CreateDisputeInput{TransactionID: txn})

	if _, err := svc.ApplyEvent(ctx, application.ApplyEventInput{DisputeID: created.View.ID, Event: domain.EventIssueRefund}); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("Reg Z refund: err = %v", err)
	}
	view, _ := svc.GetDispute(ctx, created.View.ID)
	if view.Version != 1 || len(view.Events) != 1 {
		t.Errorf("rejected event wrote something: %+v", view)
	}
}

func TestPurgeIdempotencyKeys(t *testing.T) {
	svc, pool := newService(t)
	ctx := apptest.Ctx()
	txn := seed(t, pool, domain.RailSEPADD, "EUR")
	body := []byte(`{"transactionId":"` + txn.String() + `"}`)
	in := application.CreateDisputeInput{TransactionID: txn, Idempotency: application.Idempotency{Key: "k-old", RequestBody: body}}
	if _, err := svc.CreateDispute(ctx, in); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE idempotency_keys SET created_at = now() - interval '2 days' WHERE key = 'k-old'`); err != nil {
		t.Fatal(err)
	}
	in.Idempotency.Key = "k-new"
	if _, err := svc.CreateDispute(ctx, in); err != nil {
		t.Fatal(err)
	}

	store := disputepg.NewStore(pool)
	n, err := store.PurgeIdempotencyKeys(ctx, time.Now().Add(-24*time.Hour))
	if err != nil || n != 1 {
		t.Fatalf("purge = %d, %v", n, err)
	}
	var left int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM idempotency_keys`).Scan(&left)
	if left != 1 {
		t.Errorf("keys left = %d", left)
	}

	// While another session holds the sweep lock, a purge is a no-op rather than a second deleter.
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, disputepg.PurgeLockID); err != nil {
		t.Fatal(err)
	}
	defer conn.Exec(ctx, `SELECT pg_advisory_unlock($1)`, disputepg.PurgeLockID) //nolint:errcheck // test cleanup
	if _, err := pool.Exec(ctx, `UPDATE idempotency_keys SET created_at = now() - interval '2 days'`); err != nil {
		t.Fatal(err)
	}
	if n, err := store.PurgeIdempotencyKeys(ctx, time.Now().Add(-24*time.Hour)); err != nil || n != 0 {
		t.Fatalf("purge under contention = %d, %v", n, err)
	}
}

func TestListDisputesKeysetOrderUnderRLS(t *testing.T) {
	owner, schema := pgtest.PoolWithSchema(t)
	app := pgtest.AppPool(t, schema)
	svc, err := application.NewService(disputepg.NewStore(app), nil)
	if err != nil {
		t.Fatal(err)
	}
	ctxA := tenant.WithID(context.Background(), apptest.TenantA)
	ctxB := tenant.WithID(context.Background(), apptest.TenantB)
	txnA := seedFor(t, owner, apptest.TenantA, domain.RailCard, "EUR")
	txnB := seedFor(t, owner, apptest.TenantB, domain.RailCard, "EUR")
	order := make([]uuid.UUID, 0, 7)
	for range 7 {
		r, err := svc.CreateDispute(ctxA, application.CreateDisputeInput{TransactionID: txnA})
		if err != nil {
			t.Fatal(err)
		}
		order = append(order, r.View.ID)
	}
	if _, err := svc.CreateDispute(ctxB, application.CreateDisputeInput{TransactionID: txnB}); err != nil {
		t.Fatal(err)
	}
	var got []uuid.UUID
	var after *application.Cursor
	for range 5 {
		page, err := svc.ListDisputes(ctxA, application.ListQuery{Limit: 3, After: after})
		if err != nil {
			t.Fatal(err)
		}
		for _, it := range page.Items {
			got = append(got, it.ID)
		}
		if page.Next == nil {
			break
		}
		after = page.Next
	}
	if len(got) != 7 {
		t.Fatalf("saw %d disputes across pages, want 7 (tenant B excluded)", len(got))
	}
	for i := range got {
		if got[i] != order[len(order)-1-i] {
			t.Fatalf("order at %d: %s, want %s", i, got[i], order[len(order)-1-i])
		}
	}
	pageB, _ := svc.ListDisputes(ctxB, application.ListQuery{Limit: 10})
	if len(pageB.Items) != 1 {
		t.Errorf("tenant B sees %d, want 1", len(pageB.Items))
	}
}
