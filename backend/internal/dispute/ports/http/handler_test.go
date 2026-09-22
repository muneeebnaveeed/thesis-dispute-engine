package disputehttp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application/apptest"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
	disputepg "github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/infrastructure/postgres"
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
	directory := memTenants{{ID: apptest.TenantA, Slug: "otp", Name: "OTP Bank", Issuer: "http://kc/realms/otp", EmailDomains: []string{"otpbank.hu"}}}
	if err := disputehttp.Mount(mux, svc, ready, disputehttp.WithSessions(memSessions{}), disputehttp.WithTenants(directory), disputehttp.WithKeys(&memKeys{})); err != nil {
		t.Fatal(err)
	}
	keys := keyResolver{"key-a": apptest.TenantA, "key-b": apptest.TenantB}
	h := httpserver.Chain(mux, httpserver.RequestID, auth.Bearer(keys, nil), analystHeader, auth.ServiceKey("svc-secret"))
	return api{t: t, h: h, store: store}
}

// analystHeader stands in for an OIDC token in tests: "X-Test-Analyst: <tenant a|b>:<role,role>" becomes a principal.
func analystHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if v := r.Header.Get("X-Test-Analyst"); v != "" {
			who, roles, _ := strings.Cut(v, ":")
			tid := apptest.TenantA
			if who == "b" {
				tid = apptest.TenantB
			}
			p := auth.Principal{Tenant: tid, Subject: "analyst-" + who, Roles: strings.Split(roles, ",")}
			r = r.WithContext(auth.WithPrincipal(r.Context(), p))
		}
		next.ServeHTTP(w, r)
	})
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
	if created["regime"] != "EU_PSD2_CARD" || created["state"] != "INITIATED" || created["disputedAmount"] != "42.10" {
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
	if got["deadlines"] == nil || len(got["deadlines"].([]any)) != 2 || got["ledger"] == nil || got["balances"] == nil || got["reason"] != "UNAUTHORISED" {
		t.Errorf("clocks, ledger, balances and reason must be on every dispute: %v", got)
	}
	if risk, ok := got["risk"].(map[string]any); !ok || risk["tier"] != "LOW" || len(risk["signals"].([]any)) != 7 || got["notices"] == nil {
		t.Errorf("risk and notices must be on every dispute: %v %v", got["risk"], got["notices"])
	}

	// The questionnaire follows the reason; answers are validated per question with 422 invalid-answers.
	rec, sent := a.do(http.MethodPost, "/disputes/"+id+"/events", map[string]any{"event": "SEND_QUESTIONNAIRE"}, nil)
	if rec.Code != 200 || sent["questionnaire"] == nil || len(sent["questionnaire"].(map[string]any)["questions"].([]any)) != 7 {
		t.Fatalf("send questionnaire: %d %v", rec.Code, sent["questionnaire"])
	}
	rec, refused := a.do(http.MethodPost, "/disputes/"+id+"/events", map[string]any{"event": "RECEIVE_QUESTIONNAIRE", "payload": map[string]any{"answers": map[string]any{"noticed_on": "never"}}}, nil)
	if rec.Code != 422 || refused["code"] != "invalid-answers" || len(refused["errors"].([]any)) != 6 {
		t.Fatalf("bad answers: %d %v", rec.Code, refused)
	}
	answers := map[string]any{"recognise_merchant": "no", "card_in_possession": "yes", "shared_credentials": "no",
		"prior_disputes_merchant": "no", "noticed_on": "2026-09-20", "police_report": "no"}
	rec, received := a.do(http.MethodPost, "/disputes/"+id+"/events", map[string]any{"event": "RECEIVE_QUESTIONNAIRE", "payload": map[string]any{"answers": answers}}, nil)
	if rec.Code != 200 || received["state"] != "QUESTIONNAIRE_RECEIVED" || received["questionnaire"].(map[string]any)["receivedAt"] == nil {
		t.Fatalf("answers: %d %v", rec.Code, received)
	}

	// A refund reaches the ledger with the core's answer; the write-off on close is internal and carries none.
	rec, refunded := a.do(http.MethodPost, "/disputes/"+id+"/events", map[string]any{"event": "ISSUE_REFUND", "payload": map[string]any{"liability": "2.10"}}, nil)
	if rec.Code != 200 {
		t.Fatalf("refund: %d %s", rec.Code, rec.Body.String())
	}
	ledger := refunded["ledger"].([]any)
	credit := ledger[0].(map[string]any)
	if credit["kind"] != "FAST_REFUND" || credit["amount"] != "40.00" || credit["debit"] != "SUSPENSE" || credit["credit"] != "CUSTOMER" {
		t.Errorf("credit = %v", credit)
	}
	if core, ok := credit["core"].(map[string]any); !ok || core["responseCode"] != "00" {
		t.Errorf("credit core receipt = %v", credit["core"])
	}
	if refunded["balances"].(map[string]any)["suspense"] != "40.00" {
		t.Errorf("balances = %v", refunded["balances"])
	}
	rec, closed := a.do(http.MethodPost, "/disputes/"+id+"/events", map[string]any{"event": "CLOSE"}, nil)
	if rec.Code != 200 {
		t.Fatalf("close: %d %s", rec.Code, rec.Body.String())
	}
	off := closed["ledger"].([]any)[1].(map[string]any)
	if off["kind"] != "WRITE_OFF" || off["core"] != nil || closed["balances"].(map[string]any)["suspense"] != "0.00" {
		t.Errorf("after close: %v %v", off, closed["balances"])
	}

	// The ledger's refusals are 422s naming the payload field.
	rec, problem := a.do(http.MethodPost, "/disputes/"+txnDispute(t, a)+"/events", map[string]any{"event": "ISSUE_REFUND", "payload": map[string]any{"liability": "99"}}, nil)
	if rec.Code != 422 || problem["code"] != "invalid-liability" {
		t.Fatalf("over-cap liability: %d %v", rec.Code, problem)
	}
	if errs := problem["errors"].([]any); len(errs) != 1 || errs[0].(map[string]any)["field"] != "body.payload.liability" {
		t.Errorf("field errors = %v", problem["errors"])
	}
}

