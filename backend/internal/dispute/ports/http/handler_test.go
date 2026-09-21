package disputehttp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application/apptest"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
	disputehttp "github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/ports/http"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/auth"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/httpserver"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/websession"
)

type api struct {
	t     *testing.T
	h     http.Handler
	store *apptest.MemStore
}

func newAPI(t *testing.T, ready disputehttp.Readiness) api {
	t.Helper()
	store := apptest.NewMemStore()
	svc, err := application.NewService(store, nil)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	if err := disputehttp.Mount(mux, svc, ready, disputehttp.WithSessions(memSessions{})); err != nil {
		t.Fatal(err)
	}
	keys := keyResolver{"key-a": apptest.TenantA, "key-b": apptest.TenantB}
	h := httpserver.Chain(mux, httpserver.RequestID, auth.Bearer(keys, nil), auth.ServiceKey("svc-secret"))
	return api{t: t, h: h, store: store}
}

// keyResolver stands in for the tenant_keys table; the handler tests care about the contract, not the lookup.
type keyResolver map[string]uuid.UUID

func (k keyResolver) TenantForKeyHash(_ context.Context, hash []byte) (uuid.UUID, error) {
	for key, id := range k {
		if bytes.Equal(auth.HashKey(key), hash) {
			return id, nil
		}
	}
	return uuid.Nil, application.ErrNotFound
}

