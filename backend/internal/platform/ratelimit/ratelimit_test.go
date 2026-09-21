package ratelimit

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/httpserver"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/tenant"
)

type memCounter map[string]int32

func (m memCounter) Bump(_ context.Context, id uuid.UUID, w time.Time) (int32, error) {
	k := id.String() + w.String()
	m[k]++
	return m[k], nil
}

func TestMiddleware(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	h := httpserver.Chain(inner, httpserver.RequestID, Middleware(memCounter{}, 3, slog.New(slog.NewTextHandler(io.Discard, nil))))
	a, b := uuid.New(), uuid.New()
	call := func(id uuid.UUID) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/disputes", nil)
		if id != uuid.Nil {
			req = req.WithContext(tenant.WithID(req.Context(), id))
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	for i := range 3 {
		if rec := call(a); rec.Code != http.StatusNoContent || rec.Header().Get("RateLimit-Remaining") == "" {
			t.Fatalf("request %d: %d %v", i, rec.Code, rec.Header())
		}
	}
	rec := call(a)
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("fourth request: %d %v", rec.Code, rec.Header())
	}
	var p map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &p)
	if p["code"] != "rate-limited" || p["retryable"] != true || p["retryAfterSeconds"] == nil {
		t.Errorf("problem: %v", p)
	}
	if rec := call(b); rec.Code != http.StatusNoContent {
		t.Errorf("another tenant is not affected: %d", rec.Code)
	}
	if rec := call(uuid.Nil); rec.Code != http.StatusNoContent {
		t.Errorf("anonymous request passes through: %d", rec.Code)
	}
}
