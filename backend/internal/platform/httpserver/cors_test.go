package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCORS(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	h := CORS(ParseOrigins("http://localhost:3002, https://app.example"))(inner)

	req := httptest.NewRequest(http.MethodOptions, "/disputes", nil)
	req.Header.Set("Origin", "http://localhost:3002")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent || rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3002" {
		t.Fatalf("preflight: %d %v", rec.Code, rec.Header())
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got == "" || !contains(got, "Idempotency-Key") {
		t.Errorf("allow headers = %q", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/disputes", nil)
	req.Header.Set("Origin", "https://app.example")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTeapot || rec.Header().Get("Access-Control-Allow-Origin") != "https://app.example" {
		t.Errorf("allowed origin request: %d %q", rec.Code, rec.Header().Get("Access-Control-Allow-Origin"))
	}

	req = httptest.NewRequest(http.MethodOptions, "/disputes", nil)
	req.Header.Set("Origin", "https://evil.example")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Header().Get("Access-Control-Allow-Origin") != "" || rec.Code == http.StatusNoContent {
		t.Errorf("unknown origin got CORS headers or a preflight answer: %d", rec.Code)
	}
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }
