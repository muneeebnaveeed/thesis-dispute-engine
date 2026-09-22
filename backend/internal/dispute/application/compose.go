package application

import (
	"context"
	"encoding/json"
	"time"

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
		resolved, err := tenantTemplates(ctx, tx)
		if err != nil {
			return err
		}
		for _, t := range resolved {
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
	if _, ok := notice.TemplateFor(in.Template); !ok {
		return DisputeView{}, notice.ErrUnknownTemplate.WithDetail("%q", in.Template)
	}
	var view DisputeView
	err := s.store.WithTx(ctx, func(tx Tx) error {
		rec, err := tx.GetDispute(ctx, in.DisputeID)
		if err != nil {
			return err
		}
		tpl, err := tenantTemplate(ctx, tx, in.Template)
		if err != nil {
			return err
		}
		values, err := notice.Validate(tpl, in.Fields)
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

// tenantTemplates is the catalogue with the tenant's wording laid over each base.
func tenantTemplates(ctx context.Context, tx Tx) ([]notice.Template, error) {
	overrides, err := tx.TenantTemplates(ctx)
	if err != nil {
		return nil, err
	}
	byKind := map[domain.NoticeKind]TenantTemplate{}
	for _, o := range overrides {
		byKind[o.Kind] = o
	}
	base := notice.Templates()
	out := make([]notice.Template, 0, len(base))
	for _, t := range base {
		out = append(out, applyStored(t, byKind[t.Kind]))
	}
	return out, nil
}

func tenantTemplate(ctx context.Context, tx Tx, kind domain.NoticeKind) (notice.Template, error) {
	all, err := tenantTemplates(ctx, tx)
	if err != nil {
		return notice.Template{}, err
	}
	for _, t := range all {
		if t.Kind == kind {
			return t, nil
		}
	}
	return notice.Template{}, notice.ErrUnknownTemplate.WithDetail("%q", kind)
}

// applyStored lays a stored override over its base; a stored override that no longer fits (the base changed)
// falls back to the base rather than breaking the tenant's mail, and is reported when listed for editing.
func applyStored(base notice.Template, stored TenantTemplate) notice.Template {
	if len(stored.Override) == 0 {
		return base
	}
	var o notice.Override
	if err := json.Unmarshal(stored.Override, &o); err != nil {
		return base
	}
	t, err := notice.Apply(base, o)
	if err != nil {
		return base
	}
	return t
}

// TemplateSetting is one kind as the tenant admin sees it: the base, the override if any, and the result.
type TemplateSetting struct {
	Base      notice.Template
	Override  *notice.Override
	Effective notice.Template
	UpdatedBy string
	UpdatedAt *time.Time
}

// ListTemplateSettings returns every analyst email kind with the tenant's wording, for the editor.
func (s *Service) ListTemplateSettings(ctx context.Context) ([]TemplateSetting, error) {
	var out []TemplateSetting
	err := s.store.WithTx(ctx, func(tx Tx) error {
		overrides, err := tx.TenantTemplates(ctx)
		if err != nil {
			return err
		}
		byKind := map[domain.NoticeKind]TenantTemplate{}
		for _, o := range overrides {
			byKind[o.Kind] = o
		}
		for _, base := range notice.Templates() {
			setting := TemplateSetting{Base: base, Effective: base}
			if stored, ok := byKind[base.Kind]; ok {
				var o notice.Override
				if json.Unmarshal(stored.Override, &o) == nil {
					setting.Override = &o
					if eff, err := notice.Apply(base, o); err == nil {
						setting.Effective = eff
					}
				}
				setting.UpdatedBy = stored.UpdatedBy
				at := stored.UpdatedAt
				setting.UpdatedAt = &at
			}
			out = append(out, setting)
		}
		return nil
	})
	return out, err
}

// PutTemplateSetting validates and stores the tenant's wording for one kind.
func (s *Service) PutTemplateSetting(ctx context.Context, kind domain.NoticeKind, o notice.Override, actor string) (TemplateSetting, error) {
	base, ok := notice.TemplateFor(kind)
	if !ok {
		return TemplateSetting{}, notice.ErrUnknownTemplate.WithDetail("%q", kind)
	}
	effective, err := notice.Apply(base, o)
	if err != nil {
		return TemplateSetting{}, err
	}
	raw, err := json.Marshal(o)
	if err != nil {
		return TemplateSetting{}, err
	}
	now := s.now()
	err = s.store.WithTx(ctx, func(tx Tx) error {
		return tx.PutTenantTemplate(ctx, TenantTemplate{Kind: kind, Override: raw, UpdatedBy: actor, UpdatedAt: now})
	})
	return TemplateSetting{Base: base, Override: &o, Effective: effective, UpdatedBy: actor, UpdatedAt: &now}, err
}

// DeleteTemplateSetting reverts a kind to the base wording.
func (s *Service) DeleteTemplateSetting(ctx context.Context, kind domain.NoticeKind) error {
	if _, ok := notice.TemplateFor(kind); !ok {
		return notice.ErrUnknownTemplate.WithDetail("%q", kind)
	}
	return s.store.WithTx(ctx, func(tx Tx) error {
		found, err := tx.DeleteTenantTemplate(ctx, kind)
		if err != nil {
			return err
		}
		if !found {
			return ErrNotFound
		}
		return nil
	})
}
