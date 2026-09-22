// Package notice writes what the bank tells the customer at each point of a dispute. A notice is composed as
// structured paragraphs, then rendered to plain text for email and HTML for a printable letter, so the same
// words go out on every channel and the workbench can show them without trusting markup.
package notice

import (
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
)

// Facts is everything a notice may mention. Unused fields are ignored by kinds that do not need them.
type Facts struct {
	Bank           string
	Customer       string
	DisputeID      string
	Reason         domain.Reason
	Regime         domain.Regime
	Merchant       string
	Amount         decimal.Decimal
	Currency       string
	Credited       decimal.Decimal // what the customer received on a credit or refund
	OccurredAt     time.Time       // the disputed transaction
	OpenedAt       time.Time
	Now            time.Time
	ResolutionDue  *time.Time
	Questions      []domain.Question
	Outcome        Outcome // for RESOLUTION
	ReversalAmount decimal.Decimal
}

// Outcome is how a dispute ended, as the customer needs to hear it.
type Outcome string

// Outcomes.
const (
	OutcomeCredited  Outcome = "CREDITED"  // the customer keeps the money
	OutcomeDenied    Outcome = "DENIED"    // no credit was, or will be, given
	OutcomeReversed  Outcome = "REVERSED"  // a provisional credit was taken back
	OutcomeRecovered Outcome = "RECOVERED" // the customer keeps the money and the bank recovered it
)

// Document is a composed notice: the subject and the paragraphs, in order.
type Document struct {
	Bank       string // who is writing; the letterhead
	Subject    string
	Greeting   string
	Paragraphs []string
	Closing    string
	Basis      string // the regulatory provision the notice satisfies, when one does
}

// Compose writes the notice of a kind from the facts.
func Compose(kind domain.NoticeKind, f Facts) Document {
	money := func(d decimal.Decimal) string { return d.StringFixed(2) + " " + f.Currency }
	day := func(t time.Time) string { return t.Format("2 January 2006") }
	d := Document{Bank: f.Bank, Greeting: "Dear " + f.Customer + ",", Closing: "Yours sincerely,\n" + f.Bank + " disputes team"}
	ref := "Dispute reference " + f.DisputeID + "."
	switch kind {
	case domain.NoticeAcknowledgement:
		d.Subject = "We have received your dispute"
		d.Paragraphs = []string{
			fmt.Sprintf("We have received your dispute of the %s payment of %s to %s made on %s, and we have opened an investigation.",
				strings.ToLower(reasonWords(f.Reason)), money(f.Amount), f.Merchant, day(f.OccurredAt)),
		}
		if f.ResolutionDue != nil {
			d.Paragraphs = append(d.Paragraphs, fmt.Sprintf("We will let you know the outcome by %s.", day(*f.ResolutionDue)))
		}
		d.Paragraphs = append(d.Paragraphs, "You do not need to do anything now. If we need more from you, we will ask.", ref)
		d.Basis = basis(f.Regime, kind)
	case domain.NoticeQuestionnaire:
		d.Subject = "A few questions about your dispute"
		d.Paragraphs = []string{"To investigate your dispute we need your answers to the following questions. Please reply as soon as you can."}
		for i, q := range f.Questions {
			opt := ""
			if !q.Required {
				opt = " (optional)"
			}
			d.Paragraphs = append(d.Paragraphs, fmt.Sprintf("%d. %s%s", i+1, q.Text, opt))
		}
		d.Paragraphs = append(d.Paragraphs, ref)
	case domain.NoticeProvisionalCredit:
		d.Subject = "Provisional credit posted to your account"
		d.Paragraphs = []string{
			fmt.Sprintf("While we investigate, we have credited %s to your account on %s. You may use these funds.", money(f.Credited), day(f.Now)),
			"This credit is provisional. If our investigation finds that no error occurred, we will reverse it and tell you the date and amount before we do.",
			ref,
		}
		d.Basis = basis(f.Regime, kind)
	case domain.NoticeRefund:
		d.Subject = "Your refund"
		d.Paragraphs = []string{fmt.Sprintf("We have refunded %s to your account on %s.", money(f.Credited), day(f.Now))}
		if f.Credited.LessThan(f.Amount) {
			d.Paragraphs = append(d.Paragraphs, fmt.Sprintf("This is %s less than the disputed amount, which is the share the rules leave with you; we can explain how it was determined.", money(f.Amount.Sub(f.Credited))))
		}
		d.Paragraphs = append(d.Paragraphs, ref)
		d.Basis = basis(f.Regime, kind)
	case domain.NoticeReversal:
		d.Subject = "Reversal of your provisional credit"
		d.Paragraphs = []string{
			fmt.Sprintf("Our investigation has concluded that no error occurred. The provisional credit of %s will be debited from your account on %s.",
				money(f.ReversalAmount), day(f.Now.AddDate(0, 0, 5))),
			"Until that date we will honour payments from your account up to that amount without charge, as the rules require.",
			"You may ask us for the documents we relied on.", ref,
		}
		d.Basis = basis(f.Regime, kind)
	case domain.NoticeResolution:
		d.Subject = "The outcome of your dispute"
		switch f.Outcome {
		case OutcomeCredited, OutcomeRecovered:
			d.Paragraphs = []string{fmt.Sprintf("We have completed our investigation of your dispute of %s to %s and found in your favour. The credit of %s to your account is final.", money(f.Amount), f.Merchant, money(f.Credited))}
		case OutcomeReversed:
			d.Paragraphs = []string{fmt.Sprintf("We have completed our investigation of your dispute of %s to %s and found that no error occurred. The provisional credit has been reversed, as we told you separately.", money(f.Amount), f.Merchant)}
		default:
			d.Paragraphs = []string{fmt.Sprintf("We have completed our investigation of your dispute of %s to %s and found that no error occurred. No credit has been made.", money(f.Amount), f.Merchant)}
		}
		d.Paragraphs = append(d.Paragraphs, "You may ask us for copies of the documents we relied on, and you may appeal this decision.", ref)
		d.Basis = basis(f.Regime, kind)
	}
	return d
}

