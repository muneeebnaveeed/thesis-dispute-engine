package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/google/uuid"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/errs"
)

// Audience is the value every analyst token must carry; the realm's dispute-api client scope adds it.
const Audience = "dispute-api"

// IssuerResolver maps a token issuer (one Keycloak realm per tenant) to the tenant; ErrNotFound for unknown issuers.
type IssuerResolver interface {
	TenantForIssuer(ctx context.Context, issuer string) (uuid.UUID, error)
}

// OIDC verifies analyst access tokens. Verifiers are built per issuer on first sight and cached; each fetches
// the realm's discovery document and JWKS through go-oidc, which refreshes keys on rotation.
type OIDC struct {
	issuers   IssuerResolver
	mu        sync.Mutex
	verifiers map[string]*oidc.IDTokenVerifier
}

// NewOIDC returns a verifier backed by the given issuer lookup.
func NewOIDC(issuers IssuerResolver) *OIDC {
	return &OIDC{issuers: issuers, verifiers: map[string]*oidc.IDTokenVerifier{}}
}

// LooksLikeJWT distinguishes an analyst token from a tenant key without touching the database.
func LooksLikeJWT(token string) bool {
	return strings.Count(token, ".") == 2 && !strings.HasPrefix(token, "tk_")
}

// Verify checks signature, expiry, audience and issuer, and returns the principal; every failure is ErrUnauthenticated.
func (o *OIDC) Verify(ctx context.Context, raw string) (Principal, error) {
	issuer, err := unverifiedIssuer(raw)
	if err != nil {
		return Principal{}, ErrUnauthenticated
	}
	tenant, err := o.issuers.TenantForIssuer(ctx, issuer)
	if err != nil {
		if errs.KindOf(err) == errs.NotFound {
			return Principal{}, ErrUnauthenticated
		}
		return Principal{}, err
	}
	v, err := o.verifier(ctx, issuer)
	if err != nil {
		return Principal{}, errs.Wrap(errs.New(errs.Unavailable, "unavailable", "the identity provider is temporarily unavailable"), "oidc discovery for %s: %v", issuer, err)
	}
	tok, err := v.Verify(ctx, raw)
	if err != nil {
		return Principal{}, ErrUnauthenticated
	}
	var claims struct {
		TenantID  string   `json:"tenant_id"`
		Sid       string   `json:"sid"`
		Email     string   `json:"email"`
		Name      string   `json:"preferred_username"`
		FirstName string   `json:"given_name"`
		LastName  string   `json:"family_name"`
		Roles     []string `json:"roles"`
	}
	if err := tok.Claims(&claims); err != nil {
		return Principal{}, ErrUnauthenticated
	}
	// The issuer decides the tenant; a tenant_id claim that disagrees means a misconfigured realm, so refuse.
	if claims.TenantID != "" && claims.TenantID != tenant.String() {
		return Principal{}, ErrUnauthenticated
	}
	// A realm whose scopes drop the sub mapper issues tokens that name nobody; the audit actor, the avatar key and
	// enduser.id all rest on the subject, so an anonymous token is no token.
	if tok.Subject == "" {
		return Principal{}, ErrUnauthenticated
	}
	return Principal{Tenant: tenant, Subject: tok.Subject, Sid: claims.Sid, Email: claims.Email,
		Name: claims.Name, FirstName: claims.FirstName, LastName: claims.LastName, Roles: claims.Roles}, nil
}

func (o *OIDC) verifier(ctx context.Context, issuer string) (*oidc.IDTokenVerifier, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if v, ok := o.verifiers[issuer]; ok {
		return v, nil
	}
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, err
	}
	v := provider.Verifier(&oidc.Config{ClientID: Audience})
	o.verifiers[issuer] = v
	return v, nil
}

// unverifiedIssuer reads iss from the payload before verification, only to pick which realm's keys to verify with.
func unverifiedIssuer(raw string) (string, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("not a jwt")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", err
	}
	var c struct {
		Iss string `json:"iss"`
	}
	if err := json.Unmarshal(payload, &c); err != nil || c.Iss == "" {
		return "", fmt.Errorf("no issuer")
	}
	return c.Iss, nil
}
