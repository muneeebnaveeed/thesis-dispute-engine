package application

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/notice"
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
	DisputeID uuid.UUID
	Template  domain.NoticeKind
	Fields    map[string]string
	Actor     string
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
		body, err := json.Marshal(doc)
		if err != nil {
			return err
		}
		rules, err := domain.RulesFor(rec.Regime)
		if err != nil {
			return err
		}
		channels := []domain.Channel{domain.ChannelEmail}
		if tpl.Letter && rules.WrittenNotices {
			channels = append(channels, domain.ChannelLetter)
		}
		for _, ch := range channels {
			n := NoticeRecord{DisputeID: rec.ID, Seq: int(rec.Version), Kind: tpl.Kind, Channel: ch, Subject: doc.Subject, Document: body, CreatedAt: now, Actor: in.Actor}
			switch ch {
			case domain.ChannelEmail:
				n.Recipient = facts.Email
			case domain.ChannelLetter:
				n.Recipient, n.SentAt = facts.PostalAddress, &now
			}
			if n.Recipient == "" {
				continue
			}
			if _, err := tx.InsertNotice(ctx, n); err != nil {
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
