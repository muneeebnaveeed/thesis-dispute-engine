// Package mail delivers notice emails: over SMTP to whatever relay the deployment names (Mailpit in development),
// or to the log when none is configured, so a deployment without mail still records what it would have sent.
package mail

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"mime"
	"net/smtp"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
)

// SMTP sends through a relay without authentication; Addr is host:port, From the envelope and header sender.
type SMTP struct {
	Addr string
	From string
	Now  func() time.Time
}

// Send implements application.Mailer.
func (s SMTP) Send(ctx context.Context, m application.Mail) error {
	// the recipient is personal data and stays out of the span; the relay and the size are what an operator needs
	_, span := otel.Tracer("mail").Start(ctx, "smtp.send", trace.WithSpanKind(trace.SpanKindClient), trace.WithAttributes(
		attribute.String("messaging.system", "smtp"),
		attribute.String("server.address", s.Addr),
		attribute.Int("mail.attachments", len(m.Files)),
	))
	defer span.End()
	if err := smtp.SendMail(s.Addr, nil, s.From, []string{m.To}, s.Message(m)); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("smtp %s: %w", s.Addr, err)
	}
	return nil
}

// Message builds the RFC 5322 bytes: multipart/alternative for text and HTML, wrapped in multipart/mixed when
// files travel with it, base64 in 76-column lines as RFC 2045 asks. Boundaries derive from the clock so a test
// with a fixed clock gets fixed output.
func (s SMTP) Message(m application.Mail) []byte {
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	stamp := now()
	alt := fmt.Sprintf("alt%d", stamp.UnixNano())
	var body strings.Builder
	fmt.Fprintf(&body, "--%s\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s\r\n", alt, m.Text)
	fmt.Fprintf(&body, "--%s\r\nContent-Type: text/html; charset=utf-8\r\n\r\n%s\r\n--%s--\r\n", alt, m.HTML, alt)

	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\nTo: %s\r\nSubject: %s\r\nDate: %s\r\nMIME-Version: 1.0\r\n",
		s.From, m.To, mime.QEncoding.Encode("utf-8", m.Subject), stamp.Format(time.RFC1123Z))
	if len(m.Files) == 0 {
		fmt.Fprintf(&b, "Content-Type: multipart/alternative; boundary=%s\r\n\r\n%s", alt, body.String())
		return []byte(b.String())
	}
	mixed := fmt.Sprintf("mix%d", stamp.UnixNano())
	fmt.Fprintf(&b, "Content-Type: multipart/mixed; boundary=%s\r\n\r\n", mixed)
	fmt.Fprintf(&b, "--%s\r\nContent-Type: multipart/alternative; boundary=%s\r\n\r\n%s", mixed, alt, body.String())
	for _, f := range m.Files {
		name := mime.QEncoding.Encode("utf-8", f.Name)
		fmt.Fprintf(&b, "--%s\r\nContent-Type: %s; name=\"%s\"\r\nContent-Disposition: attachment; filename=\"%s\"\r\nContent-Transfer-Encoding: base64\r\n\r\n", mixed, f.ContentType, name, name)
		enc := base64.StdEncoding.EncodeToString(f.Content)
		for len(enc) > 76 {
			b.WriteString(enc[:76] + "\r\n")
			enc = enc[76:]
		}
		b.WriteString(enc + "\r\n")
	}
	fmt.Fprintf(&b, "--%s--\r\n", mixed)
	return []byte(b.String())
}

// Log records the message instead of sending it.
type Log struct{ Log *slog.Logger }

// Send implements application.Mailer.
func (l Log) Send(ctx context.Context, m application.Mail) error {
	l.Log.InfoContext(ctx, "mail (not sent: no relay configured)", "to", m.To, "subject", m.Subject)
	return nil
}
