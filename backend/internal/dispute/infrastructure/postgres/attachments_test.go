//go:build integration

package postgres_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application/apptest"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
	disputepg "github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/infrastructure/postgres"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/postgres/pgtest"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/tenant"
)

// As dispute_api: bytes round-trip, a claim binds only this dispute's drafts, the dispatcher reads files across
// tenants, and the sweep removes stale drafts.
func TestAttachmentsThroughPostgres(t *testing.T) {
	owner, schema := pgtest.PoolWithSchema(t)
	app := pgtest.AppPool(t, schema)
	store := disputepg.NewStore(app)
	svc, err := application.NewService(store, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctxA := tenant.WithID(context.Background(), apptest.TenantA)
	txn := seedFor(t, owner, apptest.TenantA, domain.RailCard, "EUR")
	d1, _ := svc.CreateDispute(ctxA, application.CreateDisputeInput{TransactionID: txn})
	d2, _ := svc.CreateDispute(ctxA, application.CreateDisputeInput{TransactionID: txn})

	content := []byte("%PDF-1.4 bytes")
	a, err := svc.Upload(ctxA, application.UploadInput{DisputeID: d1.View.ID, Filename: "receipt.pdf", ContentType: "application/pdf", Content: content, Actor: "an@otp"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.GetAttachment(ctxA, d1.View.ID, a.ID)
	if err != nil || !bytes.Equal(got.Content, content) || got.NoticeID != nil {
		t.Fatalf("round trip = %+v %v", got, err)
	}
	// Another dispute cannot claim it.
	if _, err := svc.ComposeEmail(ctxA, application.ComposeEmailInput{DisputeID: d2.View.ID, Template: domain.NoticeCustom,
		Fields: map[string]string{"subject": "s", "body": "b"}, Attachments: []uuid.UUID{a.ID}}); err == nil {
		t.Error("claimed across disputes")
	}
	view, err := svc.ComposeEmail(ctxA, application.ComposeEmailInput{DisputeID: d1.View.ID, Template: domain.NoticeCustom,
		Fields: map[string]string{"subject": "s", "body": "b"}, Attachments: []uuid.UUID{a.ID}, Actor: "an@otp"})
	if err != nil {
		t.Fatal(err)
	}
	email := view.Notices[len(view.Notices)-1]
	if len(email.Attachments) != 1 || email.Attachments[0].Filename != "receipt.pdf" {
		t.Fatalf("notice attachments = %+v", email.Attachments)
	}
	files, err := store.NoticeAttachments(context.Background(), email.ID)
	if err != nil || len(files) != 1 || !bytes.Equal(files[0].Content, content) {
		t.Errorf("dispatcher view = %+v %v", files, err)
	}

	// A draft never attached is swept once old; the attached one stays.
	old, _ := svc.Upload(ctxA, application.UploadInput{DisputeID: d1.View.ID, Filename: "old.png", ContentType: "image/png", Content: []byte("png"), Actor: "an@otp"})
	if _, err := owner.Exec(context.Background(), `UPDATE attachments SET uploaded_at = now() - interval '2 days' WHERE id = $1`, old.ID); err != nil {
		t.Fatal(err)
	}
	n, err := store.PurgeDraftAttachments(context.Background(), time.Now().Add(-application.DraftAttachmentTTL))
	if err != nil || n != 1 {
		t.Errorf("purged %d %v", n, err)
	}
	if _, err := svc.GetAttachment(ctxA, d1.View.ID, a.ID); err != nil {
		t.Errorf("attached file swept: %v", err)
	}
}
