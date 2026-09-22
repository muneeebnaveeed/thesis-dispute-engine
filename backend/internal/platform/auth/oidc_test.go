package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/google/uuid"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/errs"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/tenant"
)

// fakeRealm is a minimal OIDC issuer: discovery, JWKS, and a signer for minting tokens.
type fakeRealm struct {
	srv    *httptest.Server
	key    *rsa.PrivateKey
	signer jose.Signer
}

func newRealm(t *testing.T) *fakeRealm {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	r := &fakeRealm{key: key}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer": r.srv.URL, "jwks_uri": r.srv.URL + "/jwks",
			"authorization_endpoint": r.srv.URL + "/auth", "token_endpoint": r.srv.URL + "/token",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "k1", Algorithm: "RS256", Use: "sig"}}})
	})
	r.srv = httptest.NewServer(mux)
	t.Cleanup(r.srv.Close)
	r.signer, err = jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key}, (&jose.SignerOptions{}).WithHeader("kid", "k1"))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func (r *fakeRealm) token(t *testing.T, claims map[string]any) string {
	t.Helper()
	base := map[string]any{"iss": r.srv.URL, "aud": Audience, "sub": "u-1", "exp": time.Now().Add(time.Minute).Unix(), "iat": time.Now().Unix()}
	for k, v := range claims {
		base[k] = v
	}
	s, err := jwt.Signed(r.signer).Claims(base).Serialize()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

type issuers map[string]uuid.UUID

func (i issuers) TenantForIssuer(_ context.Context, iss string) (uuid.UUID, error) {
	if id, ok := i[iss]; ok {
		return id, nil
	}
	return uuid.Nil, errs.New(errs.NotFound, "not-found", "unknown issuer")
}

func TestOIDCVerify(t *testing.T) {
	alpha, beta := newRealm(t), newRealm(t)
	tenantA, tenantB := uuid.New(), uuid.New()
	o := NewOIDC(issuers{alpha.srv.URL: tenantA, beta.srv.URL: tenantB})
	ctx := context.Background()

	p, err := o.Verify(ctx, alpha.token(t, map[string]any{"tenant_id": tenantA.String(), "roles": []string{"analyst"},
		"email": "a@x", "given_name": "Eszter", "family_name": "Varga"}))
	if err != nil || p.Tenant != tenantA || p.Email != "a@x" || len(p.Roles) != 1 {
		t.Fatalf("valid token: %+v %v", p, err)
	}
	if p.Subject != "u-1" || p.FirstName != "Eszter" || p.LastName != "Varga" {
		t.Errorf("who the token names: %+v", p)
	}
	if p, err := o.Verify(ctx, beta.token(t, nil)); err != nil || p.Tenant != tenantB {
		t.Fatalf("second realm: %+v %v", p, err)
	}

	unknown := newRealm(t)
	cases := map[string]string{
		"unknown issuer":        unknown.token(t, nil),
		"wrong audience":        alpha.token(t, map[string]any{"aud": "other"}),
		"expired":               alpha.token(t, map[string]any{"exp": time.Now().Add(-time.Minute).Unix()}),
		"tenant claim mismatch": alpha.token(t, map[string]any{"tenant_id": tenantB.String()}),
		"garbage":               "a.b.c",
		"no subject":            alpha.token(t, map[string]any{"sub": ""}),
	}
	for name, tok := range cases {
		if _, err := o.Verify(ctx, tok); !errors.Is(err, ErrUnauthenticated) {
			t.Errorf("%s: err = %v, want ErrUnauthenticated", name, err)
		}
	}
	// A token signed by another realm's key but claiming alpha's issuer fails signature verification.
	forged := beta.token(t, map[string]any{"iss": alpha.srv.URL})
	if _, err := o.Verify(ctx, forged); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("forged issuer: err = %v", err)
	}
}

func TestBearerAcceptsBothCredentialKinds(t *testing.T) {
	realm := newRealm(t)
	tenantA, tenantB := uuid.New(), uuid.New()
	o := NewOIDC(issuers{realm.srv.URL: tenantA})
	var gotTenant uuid.UUID
	var gotPrincipal *Principal
	h := Bearer(fakeResolver{"tk_key_b": tenantB}, o)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotTenant, _ = tenant.IDFrom(r.Context())
		if p, ok := PrincipalFrom(r.Context()); ok {
			gotPrincipal = &p
		}
	}))
	call := func(header string) {
		gotTenant, gotPrincipal = uuid.Nil, nil
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", header)
		h.ServeHTTP(httptest.NewRecorder(), req)
	}
	call("Bearer tk_key_b")
	if gotTenant != tenantB || gotPrincipal != nil {
		t.Errorf("tenant key: tenant %v principal %v", gotTenant, gotPrincipal)
	}
	call("Bearer " + realm.token(t, map[string]any{"sub": "analyst-1"}))
	if gotTenant != tenantA || gotPrincipal == nil || gotPrincipal.Subject != "analyst-1" {
		t.Errorf("oidc token: tenant %v principal %+v", gotTenant, gotPrincipal)
	}
	call("Bearer " + realm.token(t, map[string]any{"aud": "nope"}))
	if gotTenant != uuid.Nil {
		t.Errorf("bad token still resolved tenant %v", gotTenant)
	}
}
