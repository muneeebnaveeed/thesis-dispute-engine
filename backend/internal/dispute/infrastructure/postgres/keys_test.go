package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application/apptest"
	disputepg "github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/infrastructure/postgres"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/infrastructure/postgres/sqlcgen"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/auth"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/postgres/pgtest"
)

func TestKeyStoreLifecycle(t *testing.T) {
	owner, schema := pgtest.PoolWithSchema(t)
	app := pgtest.AppPool(t, schema)
	ctx := context.Background()
	q := sqlcgen.New(owner)
	if err := q.UpsertTenant(ctx, sqlcgen.UpsertTenantParams{ID: apptest.TenantA, Name: "a", Slug: "otp"}); err != nil {
		t.Fatal(err)
	}
	insert := func(secret string, expires pgtype.Timestamptz) uuid.UUID {
		id := uuid.New()
		if err := q.InsertTenantKey(ctx, sqlcgen.InsertTenantKeyParams{ID: id, TenantID: apptest.TenantA, KeyHash: auth.HashKey(secret), Prefix: auth.Prefix(secret), Label: "t", ExpiresAt: expires}); err != nil {
			t.Fatal(err)
		}
		return id
	}
	live := insert("tk_live_key_0001", pgtype.Timestamptz{})
	insert("tk_expired_key_1", pgtype.Timestamptz{Time: time.Now().Add(-time.Hour), Valid: true})
	revoked := insert("tk_revoked_key_1", pgtype.Timestamptz{})
	if _, err := q.RevokeTenantKey(ctx, revoked); err != nil {
		t.Fatal(err)
	}

	store := disputepg.NewKeyStore(app)
	if got, err := store.TenantForKeyHash(ctx, auth.HashKey("tk_live_key_0001")); err != nil || got != apptest.TenantA {
		t.Fatalf("live key: %v %v", got, err)
	}
	for _, secret := range []string{"tk_expired_key_1", "tk_revoked_key_1", "tk_never_issued"} {
		if _, err := store.TenantForKeyHash(ctx, auth.HashKey(secret)); !errors.Is(err, application.ErrNotFound) {
			t.Errorf("%s: err = %v, want ErrNotFound", secret, err)
		}
	}

	// Use is recorded off the request path, at most once per interval, by the API role.
	deadline := time.Now().Add(3 * time.Second)
	for {
		var used pgtype.Timestamptz
		_ = owner.QueryRow(ctx, `SELECT last_used_at FROM tenant_keys WHERE id = $1`, live).Scan(&used)
		if used.Valid {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("last_used_at never recorded")
		}
		time.Sleep(50 * time.Millisecond)
	}
	if _, err := owner.Exec(ctx, `UPDATE tenant_keys SET last_used_at = NULL WHERE id = $1`, live); err != nil {
		t.Fatal(err)
	}
	if _, err := store.TenantForKeyHash(ctx, auth.HashKey("tk_live_key_0001")); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	var used pgtype.Timestamptz
	_ = owner.QueryRow(ctx, `SELECT last_used_at FROM tenant_keys WHERE id = $1`, live).Scan(&used)
	if used.Valid {
		t.Error("second use within the interval wrote last_used_at again")
	}
}
