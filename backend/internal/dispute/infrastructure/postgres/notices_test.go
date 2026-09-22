package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application/apptest"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
	disputepg "github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/infrastructure/postgres"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/notice"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/postgres/pgtest"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/tenant"
)

// As dispute_api: notices are rows the tenant owns; the outbox is claimed across tenants through the owner-defined
// functions, one attempt at a time with backoff, and finished as sent or failed.
func TestNoticeOutboxThroughPostgres(t *testing.T) {
	owner, schema := pgtest.PoolWithSchema(t)
	app := pgtest.AppPool(t, schema)
	store := disputepg.NewStore(app)
	svc, err := application.NewService(store, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctxA := tenant.WithID(context.Background(), apptest.TenantA)
	ctxB := tenant.WithID(context.Background(), apptest.TenantB)
	txnA := seedFor(t, owner, apptest.TenantA, domain.RailCard, "USD") // Reg E: email and letter
	txnB := seedFor(t, owner, apptest.TenantB, domain.RailCard, "EUR")

	a, err := svc.CreateDispute(ctxA, application.CreateDisputeInput{TransactionID: txnA})
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.CreateDispute(ctxB, application.CreateDisputeInput{TransactionID: txnB})
	if err != nil {
		t.Fatal(err)
	}
	if len(a.View.Notices) != 2 || len(b.View.Notices) != 1 {
		t.Fatalf("notices at opening: A %d, B %d", len(a.View.Notices), len(b.View.Notices))
	}
	if _, _, err := svc.GetNotice(ctxB, a.View.ID, a.View.Notices[0].ID); !errors.Is(err, application.ErrNotFound) {
		t.Errorf("cross-tenant notice read: %v", err)
	}

	// The claim crosses tenants and takes only unsent emails: A's email and B's email, never A's letter.
	claimed, err := store.ClaimNotices(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 2 {
		t.Fatalf("claimed %d, want 2: %+v", len(claimed), claimed)
	}
	tenants := map[string]bool{}
	for _, n := range claimed {
		tenants[n.TenantID.String()] = true
		if n.Channel != domain.ChannelEmail || n.Attempts != 1 || n.Recipient != "holder@example.com" {
			t.Errorf("claimed = %+v", n)
		}
	}
	if len(tenants) != 2 {
		t.Errorf("claim did not cross tenants: %v", tenants)
	}
	// Claimed notices are not offered again until their backoff passes.
	if again, _ := store.ClaimNotices(context.Background(), 10); len(again) != 0 {
		t.Errorf("re-claimed %d before backoff", len(again))
	}

	// One fails, one is sent; the dispute shows each outcome.
	if err := store.FinishNotice(context.Background(), claimed[0].ID, "relay refused"); err != nil {
		t.Fatal(err)
	}
	if err := store.FinishNotice(context.Background(), claimed[1].ID, ""); err != nil {
		t.Fatal(err)
	}
	var sent, failed int
	for _, ctx := range []context.Context{ctxA, ctxB} {
		id := a.View.ID
		if ctx == ctxB {
			id = b.View.ID
		}
		view, err := svc.GetDispute(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		for _, n := range view.Notices {
			if n.Channel != domain.ChannelEmail {
				continue
			}
			switch {
			case n.SentAt != nil && n.Error == nil:
				sent++
			case n.SentAt == nil && n.Error != nil && *n.Error == "relay refused":
				failed++
			default:
				t.Errorf("email notice in an odd state: %+v", n)
			}
		}
	}
	if sent != 1 || failed != 1 {
		t.Errorf("sent %d failed %d", sent, failed)
	}
	// Pull the failed one's next attempt forward (as the owner) and it is offered again.
	if _, err := owner.Exec(context.Background(), `UPDATE notices SET next_attempt_at = now() WHERE id = $1`, claimed[0].ID); err != nil {
		t.Fatal(err)
	}
	retry, _ := store.ClaimNotices(context.Background(), 10)
	if len(retry) != 1 || retry[0].ID != claimed[0].ID || retry[0].Attempts != 2 {
		t.Errorf("retry claim = %+v", retry)
	}
}

// A tenant's wording is its own: B never sees A's override, and reverting deletes only A's row.
func TestTenantTemplatesAreIsolated(t *testing.T) {
	owner, schema := pgtest.PoolWithSchema(t)
	app := pgtest.AppPool(t, schema)
	svc, err := application.NewService(disputepg.NewStore(app), nil)
	if err != nil {
		t.Fatal(err)
	}
	ctxA := tenant.WithID(context.Background(), apptest.TenantA)
	ctxB := tenant.WithID(context.Background(), apptest.TenantB)
	seedFor(t, owner, apptest.TenantA, domain.RailCard, "EUR")
	seedFor(t, owner, apptest.TenantB, domain.RailCard, "EUR")
	if _, err := svc.PutTemplateSetting(ctxA, domain.NoticeCustom, notice.Override{Subject: "From A: {{subject}}"}, "admin@a"); err != nil {
		t.Fatal(err)
	}
	a, _ := svc.ListTemplateSettings(ctxA)
	b, _ := svc.ListTemplateSettings(ctxB)
	pick := func(list []application.TemplateSetting) application.TemplateSetting {
		for _, s := range list {
			if s.Base.Kind == domain.NoticeCustom {
				return s
			}
		}
		return application.TemplateSetting{}
	}
	if pick(a).Override == nil || pick(a).Effective.Subject != "From A: {{subject}}" || pick(a).UpdatedBy != "admin@a" {
		t.Errorf("A = %+v", pick(a))
	}
	if pick(b).Override != nil || pick(b).Effective.Subject != pick(b).Base.Subject {
		t.Errorf("B sees A's wording: %+v", pick(b))
	}
	if err := svc.DeleteTemplateSetting(ctxB, domain.NoticeCustom); !errors.Is(err, application.ErrNotFound) {
		t.Errorf("B reverting A's override: %v", err)
	}
	if err := svc.DeleteTemplateSetting(ctxA, domain.NoticeCustom); err != nil {
		t.Errorf("A reverting: %v", err)
	}
}