func (a api) do(method, path string, body any, headers map[string]string) (*httptest.ResponseRecorder, map[string]any) {
	a.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	// Tenant A by default; a test passes its own Authorization (possibly empty) to change that.
	req.Header.Set("Authorization", "Bearer key-a")
	for k, v := range headers {
		if v == "" {
			req.Header.Del(k)
			continue
		}
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	a.h.ServeHTTP(rec, req)
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec, out
}

func TestReadyz(t *testing.T) {
	ok := newAPI(t, func(context.Context) error { return nil })
	rec, body := ok.do(http.MethodGet, "/readyz", nil, nil)
	if rec.Code != 200 || body["status"] != "ok" {
		t.Errorf("ready: %d %v", rec.Code, body)
	}
	down := newAPI(t, func(context.Context) error { return errors.New("dial tcp: refused") })
	rec, body = down.do(http.MethodGet, "/readyz", nil, nil)
	if rec.Code != 503 || body["status"] != "unavailable" || strings.Contains(rec.Body.String(), "dial tcp") {
		t.Errorf("not ready: %d %s", rec.Code, rec.Body.String())
	}
}

func TestCreateApplyGetRoundTrip(t *testing.T) {
	a := newAPI(t, nil)
	txn := a.store.AddTransaction(domain.RailCard, "EUR", "EUR", "42.10")

	rec, created := a.do(http.MethodPost, "/disputes", map[string]any{"transactionId": txn.String()}, nil)
	if rec.Code != 201 {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	if created["regime"] != "EU_PSD2_CARD" || created["state"] != "INITIATED" || created["disputedAmount"] != "42.1000" {
		t.Errorf("created = %v", created)
	}
	id := created["id"].(string)

	rec, applied := a.do(http.MethodPost, "/disputes/"+id+"/events", map[string]any{"event": "OPEN_INVESTIGATION", "actor": "analyst:7", "payload": map[string]any{"note": "looks odd"}}, nil)
	if rec.Code != 200 || applied["state"] != "INVESTIGATING" || applied["version"].(float64) != 2 {
		t.Fatalf("apply: %d %v", rec.Code, applied)
	}
	events := applied["events"].([]any)
	last := events[1].(map[string]any)
	if last["actor"] != "analyst:7" || last["payload"].(map[string]any)["note"] != "looks odd" {
		t.Errorf("last event = %v", last)
	}

	rec, got := a.do(http.MethodGet, "/disputes/"+id, nil, nil)
	if rec.Code != 200 || got["state"] != "INVESTIGATING" || len(got["events"].([]any)) != 2 {
		t.Errorf("get: %d %v", rec.Code, got)
	}
}

func TestInvalidTransitionIs409WithAllowedEvents(t *testing.T) {
	a := newAPI(t, nil)
	txn := a.store.AddTransaction(domain.RailCard, "USD", "USD", "10")
	_, created := a.do(http.MethodPost, "/disputes", map[string]any{"transactionId": txn.String()}, nil)
	id := created["id"].(string)

	rec, body := a.do(http.MethodPost, "/disputes/"+id+"/events", map[string]any{"event": "WIN_CHARGEBACK"}, nil)
	if rec.Code != 409 {
		t.Fatalf("status = %d %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/problem+json") {
		t.Errorf("content-type = %s", ct)
	}
	if body["code"] != "invalid-transition" || body["retryable"] != false || body["requestId"] == "" {
		t.Errorf("problem = %v", body)
	}
	allowed, _ := body["allowedEvents"].([]any)
	if len(allowed) == 0 {
		t.Errorf("allowedEvents missing: %v", body)
	}
	if detail, _ := body["detail"].(string); strings.Contains(detail, "under US_REG_E") {
		t.Errorf("detail leaked wrap context: %s", detail)
	}
}

func TestContractViolationsAre400(t *testing.T) {
	a := newAPI(t, nil)
	rec, body := a.do(http.MethodPost, "/disputes", map[string]any{"transactionId": "not-a-uuid"}, nil)
	if rec.Code != 400 || body["code"] != "contract-violation" {
		t.Errorf("bad uuid: %d %v", rec.Code, body)
	}
	if fields, _ := body["errors"].([]any); len(fields) == 0 || fields[0].(map[string]any)["field"] != "/transactionId" {
		t.Errorf("field errors = %v", body["errors"])
	}
	rec, body = a.do(http.MethodPost, "/disputes", map[string]any{}, nil)
	if rec.Code != 400 || body["code"] != "contract-violation" {
		t.Errorf("missing field: %d %v", rec.Code, body)
	}
	txn := a.store.AddTransaction(domain.RailCard, "USD", "USD", "10")
	_, created := a.do(http.MethodPost, "/disputes", map[string]any{"transactionId": txn.String()}, nil)
	rec, body = a.do(http.MethodPost, "/disputes/"+created["id"].(string)+"/events", map[string]any{"event": "TELEPORT"}, nil)
	if rec.Code != 400 || body["code"] != "contract-violation" {
		t.Errorf("unknown event: %d %v", rec.Code, body)
	}
}

func TestIdempotencyKeyReplayAndReuse(t *testing.T) {
	a := newAPI(t, nil)
	txn := a.store.AddTransaction(domain.RailSEPADD, "EUR", "EUR", "99")
	req := map[string]any{"transactionId": txn.String()}
	hdr := map[string]string{"Idempotency-Key": "order-1"}

	rec1, first := a.do(http.MethodPost, "/disputes", req, hdr)
	rec2, second := a.do(http.MethodPost, "/disputes", req, hdr)
	if rec1.Code != 201 || rec2.Code != 201 || first["id"] != second["id"] {
		t.Fatalf("replay: %d/%d ids %v/%v", rec1.Code, rec2.Code, first["id"], second["id"])
	}
	if len(a.store.Disputes) != 1 {
		t.Errorf("disputes = %d", len(a.store.Disputes))
	}
	other := a.store.AddTransaction(domain.RailSEPADD, "EUR", "EUR", "1")
	rec3, body := a.do(http.MethodPost, "/disputes", map[string]any{"transactionId": other.String()}, hdr)
	if rec3.Code != 422 || body["code"] != "idempotency-key-reuse" {
		t.Errorf("reuse: %d %v", rec3.Code, body)
	}
}

func TestUnknownDisputeIs404AndSpecIsServed(t *testing.T) {
	a := newAPI(t, nil)
	rec, body := a.do(http.MethodGet, "/disputes/018f3a6e-0000-7000-8000-000000000000", nil, nil)
	if rec.Code != 404 || body["code"] != "not-found" {
		t.Errorf("404: %d %v", rec.Code, body)
	}
	rec, spec := a.do(http.MethodGet, "/openapi.json", nil, nil)
	if rec.Code != 200 || spec["openapi"] == nil {
		t.Errorf("spec: %d", rec.Code)
	}
}

func TestAuthenticationFollowsTheSpec(t *testing.T) {
	a := newAPI(t, func(context.Context) error { return nil })
	txn := a.store.AddTransaction(domain.RailCard, "EUR", "EUR", "10.00")
	body := map[string]any{"transactionId": txn.String()}

	// Health endpoints opt out of security in the spec; they need no key.
	if rec, _ := a.do(http.MethodGet, "/healthz", nil, map[string]string{"Authorization": ""}); rec.Code != http.StatusOK {
		t.Errorf("healthz without key: %d", rec.Code)
	}
	// Dispute operations require one; missing, wrong scheme and unknown keys all read the same to the client.
	for name, header := range map[string]string{"missing": "", "basic": "Basic key-a", "unknown": "Bearer nope"} {
		rec, problem := a.do(http.MethodPost, "/disputes", body, map[string]string{"Authorization": header})
		if rec.Code != http.StatusUnauthorized || problem["code"] != "unauthenticated" || problem["retryable"] != false {
			t.Errorf("%s: %d %v", name, rec.Code, problem)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
			t.Errorf("%s: content type %q", name, ct)
		}
	}
	// Authentication is checked before the body: an unauthenticated request with a broken body is still a 401.
	if rec, _ := a.do(http.MethodPost, "/disputes", map[string]any{"transactionId": "nope"}, map[string]string{"Authorization": ""}); rec.Code != http.StatusUnauthorized {
		t.Errorf("unauthenticated bad body: %d", rec.Code)
	}

	// A valid key for another tenant sees nothing of this one.
	rec, created := a.do(http.MethodPost, "/disputes", body, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %v", rec.Code, created)
	}
	if rec, _ := a.do(http.MethodGet, "/disputes/"+created["id"].(string), nil, map[string]string{"Authorization": "Bearer key-b"}); rec.Code != http.StatusNotFound {
		t.Errorf("cross-tenant get: %d", rec.Code)
	}
}

// memSessions is enough to exercise the internal routes' contract and authentication.
type memSessions map[uuid.UUID]websession.Blob

func (m memSessions) Put(_ context.Context, id uuid.UUID, b websession.Blob) error {
	m[id] = b
	return nil
}
func (m memSessions) Get(_ context.Context, id uuid.UUID) (websession.Blob, error) {
	b, ok := m[id]
	if !ok {
		return websession.Blob{}, application.ErrNotFound
	}
	return b, nil
}
func (m memSessions) Delete(_ context.Context, id uuid.UUID) error { delete(m, id); return nil }

func TestInternalSessionsRequireTheServiceKey(t *testing.T) {
	a := newAPI(t, func(context.Context) error { return nil })
	id := uuid.NewString()
	blob := map[string]any{"ciphertext": "aGVsbG8=", "expiresAt": time.Now().Add(time.Hour).UTC().Format(time.RFC3339)}

	// A tenant credential is not a service credential: the spec lists only serviceKey for these operations.
	if rec, p := a.do(http.MethodPut, "/internal/sessions/"+id, blob, nil); rec.Code != http.StatusUnauthorized || p["code"] != "unauthenticated" {
		t.Fatalf("tenant key on internal route: %d %v", rec.Code, p)
	}
	svc := map[string]string{"Authorization": "", "X-Service-Key": "svc-secret"}
	if rec, _ := a.do(http.MethodPut, "/internal/sessions/"+id, blob, svc); rec.Code != http.StatusNoContent {
		t.Fatalf("put: %d", rec.Code)
	}
	rec, got := a.do(http.MethodGet, "/internal/sessions/"+id, nil, svc)
	if rec.Code != http.StatusOK || got["ciphertext"] != "aGVsbG8=" {
		t.Fatalf("get: %d %v", rec.Code, got)
	}
	if rec, _ := a.do(http.MethodGet, "/internal/sessions/"+id, nil, map[string]string{"Authorization": "", "X-Service-Key": "wrong"}); rec.Code != http.StatusUnauthorized {
		t.Errorf("wrong service key: %d", rec.Code)
	}
	if rec, _ := a.do(http.MethodDelete, "/internal/sessions/"+id, nil, svc); rec.Code != http.StatusNoContent {
		t.Errorf("delete: %d", rec.Code)
	}
	if rec, _ := a.do(http.MethodGet, "/internal/sessions/"+id, nil, svc); rec.Code != http.StatusNotFound {
		t.Errorf("after delete: %d", rec.Code)
	}
	// The service key opens only the internal routes; dispute routes still need a tenant.
	if rec, _ := a.do(http.MethodGet, "/disputes/"+id, nil, svc); rec.Code != http.StatusUnauthorized {
		t.Errorf("service key on a tenant route: %d", rec.Code)
	}
}
