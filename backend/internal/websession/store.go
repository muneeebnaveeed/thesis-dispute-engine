// Package websession is the opaque session store the frontend server uses (ADR 0011): blobs in, blobs out, expiry
// enforced here. The API never decrypts them.
package websession

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/infrastructure/postgres/sqlcgen"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/errs"
)

// Blob is one stored session.
type Blob struct {
	TenantID   *uuid.UUID
	Ciphertext []byte
	ExpiresAt  time.Time
}

// MaxCiphertext bounds what one session may occupy; the spec enforces the same limit.
const MaxCiphertext = 16384

// Store persists blobs in web_sessions.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore wraps a pool.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Put stores or replaces a session.
func (s *Store) Put(ctx context.Context, id uuid.UUID, b Blob) error {
	if len(b.Ciphertext) == 0 || len(b.Ciphertext) > MaxCiphertext {
		return errs.New(errs.Invalid, "contract-violation", "ciphertext must be between 1 and 16384 bytes")
	}
	if !b.ExpiresAt.After(time.Now()) {
		return errs.New(errs.Invalid, "contract-violation", "expiresAt must be in the future")
	}
	var tenant pgtype.UUID
	if b.TenantID != nil {
		tenant = pgtype.UUID{Bytes: *b.TenantID, Valid: true}
	}
	return sqlcgen.New(s.pool).PutWebSession(ctx, sqlcgen.PutWebSessionParams{ID: id, TenantID: tenant, Ciphertext: b.Ciphertext, ExpiresAt: b.ExpiresAt})
}

// Get returns a live session or application.ErrNotFound.
func (s *Store) Get(ctx context.Context, id uuid.UUID) (Blob, error) {
	row, err := sqlcgen.New(s.pool).GetWebSession(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Blob{}, application.ErrNotFound
		}
		return Blob{}, err
	}
	b := Blob{Ciphertext: row.Ciphertext, ExpiresAt: row.ExpiresAt}
	if row.TenantID.Valid {
		t := uuid.UUID(row.TenantID.Bytes)
		b.TenantID = &t
	}
	return b, nil
}

// Delete removes a session; deleting a missing one is not an error.
func (s *Store) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := sqlcgen.New(s.pool).DeleteWebSession(ctx, id)
	return err
}

// PurgeExpired deletes dead sessions and reports how many.
func (s *Store) PurgeExpired(ctx context.Context) (int64, error) {
	return sqlcgen.New(s.pool).PurgeWebSessions(ctx)
}
