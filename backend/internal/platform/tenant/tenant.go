// Package tenant carries the current tenant through a request; storage refuses to run without one.
package tenant

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/errs"
)

type ctxKey struct{}

// ErrMissing means code reached storage without a tenant; a programming error, never a client one.
var ErrMissing = errs.New(errs.Internal, "internal", "no tenant in context")

// WithID returns ctx carrying id.
func WithID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// IDFrom returns the tenant in ctx.
func IDFrom(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(ctxKey{}).(uuid.UUID)
	return id, ok && id != uuid.Nil
}

// Static stamps every request with one tenant: single-tenant mode until per-request authentication resolves it.
func Static(id uuid.UUID) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(WithID(r.Context(), id)))
		})
	}
}
