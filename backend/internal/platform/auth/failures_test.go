package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/httpserver"
)

func TestFailureLimiter(t *testing.T) {
	limiter := NewFailureLimiter(3)
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	h := httpserver.Chain(inner, httpserver.RequestID, Bearer(fakeResolver{"tk_good": uuid.New()}, nil), limiter.Middleware)
	call := func(peer, header string) int {
		req := httptest.NewRequest(http.MethodGet, "/disputes", nil)
		req.RemoteAddr = peer + ":4000"
		if header != "" {
			req.Header.Set("Authorization", header)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	for range 3 {
		if code := call("198.51.100.7", "Bearer tk_wrong"); code != http.StatusTeapot {
			t.Fatalf("guess passed through to the handler with %d before the limit", code)
		}
	}
	if code := call("198.51.100.7", "Bearer tk_wrong"); code != http.StatusTooManyRequests {
		t.Fatalf("fourth guess: %d, want 429", code)
	}
	if code := call("198.51.100.7", "Bearer tk_good"); code != http.StatusTooManyRequests {
		t.Errorf("a blocked peer stays blocked even with a valid key: %d", code)
	}
	if code := call("203.0.113.2", "Bearer tk_wrong"); code != http.StatusTeapot {
		t.Errorf("another peer is unaffected: %d", code)
	}
	for range 10 {
		if code := call("203.0.113.9", ""); code != http.StatusTeapot {
			t.Errorf("anonymous requests are not counted: %d", code)
		}
	}
	if code := call("198.51.100.7", ""); code != http.StatusTeapot {
		t.Errorf("a blocked peer's credential-less requests (health, internal with service key) still pass: %d", code)
	}
}
