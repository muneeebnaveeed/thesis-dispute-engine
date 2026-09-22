package notice

import (
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/golden"
)

// Every word a customer can receive is pinned by a file under testdata; a change to wording is a diff in review.
// Regenerate with: go test ./internal/dispute/notice -update

func goldenFacts(regime domain.Regime) Facts {
	due := time.Date(2026, 10, 12, 23, 59, 59, 0, time.UTC)
	cur := "EUR"
	if regime == domain.RegimeUSRegE || regime == domain.RegimeUSRegZ {
		cur = "USD"
	}
	return Facts{
		Bank: "OTP Bank", Customer: "Kovács Anna", DisputeID: "01a0c601-0000-7000-8000-000000000001", Reason: domain.ReasonUnauthorised,
		Regime: regime, Merchant: "MediaMarkt", Amount: decimal.RequireFromString("1899"), Currency: cur, Credited: decimal.RequireFromString("1849"),
		OccurredAt: time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC), OpenedAt: time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC),
		Now: time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC), ResolutionDue: &due, ReversalAmount: decimal.RequireFromString("1849"),
		Questions: domain.QuestionSet(domain.ReasonUnauthorised),
	}
}

func TestAutomaticNoticesMatchGolden(t *testing.T) {
	kinds := []domain.NoticeKind{domain.NoticeAcknowledgement, domain.NoticeQuestionnaire, domain.NoticeProvisionalCredit,
		domain.NoticeRefund, domain.NoticeReversal, domain.NoticeResolution}
	outcomes := []Outcome{OutcomeCredited, OutcomeDenied, OutcomeReversed, OutcomeRecovered}
	for _, regime := range domain.AllRegimes() {
		for _, kind := range kinds {
			f := goldenFacts(regime)
			if kind == domain.NoticeResolution {
				for _, o := range outcomes {
					f.Outcome = o
					d := Compose(kind, f)
					golden.Check(t, "notices/"+strings.ToLower(string(regime))+"/"+strings.ToLower(string(kind))+"_"+strings.ToLower(string(o))+".txt", []byte(d.Text()))
				}
				continue
			}
			d := Compose(kind, f)
			base := "notices/" + strings.ToLower(string(regime)) + "/" + strings.ToLower(string(kind))
			golden.Check(t, base+".txt", []byte(d.Text()))
			if regime == domain.RegimeUSRegE {
				// One HTML rendering per kind is enough to pin the email layout; the words are the same as the text.
				golden.Check(t, base+".html", []byte(d.HTML(f.Now)))
			}
		}
	}
}

func TestAnalystTemplatesMatchGolden(t *testing.T) {
	facts := FactValues{Customer: "Kovács Anna", Bank: "OTP Bank", Amount: "1899.00 EUR", Merchant: "MediaMarkt",
		Dispute: "01a0c601-0000-7000-8000-000000000001", Today: time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)}
	inputs := map[domain.NoticeKind]map[string]string{
		domain.NoticeRequestForInformation: {"items": "receipt,merchant_contact", "other": "the serial number\nthe box it came in", "days": "10"},
		domain.NoticeStatusUpdate:          {"stage": "merchant", "note": "The merchant has 30 days to answer.", "nextBy": "2026-10-20"},
		domain.NoticeDocumentsReceived:     {"received": "receipt dated 9 September\nphotos of the item", "missing": "the courier's proof of delivery"},
		domain.NoticeCustom:                {"subject": "Your new card", "body": "We have posted a replacement card to your address on file.\nIt arrives within five working days."},
	}
	for _, tpl := range Templates() {
		values, err := Validate(tpl, inputs[tpl.Kind])
		if err != nil {
			t.Fatalf("%s: %v", tpl.Kind, err)
		}
		d := Fill(tpl, facts, values)
		name := "templates/" + strings.ToLower(string(tpl.Kind))
		golden.Check(t, name+".txt", []byte(d.Text()))
		golden.Check(t, name+".html", []byte(d.HTML(facts.Today)))
		// What the browser gets for its live preview: facts in, fields as placeholders.
		p := Placeholders(tpl, facts)
		golden.Check(t, name+".placeholders.txt", []byte(p.Subject+"\n\n"+strings.Join(p.Paragraphs, "\n\n")+"\n"))
	}
}
