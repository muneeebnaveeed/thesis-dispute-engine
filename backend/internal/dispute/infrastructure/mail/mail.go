// Package mail delivers notice emails: over SMTP to whatever relay the deployment names (Mailpit in development),
// or to the log when none is configured, so a deployment without mail still records what it would have sent.
package mail

import (
	"context"
	"fmt"
	"log/slog"
	"mime"
	"net/smtp"
	"strings"
	"time"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
)

// SMTP sends through a relay without authentication; Addr is host:port, From the envelope and header sender.
type SMTP struct {
	Addr string
	From string
	Now  func() time.Time
}

// Send implements application.Mailer as a multipart/alternative message.
func (s SMTP) Send(_ context.Context, m application.Mail) error {
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	boundary := fmt.Sprintf("b%d", now().UnixNano())
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\nTo: %s\r\nSubject: %s\r\nDate: %s\r\nMIME-Version: 1.0\r\n",
		s.From, m.To, mime.QEncoding.Encode("utf-8", m.Subject), now().Format(time.RFC1123Z))
	fmt.Fprintf(&b, "Content-Type: multipart/alternative; boundary=%s\r\n\r\n", boundary)
	fmt.Fprintf(&b, "--%s\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s\r\n", boundary, m.Text)
	fmt.Fprintf(&b, "--%s\r\nContent-Type: text/html; charset=utf-8\r\n\r\n%s\r\n--%s--\r\n", boundary, m.HTML, boundary)
	if err := smtp.SendMail(s.Addr, nil, s.From, []string{m.To}, []byte(b.String())); err != nil {
		return fmt.Errorf("smtp %s: %w", s.Addr, err)
	}
	return nil
}

// Log records the message instead of sending it.
type Log struct{ Log *slog.Logger }

// Send implements application.Mailer.
func (l Log) Send(ctx context.Context, m application.Mail) error {
	l.Log.InfoContext(ctx, "mail (not sent: no relay configured)", "to", m.To, "subject", m.Subject)
	return nil
}