// Text renders the document for the body of an email.
func (d Document) Text() string {
	var b strings.Builder
	b.WriteString(d.Greeting + "\n\n")
	for _, p := range d.Paragraphs {
		b.WriteString(p + "\n\n")
	}
	b.WriteString(d.Closing + "\n")
	if d.Basis != "" {
		b.WriteString("\n" + d.Basis + "\n")
	}
	return b.String()
}

// HTML renders the document for the body of an email. Email clients are not browsers: no stylesheet, no
// margins, no max-width, a table for layout and every style inline, which is what the widest set of clients
// renders the same way. The printable letter is laid out by the workbench, not by this.
func (d Document) HTML(date time.Time) string {
	const text = "font-family:Georgia,'Times New Roman',serif;font-size:16px;line-height:24px;color:#111111;"
	const small = "font-family:Georgia,'Times New Roman',serif;font-size:13px;line-height:20px;color:#555555;"
	cell := func(style, inner string) string {
		return "<tr><td style=\"" + style + "padding:0 0 16px 0;\">" + inner + "</td></tr>"
	}
	var b strings.Builder
	b.WriteString("<!DOCTYPE html><html lang=\"en\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width\"><title>" + html.EscapeString(d.Subject) + "</title></head>")
	b.WriteString("<body>")
	b.WriteString("<table role=\"presentation\" width=\"100%\" cellpadding=\"0\" cellspacing=\"0\"><tr><td align=\"center\" style=\"padding:24px 12px;\">")
	b.WriteString("<table role=\"presentation\" width=\"600\" cellpadding=\"0\" cellspacing=\"0\">")
	b.WriteString(cell(small, html.EscapeString(d.Bank)+"<br>"+html.EscapeString(date.Format("2 January 2006"))))
	b.WriteString(cell(text+"font-size:20px;line-height:28px;", "<b>"+html.EscapeString(d.Subject)+"</b>"))
	b.WriteString(cell(text, html.EscapeString(d.Greeting)))
	for _, p := range d.Paragraphs {
		b.WriteString(cell(text, strings.ReplaceAll(html.EscapeString(p), "\n", "<br>")))
	}
	b.WriteString(cell(text, strings.ReplaceAll(html.EscapeString(d.Closing), "\n", "<br>")))
	if d.Basis != "" {
		b.WriteString(cell(small, html.EscapeString(d.Basis)))
	}
	b.WriteString("</table></td></tr></table></body></html>")
	return b.String()
}

func reasonWords(r domain.Reason) string {
	switch r {
	case domain.ReasonNotReceived:
		return "Undelivered"
	case domain.ReasonDuplicate:
		return "Duplicate"
	case domain.ReasonAmountDiffers:
		return "Incorrectly charged"
	default:
		return "Unauthorised"
	}
}

// basis names the provision a notice satisfies; empty where none requires it.
func basis(r domain.Regime, kind domain.NoticeKind) string {
	switch r {
	case domain.RegimeUSRegE:
		switch kind {
		case domain.NoticeProvisionalCredit:
			return "This notice is given under 12 CFR 1005.11(c)(2)(iv)."
		case domain.NoticeReversal:
			return "This notice is given under 12 CFR 1005.11(d)(2)."
		case domain.NoticeResolution:
			return "This notice is given under 12 CFR 1005.11(d)(1)."
		}
	case domain.RegimeUSRegZ:
		switch kind {
		case domain.NoticeAcknowledgement:
			return "This written acknowledgement is given under 12 CFR 1026.13(c)(1)."
		case domain.NoticeResolution:
			return "This notice is given under 12 CFR 1026.13(e) and (f)."
		}
	case domain.RegimeEUPSD2Card:
		if kind == domain.NoticeRefund {
			return "Refund made under Article 73 of Directive (EU) 2015/2366."
		}
	case domain.RegimeEUSEPADirectDebit:
		if kind == domain.NoticeRefund {
			return "Refund made under Article 77 of Directive (EU) 2015/2366 and the SEPA Core Direct Debit Scheme Rulebook."
		}
	}
	return ""
}
