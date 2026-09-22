// Package auth resolves the caller's tenant from a bearer credential: a tenant key for a customer's systems, or an
// OIDC access token from the tenant's Keycloak realm for its analysts. Everything below sees only the tenant.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"slices"
	"strings"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/otel"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/errs"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/httpserver"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/tenant"
)

// ErrUnauthenticated covers a missing, malformed, unknown or revoked key; the detail never says which.
var ErrUnauthenticated = errs.New(errs.Unauthorized, "unauthenticated", "a valid tenant key is required")

// ErrForbidden is a valid credential that may not perform the operation.
var ErrForbidden = errs.New(errs.Forbidden, "forbidden", "this operation needs an analyst with the tenant-admin role")

// RoleTenantAdmin manages the tenant's keys and users.
const RoleTenantAdmin = "tenant-admin"

// RequireRole passes only analyst tokens whose realm granted the role; tenant keys and other analysts are refused.
func RequireRole(ctx context.Context, role string) error {
	p, ok := PrincipalFrom(ctx)
	if !ok || !p.HasRole(role) {
		return ErrForbidden
	}
	return nil
}

// Resolver maps a key hash to its tenant; application.ErrNotFound for unknown or revoked keys.
type Resolver interface {
	TenantForKeyHash(ctx context.Context, hash []byte) (uuid.UUID, error)
}

// NewSecret returns a 32-byte random key with a recognisable prefix so leaked keys can be grepped for.
func NewSecret() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "tk_" + base64.RawURLEncoding.EncodeToString(b[:]), nil
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

// Principal is what an analyst token resolves to.
type Principal struct {
	Tenant  uuid.UUID
	Subject string
	Sid     string
	Email   string
	Name    string
	Roles   []string
}

// HasRole reports whether the realm granted the role.
func (p Principal) HasRole(role string) bool { return slices.Contains(p.Roles, role) }

type ctxKey struct{}
type principalKey struct{}

// WithPrincipal returns ctx carrying an analyst and their tenant; Bearer uses it, and so do tests that need one.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return tenant.WithID(context.WithValue(ctx, principalKey{}, p), p.Tenant)
}

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
			// its own span, so the waterfall shows the credential check apart from the work it guards
			authCtx, span := otel.Tracer("auth").Start(ctx, "auth.authenticate")
			var id uuid.UUID
			var err error
			kind := "key"
			if LooksLikeJWT(cred) {
				kind = "oidc"
				var p Principal
				if oidc == nil {
					err = ErrUnauthenticated
				} else if p, err = oidc.Verify(authCtx, cred); err == nil {
					id = p.Tenant
					ctx = WithPrincipal(ctx, p)
					trace.SpanFromContext(ctx).SetAttributes(attribute.String("enduser.id", p.Subject))
					httpserver.Annotate(ctx, "user", p.Subject)
				}
			} else {
				id, err = keys.TenantForKeyHash(authCtx, HashKey(cred))
			}
			span.SetAttributes(attribute.String("auth.kind", kind), attribute.Bool("auth.accepted", err == nil))
			span.End()
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

type serviceKey struct{}

// ServiceKey marks requests that carry the shared secret for the frontend server's internal endpoints. An empty
// configured key disables them. Comparison is constant time; the header is never logged.
func ServiceKey(key string) httpserver.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got := r.Header.Get("X-Service-Key")
			if key != "" && got != "" && subtle.ConstantTimeCompare([]byte(got), []byte(key)) == 1 {
				httpserver.Annotate(r.Context(), "auth", "service")
				r = r.WithContext(context.WithValue(r.Context(), serviceKey{}, true))
			}
			next.ServeHTTP(w, r)
		})
	}
}

// IsService reports whether the request authenticated with the service key.
func IsService(ctx context.Context) bool {
	ok, _ := ctx.Value(serviceKey{}).(bool)
	return ok
}

// Required is the validator's authentication hook: it runs only for operations the spec marks as secured, once per
// security scheme the operation lists. bearerAuth needs a tenant; serviceKey needs the shared secret.
func Required(ctx context.Context, scheme string) error {
	if scheme == "serviceKey" {
		if IsService(ctx) {
			return nil
		}
		return ErrUnauthenticated
	}
	if _, ok := tenant.IDFrom(ctx); ok {
		return nil
	}
	if err, ok := ctx.Value(ctxKey{}).(error); ok {
		return err
	}
	return ErrUnauthenticated
}
