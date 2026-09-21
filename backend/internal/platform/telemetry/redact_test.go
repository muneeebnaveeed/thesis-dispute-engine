package telemetry

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestRedactDropsSensitiveKeysAtAnyDepth(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(Redact(slog.NewJSONHandler(&buf, nil))).With("Authorization", "Bearer abc")
	logger.Info("req", "user", "u1", "password", "hunter2",
		slog.Group("card", "pan", "4111111111111111", "last4", "1111"),
		slog.Group("http", slog.Group("headers", "cookie", "sid=1")))
	out := buf.String()
	for _, leaked := range []string{"Bearer abc", "hunter2", "4111111111111111", "sid=1"} {
		if strings.Contains(out, leaked) {
			t.Errorf("leaked %q in %s", leaked, out)
		}
	}
	for _, kept := range []string{`"user":"u1"`, `"last4":"1111"`, `"msg":"req"`, "[REDACTED]"} {
		if !strings.Contains(out, kept) {
			t.Errorf("missing %q in %s", kept, out)
		}
	}
}
