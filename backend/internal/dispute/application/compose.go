package application

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/notice"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/errs"
)

// EmailTemplates are the analyst's choices for one dispute: each template with the facts already in its words
// and the fields left as placeholders, so the browser can preview as the analyst types.
type EmailTemplates struct {
	Templates []notice.Template
	Facts     notice.FactValues
}

// ListEmailTemplates returns the catalogue rendered against one dispute.
func (s *Service) ListEmailTemplates(ctx context.Context, disputeID uuid.UUID) (EmailTemplates, error) {
	var out EmailTemplates
	err := s.store.WithTx(ctx, func(tx Tx) error {
		rec, err := tx.GetDispute(ctx, disputeID)
		if err != nil {
			return err
		}
		facts, err := s.facts(ctx, tx, rec, s.now())
		if err != nil {
			return err
		}
		out.Facts = factValues(facts.Facts)
		for _, t := range notice.Templates() {
			out.Templates = append(out.Templates, notice.Placeholders(t, out.Facts))
		}
		return nil
	})
	return out, err
}

// ComposeEmailInput is what the analyst sends: which template, the form values, and who they are.
type ComposeEmailInput struct {
	DisputeID   uuid.UUID
	Template    domain.NoticeKind
	Fields      map[string]string
	Attachments []uuid.UUID // drafts uploaded for this dispute
	Actor       string
}

// ComposeEmail validates the form against the template, composes the final document server-side and puts it in
// the outbox (and on paper where the regime requires writing); the dispatcher sends it after commit.
func (s *Service) ComposeEmail(ctx context.Context, in ComposeEmailInput) (DisputeView, error) {
	tpl, ok := notice.TemplateFor(in.Template)
	if !ok {
		return DisputeView{}, notice.ErrUnknownTemplate.WithDetail("%q", in.Template)
	}
	values, err := notice.Validate(tpl, in.Fields)
	if err != nil {
		return DisputeView{}, err
	}
	var view DisputeView
	err = s.store.WithTx(ctx, func(tx Tx) error {
		rec, err := tx.GetDispute(ctx, in.DisputeID)
		if err != nil {
			return err
		}
		now := s.now()
		facts, err := s.facts(ctx, tx, rec, now)
		if err != nil {
			return err
		}
		doc := notice.Fill(tpl, factValues(facts.Facts), values)
		rules, err := domain.RulesFor(rec.Regime)
		if err != nil {
			return err
		}
		// The email carries the files; the letter, if any, lists them as enclosures. Attachments are claimed by
		// the email's row, so the claim happens once the row exists and before the letter is composed.
		emailBody, err := json.Marshal(doc)
		if err != nil {
			return err
		}
		if facts.Email == "" {
			return ErrNotResendable.WithDetail("the account has no email address")
		}
		email := NoticeRecord{DisputeID: rec.ID, Seq: int(rec.Version), Kind: tpl.Kind, Channel: domain.ChannelEmail, Recipient: facts.Email,
			Subject: doc.Subject, Document: emailBody, CreatedAt: now, Actor: in.Actor}
		emailID, err := tx.InsertNotice(ctx, email)
		if err != nil {
			return err
		}
		files, err := claimAttachments(ctx, tx, rec.ID, emailID, in.Attachments)
		if err != nil {
			return err
		}
		if tpl.Letter && rules.WrittenNotices && facts.PostalAddress != "" {
			letterDoc := doc
			if enc := enclosures(files); enc != "" {
				letterDoc.Paragraphs = append(append([]string(nil), doc.Paragraphs...), enc)
			}
			letterBody, err := json.Marshal(letterDoc)
			if err != nil {
				return err
			}
			letter := NoticeRecord{DisputeID: rec.ID, Seq: int(rec.Version), Kind: tpl.Kind, Channel: domain.ChannelLetter, Recipient: facts.PostalAddress,
				Subject: doc.Subject, Document: letterBody, CreatedAt: now, SentAt: &now, Actor: in.Actor}
			if _, err := tx.InsertNotice(ctx, letter); err != nil {
				return err
			}
		}
		s.composed.Add(ctx, 1, metric.WithAttributes(tenantAttr(ctx), attribute.String("template", string(tpl.Kind))))
		view, err = s.view(ctx, tx, rec)
		return err
	})
	if err == nil {
		s.afterCommit()
	}
	return view, err
}

func factValues(f notice.Facts) notice.FactValues {
	return notice.FactValues{Customer: f.Customer, Bank: f.Bank, Amount: f.Amount.StringFixed(2) + " " + f.Currency,
		Merchant: f.Merchant, Dispute: f.DisputeID, Today: f.Now}
}

// ResendInput names the notice to send again and who asked.
type ResendInput struct {
	DisputeID uuid.UUID
	NoticeID  int64
	Actor     string
}

// ErrNotResendable means the notice is a letter, or an email that never went out (the outbox is still trying).
var ErrNotResendable = errs.New(errs.Conflict, "not-resendable", "only an email that has been sent can be sent again")

// Resend queues a new email with the original's words, to the customer's current address, chained to the
// original and authored by the analyst; the original keeps its own history.
func (s *Service) Resend(ctx context.Context, in ResendInput) (DisputeView, error) {
	var view DisputeView
	err := s.store.WithTx(ctx, func(tx Tx) error {
		rec, err := tx.GetDispute(ctx, in.DisputeID)
		if err != nil {
			return err
		}
		orig, err := tx.GetNotice(ctx, rec.ID, in.NoticeID)
		if err != nil {
			return err
		}
		if orig.Channel != domain.ChannelEmail || orig.SentAt == nil {
			return ErrNotResendable
		}
		account, err := tx.GetAccount(ctx, rec.AccountID)
		if err != nil {
			return err
		}
		if account.Email == "" {
			return ErrNotResendable.WithDetail("the account has no email address")
		}
		now := s.now()
		id := orig.ID
		n := NoticeRecord{DisputeID: rec.ID, Seq: orig.Seq, Kind: orig.Kind, Channel: domain.ChannelEmail, Recipient: account.Email,
			Subject: orig.Subject, Document: orig.Document, CreatedAt: now, Actor: in.Actor, ResendOf: &id}
		if _, err := tx.InsertNotice(ctx, n); err != nil {
			return err
		}
		s.composed.Add(ctx, 1, metric.WithAttributes(tenantAttr(ctx), attribute.String("template", "resend")))
		view, err = s.view(ctx, tx, rec)
		return err
	})
	if err == nil {
		s.afterCommit()
	}
	return view, err
}
