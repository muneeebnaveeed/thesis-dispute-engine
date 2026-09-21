package websession_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/errs"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/postgres/pgtest"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/websession"
)

func TestStoreLifecycle(t *testing.T) {
	owner, schema := pgtest.PoolWithSchema(t)
	store := websession.NewStore(pgtest.AppPool(t, schema))
	ctx := context.Background()
	id := uuid.New()
	tenant := uuid.New()
	if _, err := owner.Exec(ctx, `INSERT INTO tenants (id, name, slug) VALUES ($1, 't', 'tenant-t')`, tenant); err != nil {
		t.Fatal(err)
	}

	if err := store.Put(ctx, id, websession.Blob{Ciphertext: []byte("c1"), ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	b, err := store.Get(ctx, id)
	if err != nil || string(b.Ciphertext) != "c1" || b.TenantID != nil {
		t.Fatalf("get: %+v %v", b, err)
	}
	if err := store.Put(ctx, id, websession.Blob{TenantID: &tenant, Ciphertext: []byte("c2"), ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if b, _ = store.Get(ctx, id); string(b.Ciphertext) != "c2" || b.TenantID == nil || *b.TenantID != tenant {
		t.Fatalf("replace: %+v", b)
	}

	if err := store.Put(ctx, uuid.New(), websession.Blob{Ciphertext: []byte("x"), ExpiresAt: time.Now().Add(-time.Second)}); errs.KindOf(err) != errs.Invalid {
		t.Errorf("past expiry accepted: %v", err)
	}
	if err := store.Put(ctx, uuid.New(), websession.Blob{Ciphertext: make([]byte, websession.MaxCiphertext+1), ExpiresAt: time.Now().Add(time.Hour)}); errs.KindOf(err) != errs.Invalid {
		t.Errorf("oversized blob accepted: %v", err)
	}

	// Expired rows are invisible before the sweep and gone after it.
	dead := uuid.New()
	if err := store.Put(ctx, dead, websession.Blob{Ciphertext: []byte("d"), ExpiresAt: time.Now().Add(150 * time.Millisecond)}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	if _, err := store.Get(ctx, dead); !errors.Is(err, application.ErrNotFound) {
		t.Errorf("expired row still readable: %v", err)
	}
	if n, err := store.PurgeExpired(ctx); err != nil || n != 1 {
		t.Errorf("purge = %d, %v", n, err)
	}
	if err := store.Delete(ctx, id); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(ctx, id); err != nil {
		t.Errorf("deleting twice: %v", err)
	}
}