// txnDispute opens a fresh dispute in INVESTIGATING and returns its id.
func txnDispute(t *testing.T, a api) string {
	t.Helper()
	txn := a.store.AddTransaction(domain.RailCard, "EUR", "EUR", "42.10")
	_, created := a.do(http.MethodPost, "/disputes", map[string]any{"transactionId": txn.String()}, nil)
	id := created["id"].(string)
	if rec, _ := a.do(http.MethodPost, "/disputes/"+id+"/events", map[string]any{"event": "OPEN_INVESTIGATION"}, nil); rec.Code != 200 {
		t.Fatalf("open investigation: %d", rec.Code)
	}
	return id
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
func (m memSessions) DeleteMatching(_ context.Context, sel websession.Selector) (int64, error) {
	var n int64
	for id, b := range m {
		if (sel.Sid != "" && b.Sid == sel.Sid) || (sel.TenantID != nil && b.TenantID != nil && *b.TenantID == *sel.TenantID && (sel.Subject == "" || b.Subject == sel.Subject)) {
			delete(m, id)
			n++
		}
	}
	return n, nil
}

type memTenants []disputepg.TenantSummary

func (m memTenants) ActiveTenants(context.Context) ([]disputepg.TenantSummary, error) { return m, nil }

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

func TestInternalBulkSessionDeleteAndTenantDirectory(t *testing.T) {
	a := newAPI(t, func(context.Context) error { return nil })
	svc := map[string]string{"Authorization": "", "X-Service-Key": "svc-secret"}
	put := func(id, sid, subject string) {
		body := map[string]any{"ciphertext": "aGVsbG8=", "expiresAt": time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
			"tenantId": apptest.TenantA.String(), "sid": sid, "subject": subject}
		if rec, _ := a.do(http.MethodPut, "/internal/sessions/"+id, body, svc); rec.Code != http.StatusNoContent {
			t.Fatalf("put %s: %d", id, rec.Code)
		}
	}
	put(uuid.NewString(), "sid-1", "alice")
	put(uuid.NewString(), "sid-2", "alice")
	put(uuid.NewString(), "sid-3", "bob")

	rec, out := a.do(http.MethodDelete, "/internal/sessions?sid=sid-1", nil, svc)
	if rec.Code != http.StatusOK || out["deleted"] != float64(1) {
		t.Fatalf("by sid: %d %v", rec.Code, out)
	}
	rec, out = a.do(http.MethodDelete, "/internal/sessions?tenantId="+apptest.TenantA.String()+"&subject=alice", nil, svc)
	if rec.Code != http.StatusOK || out["deleted"] != float64(1) {
		t.Fatalf("by subject: %d %v", rec.Code, out)
	}
	if rec, _ := a.do(http.MethodDelete, "/internal/sessions", nil, svc); rec.Code != http.StatusBadRequest {
		t.Errorf("no selector: %d", rec.Code)
	}
	rec, _ = a.do(http.MethodDelete, "/internal/sessions?sid=x", nil, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("tenant key on bulk delete: %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/internal/tenants", nil)
	req.Header.Set("X-Service-Key", "svc-secret")
	a.h.ServeHTTP(rec, req)
	var tenants []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &tenants); err != nil || rec.Code != http.StatusOK || len(tenants) != 1 || tenants[0]["slug"] != "otp" {
		t.Fatalf("tenants: %d %s", rec.Code, rec.Body.String())
	}
}

// memKeys is the key manager's contract in memory: tenant-scoped, secret returned once, revoke is idempotent.
type memKeys struct {
	mu   sync.Mutex
	recs map[uuid.UUID]struct {
		tenant uuid.UUID
		rec    disputepg.KeyRecord
	}
}

func (m *memKeys) ListForTenant(_ context.Context, tenantID uuid.UUID) ([]disputepg.KeyRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []disputepg.KeyRecord
	for _, e := range m.recs {
		if e.tenant == tenantID {
			out = append(out, e.rec)
		}
	}
	return out, nil
}

func (m *memKeys) Issue(_ context.Context, tenantID uuid.UUID, label string, expiresAt *time.Time) (disputepg.KeyRecord, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.recs == nil {
		m.recs = map[uuid.UUID]struct {
			tenant uuid.UUID
			rec    disputepg.KeyRecord
		}{}
	}
	secret, _ := auth.NewSecret()
	rec := disputepg.KeyRecord{ID: uuid.New(), Prefix: auth.Prefix(secret), Label: label, CreatedAt: time.Now(), ExpiresAt: expiresAt}
	m.recs[rec.ID] = struct {
		tenant uuid.UUID
		rec    disputepg.KeyRecord
	}{tenantID, rec}
	return rec, secret, nil
}

func (m *memKeys) Revoke(_ context.Context, tenantID, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.recs[id]
	if !ok || e.tenant != tenantID {
		return application.ErrNotFound
	}
	now := time.Now()
	e.rec.RevokedAt = &now
	m.recs[id] = e
	return nil
}

func TestTenantKeySelfService(t *testing.T) {
	a := newAPI(t, func(context.Context) error { return nil })
	admin := map[string]string{"Authorization": "", "X-Test-Analyst": "a:analyst,tenant-admin"}
	analyst := map[string]string{"Authorization": "", "X-Test-Analyst": "a:analyst"}
	adminB := map[string]string{"Authorization": "", "X-Test-Analyst": "b:tenant-admin"}

	// A tenant key is a machine credential and cannot mint keys; an analyst without the role cannot either.
	if rec, p := a.do(http.MethodGet, "/tenant-keys", nil, nil); rec.Code != http.StatusForbidden || p["code"] != "forbidden" {
		t.Fatalf("tenant key on /tenant-keys: %d %v", rec.Code, p)
	}
	if rec, _ := a.do(http.MethodPost, "/tenant-keys", map[string]any{"label": "x"}, analyst); rec.Code != http.StatusForbidden {
		t.Fatalf("analyst without role: %d", rec.Code)
	}

	rec, issued := a.do(http.MethodPost, "/tenant-keys", map[string]any{"label": "core banking"}, admin)
	if rec.Code != http.StatusCreated || issued["secret"] == nil || issued["status"] != "live" {
		t.Fatalf("issue: %d %v", rec.Code, issued)
	}
	secret, _ := issued["secret"].(string)
	prefix, _ := issued["prefix"].(string)
	if !strings.HasPrefix(secret, "tk_") || !strings.HasPrefix(secret, prefix) {
		t.Errorf("secret %q should start with tk_ and its prefix %q", secret, prefix)
	}
	if rec, _ := a.do(http.MethodPost, "/tenant-keys", map[string]any{"label": "old", "expiresAt": "2020-01-01T00:00:00Z"}, admin); rec.Code != http.StatusBadRequest {
		t.Errorf("past expiry: %d", rec.Code)
	}
	if rec, _ := a.do(http.MethodPost, "/tenant-keys", map[string]any{}, admin); rec.Code != http.StatusBadRequest {
		t.Errorf("missing label: %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/tenant-keys", nil)
	req.Header.Set("X-Test-Analyst", "a:tenant-admin")
	a.h.ServeHTTP(rec, req)
	var list []map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if rec.Code != http.StatusOK || len(list) != 1 || list[0]["secret"] != nil || list[0]["prefix"] != prefix {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}

	id, _ := issued["id"].(string)
	if rec, _ := a.do(http.MethodDelete, "/tenant-keys/"+id, nil, adminB); rec.Code != http.StatusNotFound {
		t.Errorf("another tenant's admin revoking: %d, want 404 (ids must not leak)", rec.Code)
	}
	if rec, _ := a.do(http.MethodDelete, "/tenant-keys/"+id, nil, admin); rec.Code != http.StatusNoContent {
		t.Errorf("revoke: %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	a.h.ServeHTTP(rec, req)
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if list[0]["status"] != "revoked" {
		t.Errorf("after revoke: %v", list[0])
	}
	if rec, _ := a.do(http.MethodDelete, "/tenant-keys/"+uuid.NewString(), nil, admin); rec.Code != http.StatusNotFound {
		t.Errorf("unknown key: %d", rec.Code)
	}
}

func TestListDisputesPagesNewestFirstWithinTheTenant(t *testing.T) {
	a := newAPI(t, func(context.Context) error { return nil })
	txn := a.store.AddTransaction(domain.RailCard, "EUR", "EUR", "10.00")
	foreign := a.store.AddTransactionFor(apptest.TenantB, domain.RailCard, "EUR", "EUR", "10.00")
	ids := make([]string, 0, 5)
	for range 5 {
		rec, d := a.do(http.MethodPost, "/disputes", map[string]any{"transactionId": txn.String()}, nil)
		if rec.Code != http.StatusCreated {
			t.Fatal(rec.Code)
		}
		ids = append(ids, d["id"].(string))
	}
	if rec, _ := a.do(http.MethodPost, "/disputes", map[string]any{"transactionId": foreign.String()}, map[string]string{"Authorization": "Bearer key-b"}); rec.Code != http.StatusCreated {
		t.Fatal(rec.Code)
	}

	page := func(query string) (map[string]any, []string) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/disputes"+query, nil)
		req.Header.Set("Authorization", "Bearer key-a")
		a.h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", query, rec.Code, rec.Body.String())
		}
		var body map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		items := body["items"].([]any)
		got := make([]string, 0, len(items))
		for _, it := range items {
			got = append(got, it.(map[string]any)["id"].(string))
		}
		return body, got
	}
	first, got := page("?limit=2")
	if len(got) != 2 || first["nextCursor"] == nil {
		t.Fatalf("first page: %v", first)
	}
	second, got2 := page("?limit=2&cursor=" + first["nextCursor"].(string))
	third, got3 := page("?limit=2&cursor=" + second["nextCursor"].(string))
	all := append(append(got, got2...), got3...)
	if len(all) != 5 || third["nextCursor"] != nil {
		t.Fatalf("pages: %v %v %v", got, got2, got3)
	}
	seen := map[string]bool{}
	for _, id := range all {
		if seen[id] {
			t.Errorf("duplicate across pages: %s", id)
		}
		seen[id] = true
	}
	// Newest first: the last created id comes first; and none of the pages leak tenant B's dispute.
	if all[0] != ids[4] {
		t.Errorf("order: first is %s, want newest %s", all[0], ids[4])
	}
	if _, filtered := page("?state=CLOSED"); len(filtered) != 0 {
		t.Errorf("state filter: %v", filtered)
	}
	if _, filtered := page("?reason=UNAUTHORISED"); len(filtered) != 5 {
		t.Errorf("reason filter keeps the seeded reason: %v", filtered)
	}
	if _, filtered := page("?reason=DUPLICATE"); len(filtered) != 0 {
		t.Errorf("reason filter excludes the others: %v", filtered)
	}
	if rec, _ := a.do(http.MethodGet, "/disputes?reason=NONSENSE", nil, nil); rec.Code != http.StatusBadRequest {
		t.Errorf("reason outside the enum should fail validation: %d", rec.Code)
	}
	if rec, p := a.do(http.MethodGet, "/disputes?cursor=not-a-cursor", nil, nil); rec.Code != http.StatusBadRequest || p["code"] != "contract-violation" {
		t.Errorf("bad cursor: %d %v", rec.Code, p)
	}
	if rec, _ := a.do(http.MethodGet, "/disputes?limit=1000", nil, nil); rec.Code != http.StatusBadRequest {
		t.Errorf("limit above the maximum should fail validation: %d", rec.Code)
	}
}

func TestAnalystsComposeEmailsFromTemplatesAndTenantKeysCannot(t *testing.T) {
	a := newAPI(t, nil)
	id := txnDispute(t, a)
	analyst := map[string]string{"Authorization": "", "X-Test-Analyst": "a:analyst"}

	rec, cat := a.do(http.MethodGet, "/disputes/"+id+"/email-templates", nil, analyst)
	if rec.Code != 200 {
		t.Fatalf("templates: %d %s", rec.Code, rec.Body.String())
	}
	templates := cat["templates"].([]any)
	if len(templates) != 4 || cat["facts"].(map[string]any)["customer"] != "Test Holder" {
		t.Fatalf("catalogue = %v", cat)
	}
	var rfi map[string]any
	for _, x := range templates {
		if m := x.(map[string]any); m["kind"] == "REQUEST_FOR_INFORMATION" {
			rfi = m
		}
	}
	if rfi == nil || !strings.Contains(strings.Join(anyStrings(rfi["paragraphs"]), " "), "{{items}}") {
		t.Errorf("RFI template = %v", rfi)
	}

	// Incomplete form: one error per field, nothing queued.
	rec, problem := a.do(http.MethodPost, "/disputes/"+id+"/notices", map[string]any{"template": "REQUEST_FOR_INFORMATION", "fields": map[string]any{"days": "99"}}, analyst)
	if rec.Code != 422 || problem["code"] != "invalid-fields" || len(problem["errors"].([]any)) != 2 {
		t.Fatalf("bad form: %d %v", rec.Code, problem)
	}
	rec, sent := a.do(http.MethodPost, "/disputes/"+id+"/notices", map[string]any{"template": "REQUEST_FOR_INFORMATION",
		"fields": map[string]any{"items": "receipt,delivery", "days": "10"}}, analyst)
	if rec.Code != 201 {
		t.Fatalf("compose: %d %s", rec.Code, rec.Body.String())
	}
	notices := sent["notices"].([]any)
	last := notices[len(notices)-1].(map[string]any)
	if last["kind"] != "REQUEST_FOR_INFORMATION" || last["channel"] != "EMAIL" || last["actor"] != "analyst-a" || last["sentAt"] != nil {
		t.Errorf("queued notice = %v", last)
	}
	rec, doc := a.do(http.MethodGet, "/disputes/"+id+"/notices/"+strconv.FormatInt(int64(last["id"].(float64)), 10), nil, analyst)
	if rec.Code != 200 || !strings.Contains(strings.Join(anyStrings(doc["paragraphs"]), "\n"), "- the expected delivery date") {
		t.Errorf("document = %d %v", rec.Code, doc)
	}

	// A tenant key is a machine; it does not write to customers.
	rec, problem = a.do(http.MethodPost, "/disputes/"+id+"/notices", map[string]any{"template": "CUSTOM", "fields": map[string]any{"subject": "x", "body": "y"}}, nil)
	if rec.Code != 403 || problem["code"] != "forbidden" {
		t.Errorf("tenant key composing: %d %v", rec.Code, problem)
	}
	rec, problem = a.do(http.MethodPost, "/disputes/"+id+"/notices", map[string]any{"template": "REFUND", "fields": map[string]any{}}, analyst)
	if rec.Code != 400 || problem["code"] != "unknown-template" {
		t.Errorf("automatic kind by hand: %d %v", rec.Code, problem)
	}
}

func anyStrings(v any) []string {
	items, _ := v.([]any)
	out := make([]string, 0, len(items))
	for _, x := range items {
		s, _ := x.(string)
		out = append(out, s)
	}
	return out
}

func TestAttachmentsTravelWithComposedEmails(t *testing.T) {
	a := newAPI(t, nil)
	id := txnDispute(t, a)
	analyst := map[string]string{"Authorization": "", "X-Test-Analyst": "a:analyst"}

	upload := func(name, ctype string, content []byte) (*httptest.ResponseRecorder, map[string]any) {
		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		hdr := textproto.MIMEHeader{}
		hdr.Set("Content-Disposition", `form-data; name="file"; filename="`+name+`"`)
		hdr.Set("Content-Type", ctype)
		part, _ := w.CreatePart(hdr)
		_, _ = part.Write(content)
		_ = w.Close()
		req := httptest.NewRequest(http.MethodPost, "/disputes/"+id+"/attachments", &buf)
		req.Header.Set("Content-Type", w.FormDataContentType())
		req.Header.Set("X-Test-Analyst", "a:analyst")
		rec := httptest.NewRecorder()
		a.h.ServeHTTP(rec, req)
		var out map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return rec, out
	}
	rec, refused := upload("notes.txt", "text/plain", []byte("hello"))
	if rec.Code != 422 || refused["code"] != "attachment-refused" {
		t.Fatalf("text file: %d %v", rec.Code, refused)
	}
	rec, stored := upload("receipt.pdf", "application/pdf", []byte("%PDF-1.4 fake"))
	if rec.Code != 201 || stored["filename"] != "receipt.pdf" || stored["size"].(float64) != 13 {
		t.Fatalf("pdf: %d %v", rec.Code, stored)
	}
	attachmentID := stored["id"].(string)

	// Composing with an id that is not this dispute's draft is refused; with the draft it is claimed.
	rec, problem := a.do(http.MethodPost, "/disputes/"+id+"/notices", map[string]any{"template": "CUSTOM", "fields": map[string]any{"subject": "s", "body": "b"},
		"attachments": []string{uuid.NewString()}}, analyst)
	if rec.Code != 422 || problem["code"] != "attachment-unknown" {
		t.Fatalf("unknown attachment: %d %v", rec.Code, problem)
	}
	rec, sent := a.do(http.MethodPost, "/disputes/"+id+"/notices", map[string]any{"template": "CUSTOM", "fields": map[string]any{"subject": "s", "body": "b"},
		"attachments": []string{attachmentID}}, analyst)
	if rec.Code != 201 {
		t.Fatalf("compose with attachment: %d %s", rec.Code, rec.Body.String())
	}
	notices := sent["notices"].([]any)
	last := notices[len(notices)-1].(map[string]any)
	files := last["attachments"].([]any)
	if len(files) != 1 || files[0].(map[string]any)["filename"] != "receipt.pdf" {
		t.Errorf("attachments on the notice = %v", last["attachments"])
	}
	// The same draft cannot be attached twice.
	rec, problem = a.do(http.MethodPost, "/disputes/"+id+"/notices", map[string]any{"template": "CUSTOM", "fields": map[string]any{"subject": "s", "body": "b"},
		"attachments": []string{attachmentID}}, analyst)
	if rec.Code != 422 || problem["code"] != "attachment-unknown" {
		t.Errorf("reusing a claimed attachment: %d %v", rec.Code, problem)
	}

	// Download serves the bytes under the file's own type and name.
	req := httptest.NewRequest(http.MethodGet, "/disputes/"+id+"/attachments/"+attachmentID, nil)
	req.Header.Set("X-Test-Analyst", "a:analyst")
	rec = httptest.NewRecorder()
	a.h.ServeHTTP(rec, req)
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "application/pdf" || !strings.Contains(rec.Header().Get("Content-Disposition"), "receipt.pdf") || rec.Body.String() != "%PDF-1.4 fake" {
		t.Errorf("download: %d %v %q", rec.Code, rec.Header(), rec.Body.String())
	}
}

func TestTenantAdminsRewordTemplatesAndAnalystsSeeTheResult(t *testing.T) {
	a := newAPI(t, nil)
	id := txnDispute(t, a)
	admin := map[string]string{"Authorization": "", "X-Test-Analyst": "a:analyst,tenant-admin"}
	analyst := map[string]string{"Authorization": "", "X-Test-Analyst": "a:analyst"}

	if rec, _ := a.do(http.MethodGet, "/tenant-templates", nil, analyst); rec.Code != 403 {
		t.Fatalf("analyst listing settings: %d", rec.Code)
	}
	rec, _ := a.do(http.MethodGet, "/tenant-templates", nil, admin)
	if rec.Code != 200 {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}
	var settings []map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &settings)
	if len(settings) != 4 || settings[0]["override"] != nil {
		t.Fatalf("settings = %v", settings)
	}

	// A placeholder the form does not have is refused; a good override is stored and reported.
	rec, problem := a.do(http.MethodPut, "/tenant-templates/STATUS_UPDATE", map[string]any{"paragraphs": []string{"{{nothing}}"}}, admin)
	if rec.Code != 422 || problem["code"] != "invalid-template-override" {
		t.Fatalf("bad override: %d %v", rec.Code, problem)
	}
	rec, stored := a.do(http.MethodPut, "/tenant-templates/STATUS_UPDATE", map[string]any{
		"subject":     "Where your {{merchant}} dispute stands",
		"paragraphs":  []string{"{{stage}}", "{{note}}", "Reference {{dispute}}. Yours, {{bank}}."},
		"optionTexts": map[string]any{"stage.final": "We are close to a decision and will write within days."},
	}, admin)
	if rec.Code != 200 || stored["updatedBy"] != "analyst-a" {
		t.Fatalf("put: %d %v", rec.Code, stored["updatedBy"])
	}
	effective := stored["effective"].(map[string]any)
	if effective["subject"] != "Where your {{merchant}} dispute stands" {
		t.Errorf("effective = %v", effective)
	}

	// Analysts get the tenant's wording, with facts filled; the base label stays since it was not overridden.
	rec, cat := a.do(http.MethodGet, "/disputes/"+id+"/email-templates", nil, analyst)
	if rec.Code != 200 {
		t.Fatal(rec.Code)
	}
	var found bool
	for _, x := range cat["templates"].([]any) {
		m := x.(map[string]any)
		if m["kind"] != "STATUS_UPDATE" {
			continue
		}
		found = true
		if m["subject"] != "Where your ACME dispute stands" || m["label"] != "Status update" {
			t.Errorf("analyst view = %v", m)
		}
		for _, f := range m["fields"].([]any) {
			if fm := f.(map[string]any); fm["id"] == "stage" {
				for _, o := range fm["options"].([]any) {
					if om := o.(map[string]any); om["key"] == "final" && om["text"] != "We are close to a decision and will write within days." {
						t.Errorf("option text = %v", om)
					}
				}
			}
		}
	}
	if !found {
		t.Fatal("STATUS_UPDATE missing")
	}
	// And a composed email uses it.
	rec, sent := a.do(http.MethodPost, "/disputes/"+id+"/notices", map[string]any{"template": "STATUS_UPDATE", "fields": map[string]any{"stage": "final"}}, analyst)
	if rec.Code != 201 {
		t.Fatalf("compose: %d %s", rec.Code, rec.Body.String())
	}
	notices := sent["notices"].([]any)
	if last := notices[len(notices)-1].(map[string]any); last["subject"] != "Where your ACME dispute stands" {
		t.Errorf("sent subject = %v", last["subject"])
	}

	// Reverting brings the base back.
	if rec, _ := a.do(http.MethodDelete, "/tenant-templates/STATUS_UPDATE", nil, admin); rec.Code != 204 {
		t.Fatalf("delete: %d", rec.Code)
	}
	if rec, _ := a.do(http.MethodDelete, "/tenant-templates/STATUS_UPDATE", nil, admin); rec.Code != 404 {
		t.Errorf("delete again: %d", rec.Code)
	}
}
