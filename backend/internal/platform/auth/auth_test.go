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
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/tenant"
)

type fakeResolver map[string]uuid.UUID

func (f fakeResolver) TenantForKeyHash(_ context.Context, hash []byte) (uuid.UUID, error) {
	for key, id := range f {
		if bytes.Equal(HashKey(key), hash) {
			return id, nil
		}
	}
	return uuid.Nil, errs.New(errs.NotFound, "not-found", "no such key")
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
