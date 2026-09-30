// Package principal carries who is calling, for both credential kinds, so authorization and audit never branch on
// how the caller authenticated.
package principal

import "context"

// Kind is how the caller authenticated.
type Kind string

// The credential kinds: an OIDC token from the tenant's realm, or a tenant key.
const (
	Analyst Kind = "analyst"
	Key     Kind = "key"
)

// Principal is the authenticated caller. ID is stable (the OIDC subject or the tenant key id) and is what policies and
// the audit trail compare; Display is for people reading the log.
type Principal struct {
	Kind        Kind
	ID          string
	Display     string
	TenantAdmin bool
}

type ctxKey struct{}

// With returns ctx carrying p.
func With(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, ctxKey{}, p)
}

// From returns the caller, if the request authenticated.
func From(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(ctxKey{}).(Principal)
	return p, ok
}
