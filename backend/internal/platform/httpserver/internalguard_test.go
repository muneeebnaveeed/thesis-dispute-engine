package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestInternalOnly(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	h := InternalOnly("/internal/", ParseCIDRs("127.0.0.0/8, ::1/128, 10.0.0.0/8"))(inner)
	cases := map[string]struct {
		path, peer string
		want       int
	}{
		"loopback internal":     {"/internal/sessions/x", "127.0.0.1:5000", 204},
		"ipv6 loopback":         {"/internal/sessions/x", "[::1]:5000", 204},
		"private net":           {"/internal/tenants", "10.1.2.3:1", 204},
		"public peer internal":  {"/internal/tenants", "203.0.113.9:1", 404},
		"public peer public":    {"/disputes", "203.0.113.9:1", 204},
		"mapped ipv4 loopback":  {"/internal/x", "[::ffff:127.0.0.1]:1", 204},
		"unparseable remote":    {"/internal/x", "garbage", 404},
		"prefix must match dir": {"/internalish", "203.0.113.9:1", 204},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			req.RemoteAddr = tc.peer
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Errorf("%s from %s: %d, want %d", tc.path, tc.peer, rec.Code, tc.want)
			}
		})
	}
}
