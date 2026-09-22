package notice

import (
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
)

func facts() Facts {
	due := time.Date(2026, 10, 12, 23, 59, 59, 0, time.UTC)
	return Facts{
		Bank: "OTP Bank", Customer: "Kovács Anna", DisputeID: "01a0c601", Reason: domain.ReasonUnauthorised, Regime: domain.RegimeUSRegE,
		Merchant: "MediaMarkt", Amount: decimal.RequireFromString("1899"), Currency: "EUR", Credited: decimal.RequireFromString("1849"),
		OccurredAt: time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC), OpenedAt: time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC),
		Now: time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC), ResolutionDue: &due,
	}
}

func TestEveryKindComposesAndRendersBothWays(t *testing.T) {
	for _, kind := range []domain.NoticeKind{domain.NoticeAcknowledgement, domain.NoticeQuestionnaire, domain.NoticeProvisionalCredit,
		domain.NoticeRefund, domain.NoticeReversal, domain.NoticeResolution} {
		f := facts()
		f.Questions = domain.QuestionSet(domain.ReasonUnauthorised)
		f.Outcome = OutcomeDenied
		d := Compose(kind, f)
		if d.Subject == "" || len(d.Paragraphs) < 2 || !strings.Contains(d.Text(), "01a0c601") {
			t.Errorf("%s: %+v", kind, d)
		}
		h := d.HTML(f.Now)
		if !strings.Contains(h, "<b>"+d.Subject+"</b>") || !strings.Contains(h, "Kovács Anna") {
			t.Errorf("%s: html %s", kind, h[:80])
		}
		// Email-client friendly: no stylesheet, no margins or max-width, every style inline on table cells.
		for _, banned := range []string{"<style", "max-width", "margin:3em", "font:"} {
			if strings.Contains(h, banned) {
				t.Errorf("%s: html uses %q, which email clients render unevenly", kind, banned)
			}
		}
	}
}

func TestWordsFollowTheFacts(t *testing.T) {
	f := facts()
	ack := Compose(domain.NoticeAcknowledgement, f).Text()
	if !strings.Contains(ack, "unauthorised payment of 1899.00 EUR to MediaMarkt made on 9 September 2026") || !strings.Contains(ack, "by 12 October 2026") {
		t.Errorf("acknowledgement:\n%s", ack)
	}
	refund := Compose(domain.NoticeRefund, f).Text()
	if !strings.Contains(refund, "refunded 1849.00 EUR") || !strings.Contains(refund, "50.00 EUR less than the disputed amount") {
		t.Errorf("refund with liability:\n%s", refund)
	}
	f.Credited = f.Amount
	if strings.Contains(Compose(domain.NoticeRefund, f).Text(), "less than") {
		t.Error("full refund mentions a shortfall")
	}
	f.ReversalAmount = decimal.RequireFromString("1849")
	rev := Compose(domain.NoticeReversal, f)
	if !strings.Contains(rev.Text(), "1849.00 EUR will be debited") || !strings.Contains(rev.Basis, "1005.11(d)(2)") {
		t.Errorf("reversal:\n%s\n%s", rev.Text(), rev.Basis)
	}
	f.Regime = domain.RegimeEUPSD2Card
	if b := Compose(domain.NoticeReversal, f).Basis; b != "" {
		t.Errorf("PSD2 reversal cites %q", b)
	}
	if b := Compose(domain.NoticeRefund, f).Basis; !strings.Contains(b, "Article 73") {
		t.Errorf("PSD2 refund basis %q", b)
	}
}

func TestHTMLEscapesEverything(t *testing.T) {
	f := facts()
	f.Customer = `<script>alert("x")</script>`
	f.Bank = "<b>bank</b>"
	h := Compose(domain.NoticeAcknowledgement, f).HTML(f.Now)
	if strings.Contains(h, "<script>") || strings.Contains(h, "<b>bank") {
		t.Error("unescaped input reached the letter")
	}
}
