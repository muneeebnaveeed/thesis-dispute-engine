package auth

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/errs"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/principal"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/tenant"
)

type fakeResolver map[string]uuid.UUID

// fakeKeyID derives a stable key id from the secret, so tests can assert the principal without a table.
func fakeKeyID(key string) uuid.UUID { return uuid.NewSHA1(uuid.Nil, []byte(key)) }

func (f fakeResolver) KeyForHash(_ context.Context, hash []byte) (KeyIdentity, error) {
	for key, id := range f {
		if bytes.Equal(HashKey(key), hash) {
			return KeyIdentity{Tenant: id, ID: fakeKeyID(key), Prefix: Prefix(key)}, nil
		}
	}
	return KeyIdentity{}, errs.New(errs.NotFound, "not-found", "no such key")
}

func TestBearer(t *testing.T) {
	want := uuid.New()
	var gotTenant uuid.UUID
	var gotErr error
	h := Bearer(fakeResolver{"good-key": want}, nil)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotTenant, _ = tenant.IDFrom(r.Context())
		gotErr = Required(r.Context(), "bearerAuth")
	}))
	cases := map[string]struct {
		header     string
		wantTenant uuid.UUID
		wantErr    bool
	}{
		"valid":        {"Bearer good-key", want, false},
		"missing":      {"", uuid.Nil, true},
		"wrong scheme": {"Basic good-key", uuid.Nil, true},
		"unknown key":  {"Bearer other", uuid.Nil, true},
		"empty bearer": {"Bearer ", uuid.Nil, true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			gotTenant, gotErr = uuid.Nil, nil
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			h.ServeHTTP(httptest.NewRecorder(), req)
			if gotTenant != tc.wantTenant {
				t.Errorf("tenant = %v, want %v", gotTenant, tc.wantTenant)
			}
			if tc.wantErr != (gotErr != nil) || (gotErr != nil && !errors.Is(gotErr, ErrUnauthenticated)) {
				t.Errorf("Required() = %v", gotErr)
			}
		})
	}
}

func TestBearerSetsKeyPrincipal(t *testing.T) {
	var got principal.Principal
	h := Bearer(fakeResolver{"tk_abcdefghijk": uuid.New()}, nil)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got, _ = principal.From(r.Context())
	}))
	req := httptest.NewRequest(http.MethodGet, "/disputes", nil)
	req.Header.Set("Authorization", "Bearer tk_abcdefghijk")
	h.ServeHTTP(httptest.NewRecorder(), req)
	want := principal.Principal{Kind: principal.Key, ID: fakeKeyID("tk_abcdefghijk").String(), Display: "key:tk_abcdefghi"}
	if got != want {
		t.Fatalf("principal = %+v, want %+v", got, want)
	}
}
