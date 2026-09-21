// Package auth resolves the caller's tenant from a bearer credential: a tenant key for a customer's systems, or an
// OIDC access token from the tenant's Keycloak realm for its analysts. Everything below sees only the tenant.
package auth

import (
	"context"
	"crypto/sha256"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/errs"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/httpserver"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/tenant"
)

// ErrUnauthenticated covers a missing, malformed, unknown or revoked key; the detail never says which.
var ErrUnauthenticated = errs.New(errs.Unauthorized, "unauthenticated", "a valid tenant key is required")

// Resolver maps a key hash to its tenant; application.ErrNotFound for unknown or revoked keys.
type Resolver interface {
	TenantForKeyHash(ctx context.Context, hash []byte) (uuid.UUID, error)
}

// HashKey is the only form a key is ever stored or compared in.
func HashKey(key string) []byte {
	sum := sha256.Sum256([]byte(key))
	return sum[:]
}

// PrefixLen is how much of a key is stored in clear so a leaked value can be matched to its row.
const PrefixLen = 12

// Prefix returns the identifying, non-secret start of a key.
func Prefix(key string) string {
	if len(key) < PrefixLen {
		return key
	}
	return key[:PrefixLen]
}

type ctxKey struct{}
type principalKey struct{}

// PrincipalFrom returns the signed-in analyst, if the request carried an OIDC token rather than a tenant key.
func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}

// Bearer resolves the credential if one is presented and records the outcome; it never rejects by itself, because the
// OpenAPI spec decides which operations need a tenant (see Required) and health endpoints do not. A nil oidc means
// analyst tokens are not accepted.
func Bearer(keys Resolver, oidc *OIDC) httpserver.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			cred, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			cred = strings.TrimSpace(cred)
			if !ok || cred == "" {
				next.ServeHTTP(w, r.WithContext(context.WithValue(ctx, ctxKey{}, ErrUnauthenticated)))
				return
			}
			var id uuid.UUID
			var err error
			kind := "key"
			if LooksLikeJWT(cred) {
				kind = "oidc"
				var p Principal
				if oidc == nil {
					err = ErrUnauthenticated
				} else if p, err = oidc.Verify(ctx, cred); err == nil {
					id = p.Tenant
					ctx = context.WithValue(ctx, principalKey{}, p)
					trace.SpanFromContext(ctx).SetAttributes(attribute.String("enduser.id", p.Subject))
				}
			} else {
				id, err = keys.TenantForKeyHash(ctx, HashKey(cred))
			}
			if err != nil {
				if errs.KindOf(err) == errs.NotFound {
					err = ErrUnauthenticated
				}
				next.ServeHTTP(w, r.WithContext(context.WithValue(ctx, ctxKey{}, err)))
				return
			}
			trace.SpanFromContext(ctx).SetAttributes(attribute.String("tenant.id", id.String()), attribute.String("auth.kind", kind))
			httpserver.Annotate(ctx, "tenant", id.String())
			httpserver.Annotate(ctx, "auth", kind)
			next.ServeHTTP(w, r.WithContext(tenant.WithID(ctx, id)))
		})
	}
}

// Required is the validator's authentication hook: it runs only for operations the spec marks as secured.
func Required(ctx context.Context) error {
	if _, ok := tenant.IDFrom(ctx); ok {
		return nil
	}
	if err, ok := ctx.Value(ctxKey{}).(error); ok {
		return err
	}
	return ErrUnauthenticated
}
