package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application/apptest"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
	disputepg "github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/infrastructure/postgres"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/postgres/pgtest"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/tenant"
)

// Runs as dispute_api: the clocks are rows behind row-level security, the calendar comes from tenants.settings,
// and the overdue view counts across tenants for the gauge.
func TestDeadlinesThroughPostgres(t *testing.T) {
	owner, schema := pgtest.PoolWithSchema(t)
	app := pgtest.AppPool(t, schema)
	store := disputepg.NewStore(app)
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC) // a Monday
	svc, err := application.NewService(store, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	ctxA := tenant.WithID(context.Background(), apptest.TenantA)
	ctxB := tenant.WithID(context.Background(), apptest.TenantB)
	txnA := seedFor(t, owner, apptest.TenantA, domain.RailCard, "EUR")
	txnB := seedFor(t, owner, apptest.TenantB, domain.RailCard, "EUR")
	if _, err := owner.Exec(context.Background(),
		`UPDATE tenants SET settings = '{"timezone":"Europe/Budapest","holidays":["2026-09-22"]}' WHERE id = $1`, apptest.TenantA); err != nil {
		t.Fatal(err)
	}

	created, err := svc.CreateDispute(ctxA, application.CreateDisputeInput{TransactionID: txnA})
	if err != nil {
		t.Fatal(err)
	}
	if len(created.View.Deadlines) != 2 {
		t.Fatalf("deadlines = %+v", created.View.Deadlines)
	}
	budapest, _ := time.LoadLocation("Europe/Budapest")
	for _, d := range created.View.Deadlines {
		if d.Kind == domain.DeadlineRefund {
			// Tuesday is a holiday for this tenant, so one business day ends Wednesday, Budapest time.
			if got := d.DueAt.In(budapest).Format("2006-01-02 15:04"); got != "2026-09-23 23:59" {
				t.Errorf("refund due %s", got)
			}
		}
	}
	other, err := svc.CreateDispute(ctxB, application.CreateDisputeInput{TransactionID: txnB})
	if err != nil {
		t.Fatal(err)
	}
	// Tenant B has no calendar: UTC and weekends only, so the refund is due Tuesday.
	for _, d := range other.View.Deadlines {
		if d.Kind == domain.DeadlineRefund && d.DueAt.UTC().Format("2006-01-02") != "2026-09-22" {
			t.Errorf("tenant B refund due %s", d.DueAt.UTC())
		}
	}

	// Past every due time: A's dispute is overdue for A, invisible to B, and the view counts both tenants.
	now = now.Add(30 * 24 * time.Hour)
	pageA, err := svc.ListDisputes(ctxA, application.ListQuery{Limit: 10, Overdue: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(pageA.Items) != 1 || pageA.Items[0].ID != created.View.ID {
		t.Fatalf("overdue for A = %+v", pageA.Items)
	}
	if nd := pageA.Items[0].NextDeadline; nd == nil || nd.Kind != domain.DeadlineRefund || nd.Status != domain.DeadlineBreached {
		t.Errorf("next deadline = %+v", nd)
	}
	pageB, _ := svc.ListDisputes(ctxB, application.ListQuery{Limit: 10, Overdue: true})
	if len(pageB.Items) != 1 || pageB.Items[0].ID != other.View.ID {
		t.Errorf("overdue for B = %+v", pageB.Items)
	}
	counts, err := store.CountOverdue(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var total int64
	tenants := map[string]bool{}
	for _, c := range counts {
		total += c.N
		tenants[c.TenantID.String()] = true
	}
	// The view compares against the database clock, which is before every due time computed for "now" above,
	// so nothing is overdue yet by its reckoning; the query must still run as dispute_api and cross tenants.
	if total != 0 {
		t.Errorf("overdue by database time = %d, want 0", total)
	}

	// Settling a month on: the credit and the closing both come after their due times; both survive a fresh read.
	for _, e := range []domain.Event{domain.EventOpenInvestigation, domain.EventIssueRefund, domain.EventClose} {
		if _, err := svc.ApplyEvent(ctxA, application.ApplyEventInput{DisputeID: created.View.ID, Event: e}); err != nil {
			t.Fatalf("%s: %v", e, err)
		}
	}
	view, err := svc.GetDispute(ctxA, created.View.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range view.Deadlines {
		switch d.Kind {
		case domain.DeadlineRefund:
			if d.Status != domain.DeadlineLate || d.MetAt == nil {
				t.Errorf("refund = %+v", d)
			}
		case domain.DeadlineResolution:
			if d.Status != domain.DeadlineLate || d.MetAt == nil {
				t.Errorf("resolution = %+v", d)
			}
		case domain.DeadlineAcknowledge:
			t.Errorf("unexpected acknowledgement clock under PSD2")
		}
	}
	after, _ := svc.ListDisputes(ctxA, application.ListQuery{Limit: 10, Overdue: true})
	if len(after.Items) != 0 {
		t.Errorf("still overdue after settling: %+v", after.Items)
	}
}
