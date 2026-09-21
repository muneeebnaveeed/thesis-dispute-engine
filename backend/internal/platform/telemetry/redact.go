package telemetry

import (
	"context"
	"log/slog"
	"strings"
)

// Sensitive keys are dropped from every log record wherever they appear; card and auth material must never reach a sink.
var sensitiveKeys = map[string]struct{}{
	"authorization": {}, "cookie": {}, "set-cookie": {}, "password": {}, "secret": {}, "token": {},
	"access_token": {}, "refresh_token": {}, "api_key": {}, "pan": {}, "card_number": {}, "cvv": {}, "body": {},
}

const redacted = "[REDACTED]"

// Redact wraps a handler so attributes with sensitive keys (case-insensitive, at any depth) are replaced.
func Redact(base slog.Handler) slog.Handler { return redactor{base} }

type redactor struct{ slog.Handler }

func (r redactor) Handle(ctx context.Context, rec slog.Record) error {
	clean := slog.NewRecord(rec.Time, rec.Level, rec.Message, rec.PC)
	rec.Attrs(func(a slog.Attr) bool {
		clean.AddAttrs(redactAttr(a))
		return true
	})
	return r.Handler.Handle(ctx, clean)
}

func (r redactor) WithAttrs(attrs []slog.Attr) slog.Handler {
	out := make([]slog.Attr, 0, len(attrs))
	for _, a := range attrs {
		out = append(out, redactAttr(a))
	}
	return redactor{r.Handler.WithAttrs(out)}
}

func (r redactor) WithGroup(name string) slog.Handler { return redactor{r.Handler.WithGroup(name)} }

func redactAttr(a slog.Attr) slog.Attr {
	if _, ok := sensitiveKeys[strings.ToLower(a.Key)]; ok {
		return slog.String(a.Key, redacted)
	}
	if a.Value.Kind() == slog.KindGroup {
		members := a.Value.Group()
		out := make([]slog.Attr, 0, len(members))
		for _, m := range members {
			out = append(out, redactAttr(m))
		}
		return slog.Attr{Key: a.Key, Value: slog.GroupValue(out...)}
	}
	return a
}
