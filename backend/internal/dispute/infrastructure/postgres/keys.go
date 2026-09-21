package postgres

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/infrastructure/postgres/sqlcgen"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/auth"
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

// TenantForIssuer implements auth.IssuerResolver: one Keycloak realm per tenant, recorded on the tenant row.
func (k *KeyStore) TenantForIssuer(ctx context.Context, issuer string) (uuid.UUID, error) {
	row, err := sqlcgen.New(k.pool).GetTenantByIssuer(ctx, &issuer)
	if err != nil {
		return uuid.Nil, mapErr(err)
	}
	return row.ID, nil
}

// TenantSummary is what the sign-in page needs to send an analyst to the right realm.
type TenantSummary struct {
	ID           uuid.UUID
	Slug         string
	Name         string
	Issuer       string
	EmailDomains []string
}

// ActiveTenants lists tenants that are not disabled.
func (k *KeyStore) ActiveTenants(ctx context.Context) ([]TenantSummary, error) {
	rows, err := sqlcgen.New(k.pool).ListTenantsForDiscovery(ctx)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]TenantSummary, 0, len(rows))
	for _, r := range rows {
		t := TenantSummary{ID: r.ID, Slug: r.Slug, Name: r.Name, EmailDomains: r.EmailDomains}
		if r.OidcIssuer != nil {
			t.Issuer = *r.OidcIssuer
		}
		out = append(out, t)
	}
	return out, nil
}

// KeyRecord is one tenant key as the tenant's admins see it: never the secret, always the prefix.
type KeyRecord struct {
	ID         uuid.UUID
	Prefix     string
	Label      string
	CreatedAt  time.Time
	LastUsedAt *time.Time
	ExpiresAt  *time.Time
	RevokedAt  *time.Time
}

// Status derives live, expired or revoked at read time.
func (r KeyRecord) Status(now time.Time) string {
	switch {
	case r.RevokedAt != nil:
		return "revoked"
	case r.ExpiresAt != nil && !r.ExpiresAt.After(now):
		return "expired"
	default:
		return "live"
	}
}

// ListForTenant returns a tenant's keys, newest first.
func (k *KeyStore) ListForTenant(ctx context.Context, tenantID uuid.UUID) ([]KeyRecord, error) {
	rows, err := sqlcgen.New(k.pool).ListTenantKeysByTenant(ctx, tenantID)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]KeyRecord, 0, len(rows))
	for _, r := range rows {
		out = append(out, KeyRecord{ID: r.ID, Prefix: r.Prefix, Label: r.Label, CreatedAt: r.CreatedAt,
			LastUsedAt: tsPtr(r.LastUsedAt), ExpiresAt: tsPtr(r.ExpiresAt), RevokedAt: tsPtr(r.RevokedAt)})
	}
	slices.Reverse(out)
	return out, nil
}

// Issue creates a key for the tenant and returns the record and the secret, which exists only in this return.
func (k *KeyStore) Issue(ctx context.Context, tenantID uuid.UUID, label string, expiresAt *time.Time) (KeyRecord, string, error) {
	secret, err := auth.NewSecret()
	if err != nil {
		return KeyRecord{}, "", err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return KeyRecord{}, "", err
	}
	var exp pgtype.Timestamptz
	if expiresAt != nil {
		exp = pgtype.Timestamptz{Time: *expiresAt, Valid: true}
	}
	if err := sqlcgen.New(k.pool).InsertTenantKey(ctx, sqlcgen.InsertTenantKeyParams{
		ID: id, TenantID: tenantID, KeyHash: auth.HashKey(secret), Prefix: auth.Prefix(secret), Label: label, ExpiresAt: exp,
	}); err != nil {
		return KeyRecord{}, "", mapErr(err)
	}
	rec, err := k.getForTenant(ctx, id, tenantID)
	return rec, secret, err
}

// Revoke ends a key of the tenant; a key of another tenant is not found, never forbidden, so ids do not leak.
func (k *KeyStore) Revoke(ctx context.Context, tenantID, id uuid.UUID) error {
	if _, err := k.getForTenant(ctx, id, tenantID); err != nil {
		return err
	}
	_, err := sqlcgen.New(k.pool).RevokeTenantKeyForTenant(ctx, sqlcgen.RevokeTenantKeyForTenantParams{ID: id, TenantID: tenantID})
	return mapErr(err)
}

func (k *KeyStore) getForTenant(ctx context.Context, id, tenantID uuid.UUID) (KeyRecord, error) {
	r, err := sqlcgen.New(k.pool).GetTenantKeyForTenant(ctx, sqlcgen.GetTenantKeyForTenantParams{ID: id, TenantID: tenantID})
	if err != nil {
		return KeyRecord{}, mapErr(err)
	}
	return KeyRecord{ID: r.ID, Prefix: r.Prefix, Label: r.Label, CreatedAt: r.CreatedAt,
		LastUsedAt: tsPtr(r.LastUsedAt), ExpiresAt: tsPtr(r.ExpiresAt), RevokedAt: tsPtr(r.RevokedAt)}, nil
}

func tsPtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}
