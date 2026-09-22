package mail

import (
	"testing"
	"time"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/golden"
)

// The bytes on the wire, with and without an attachment. Regenerate with: go test ./internal/dispute/infrastructure/mail -update
func TestMessageMatchesGolden(t *testing.T) {
	s := SMTP{Addr: "localhost:1025", From: "disputes@dispute-engine.local", Now: func() time.Time { return time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC) }}
	plain := application.Mail{To: "anna.kovacs@example.com", Subject: "Your refund", Text: "Dear Kovács Anna,\n\nWe have refunded 1849.00 EUR.\n", HTML: "<html><body>We have refunded 1849.00 EUR.</body></html>"}
	golden.Check(t, "plain.eml", s.Message(plain))
	withFile := plain
	withFile.Subject = "Your statement (with attachment)"
	withFile.Files = []application.MailFile{{Name: "statement.pdf", ContentType: "application/pdf", Content: []byte("%PDF-1.4\n" + string(make([]byte, 100)) + "\n%%EOF")}}
	golden.Check(t, "attachment.eml", s.Message(withFile))
}
