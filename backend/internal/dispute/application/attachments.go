package application

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/errs"
)

// Attachment limits: what a mail relay and a database row both take without complaint.
const (
	MaxAttachmentBytes = 5 << 20
	MaxAttachments     = 3
	DraftAttachmentTTL = 24 * time.Hour
)

// AllowedAttachmentTypes are the documents a dispute exchange needs: statements, receipts, forms, photographs.
var AllowedAttachmentTypes = []string{"application/pdf", "image/png", "image/jpeg"}

// Errors for uploads and their use.
var (
	ErrAttachmentRefused = errs.New(errs.Unprocessable, "attachment-refused", "the file cannot be attached")
	ErrAttachmentUnknown = errs.New(errs.Unprocessable, "attachment-unknown", "an attachment is not a draft of this dispute")
)

// UploadInput is one file from the analyst.
type UploadInput struct {
	DisputeID   uuid.UUID
	Filename    string
	ContentType string
	Content     []byte
	Actor       string
}

// Upload stores a draft attachment for a dispute; a later ComposeEmail claims it.
func (s *Service) Upload(ctx context.Context, in UploadInput) (Attachment, error) {
	switch {
	case len(in.Content) == 0:
		return Attachment{}, ErrAttachmentRefused.WithDetail("the file is empty")
	case len(in.Content) > MaxAttachmentBytes:
		return Attachment{}, ErrAttachmentRefused.WithDetail("the file is larger than %d MB", MaxAttachmentBytes>>20)
	case !slices.Contains(AllowedAttachmentTypes, in.ContentType):
		return Attachment{}, ErrAttachmentRefused.WithDetail("%s is not a PDF, PNG or JPEG", in.ContentType)
	case in.Filename == "" || len(in.Filename) > 200:
		return Attachment{}, ErrAttachmentRefused.WithDetail("the file needs a name of at most 200 characters")
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Attachment{}, err
	}
	a := Attachment{ID: id, DisputeID: in.DisputeID, Filename: in.Filename, ContentType: in.ContentType, Size: len(in.Content), Content: in.Content,
		UploadedBy: in.Actor, UploadedAt: s.now()}
	err = s.store.WithTx(ctx, func(tx Tx) error {
		if _, err := tx.GetDispute(ctx, in.DisputeID); err != nil {
			return err
		}
		return tx.InsertAttachment(ctx, in.DisputeID, a)
	})
	a.Content = nil
	return a, err
}

// GetAttachment returns one file with its bytes, for the analyst to open from the sent list.
func (s *Service) GetAttachment(ctx context.Context, disputeID, id uuid.UUID) (Attachment, error) {
	var a Attachment
	err := s.store.WithTx(ctx, func(tx Tx) error {
		var err error
		a, err = tx.GetAttachment(ctx, disputeID, id)
		return err
	})
	return a, err
}

// claimAttachments binds the drafts an email names to its notice and refuses anything that is not this
// dispute's draft; enclosures are then listed on the letter, if any.
func claimAttachments(ctx context.Context, tx Tx, disputeID uuid.UUID, noticeID int64, ids []uuid.UUID) ([]Attachment, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	if len(ids) > MaxAttachments {
		return nil, ErrAttachmentRefused.WithDetail("at most %d attachments per email", MaxAttachments)
	}
	n, err := tx.ClaimAttachments(ctx, disputeID, noticeID, ids)
	if err != nil {
		return nil, err
	}
	if n != len(ids) {
		return nil, ErrAttachmentUnknown
	}
	metas, err := tx.ListAttachmentMeta(ctx, disputeID, []int64{noticeID})
	if err != nil {
		return nil, err
	}
	var claimed []Attachment
	for _, m := range metas {
		if m.NoticeID != nil && *m.NoticeID == noticeID {
			claimed = append(claimed, m)
		}
	}
	return claimed, nil
}

// RunDraftAttachmentPurge deletes uploads never attached to an email once they are older than DraftAttachmentTTL.
func RunDraftAttachmentPurge(ctx context.Context, store Store, log *slog.Logger) {
	counter, err := otel.Meter(scopeName).Int64Counter("dispute.draft_attachments_purged", metric.WithDescription("Uploads never attached to an email, deleted by the sweep"))
	if err != nil {
		log.Error("draft attachment purge: meter", "err", err)
		return
	}
	timer := time.NewTimer(2 * time.Minute)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		n, err := store.PurgeDraftAttachments(ctx, time.Now().Add(-DraftAttachmentTTL))
		switch {
		case err != nil && ctx.Err() == nil:
			log.Warn("draft attachment purge", "err", err)
		case n > 0:
			counter.Add(ctx, n)
			log.Info("draft attachment purge", "deleted", n)
		}
		timer.Reset(time.Hour)
	}
}

func enclosures(files []Attachment) string {
	if len(files) == 0 {
		return ""
	}
	out := "Enclosures:"
	for _, f := range files {
		out += fmt.Sprintf("\n- %s", f.Filename)
	}
	return out
}
