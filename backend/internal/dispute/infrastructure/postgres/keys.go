package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/infrastructure/postgres/sqlcgen"
)

// KeyStore implements auth.Resolver over api_keys.
type KeyStore struct {
	pool *pgxpool.Pool
}

// NewKeyStore wraps a pool.
func NewKeyStore(pool *pgxpool.Pool) *KeyStore { return &KeyStore{pool: pool} }

// TenantForKeyHash returns the tenant owning a live key; unknown and revoked keys are both ErrNotFound.
func (k *KeyStore) TenantForKeyHash(ctx context.Context, hash []byte) (uuid.UUID, error) {
	id, err := sqlcgen.New(k.pool).GetTenantByAPIKeyHash(ctx, hash)
	if err != nil {
		return uuid.Nil, mapErr(err)
	}
	return id, nil
}
