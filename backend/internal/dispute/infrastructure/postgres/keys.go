package postgres

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/infrastructure/postgres/sqlcgen"
)

// touchEvery bounds last_used_at writes to one per key per interval; the column is a rotation aid, not an audit log.
const touchEvery = time.Minute

// KeyStore implements auth.Resolver over tenant_keys.
type KeyStore struct {
	pool    *pgxpool.Pool
	touched sync.Map // key id -> time.Time of the last recorded use
}

// NewKeyStore wraps a pool.
func NewKeyStore(pool *pgxpool.Pool) *KeyStore { return &KeyStore{pool: pool} }

// TenantForKeyHash returns the tenant owning a live key; unknown, revoked and expired keys are all ErrNotFound.
func (k *KeyStore) TenantForKeyHash(ctx context.Context, hash []byte) (uuid.UUID, error) {
	row, err := sqlcgen.New(k.pool).GetTenantByTenantKeyHash(ctx, hash)
	if err != nil {
		return uuid.Nil, mapErr(err)
	}
	k.touch(ctx, row.ID)
	return row.TenantID, nil
}

// touch records use off the request path; a failed write is invisible to the caller.
func (k *KeyStore) touch(ctx context.Context, id uuid.UUID) {
	now := time.Now()
	if last, ok := k.touched.Load(id); ok && now.Sub(last.(time.Time)) < touchEvery {
		return
	}
	k.touched.Store(id, now)
	go func() {
		bg, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = sqlcgen.New(k.pool).TouchTenantKey(bg, id)
	}()
}
