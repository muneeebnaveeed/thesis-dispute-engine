//go:build integration

package migrations_test

import (
	"context"
	"testing"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/postgres"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/postgres/pgtest"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/migrations"
)

// Fresh schemas hide backfill bugs; this applies the first schema, writes rows the way the pre-tenancy service did,
// then runs the remaining migrations over them.
func TestMigrationsApplyOverPopulatedData(t *testing.T) {
	pool, _ := pgtest.EmptyPool(t)
	ctx := context.Background()
	files, err := postgres.Load(migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := postgres.Migrate(ctx, pool, files[:2]); err != nil {
		t.Fatalf("baseline: %v", err)
	}
	fixture := `
		INSERT INTO accounts (id, holder_name, currency) VALUES ('00000000-0000-8000-8000-0000000000e1', 'legacy', 'EUR');
		INSERT INTO transactions (id, account_id, rail, amount, currency, merchant, occurred_at)
			VALUES ('00000000-0000-8000-8000-0000000000e2', '00000000-0000-8000-8000-0000000000e1', 'CARD', 10, 'EUR', 'm', now());
		INSERT INTO disputes (id, regime, state, transaction_id, account_id, disputed_amount, currency)
			VALUES ('00000000-0000-8000-8000-0000000000e3', 'EU_PSD2_CARD', 'INITIATED', '00000000-0000-8000-8000-0000000000e2', '00000000-0000-8000-8000-0000000000e1', 10, 'EUR');
		INSERT INTO dispute_events (dispute_id, seq, event, from_state, to_state, actor)
			VALUES ('00000000-0000-8000-8000-0000000000e3', 1, 'OPENED', '', 'INITIATED', 'legacy');
		INSERT INTO idempotency_keys (scope, key, request_hash, status_code, response) VALUES ('s', 'k', '\x00', 201, '{}');`
	if _, err := pool.Exec(ctx, fixture); err != nil {
		t.Fatalf("fixture: %v", err)
	}

	if _, err := postgres.Migrate(ctx, pool, files); err != nil {
		t.Fatalf("remaining migrations over data: %v", err)
	}

	var orphans int
	if err := pool.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM accounts WHERE tenant_id IS NULL)
		     + (SELECT count(*) FROM transactions WHERE tenant_id IS NULL)
		     + (SELECT count(*) FROM disputes WHERE tenant_id IS NULL)
		     + (SELECT count(*) FROM dispute_events WHERE tenant_id IS NULL)
		     + (SELECT count(*) FROM idempotency_keys WHERE tenant_id IS NULL)`).Scan(&orphans); err != nil {
		t.Fatal(err)
	}
	if orphans != 0 {
		t.Errorf("%d rows without a tenant after backfill", orphans)
	}
	// The append-only guard is back on after the backfill.
	if _, err := pool.Exec(ctx, `UPDATE dispute_events SET actor = 'tampered'`); err == nil {
		t.Error("dispute_events accepted an UPDATE after migration")
	}
}
