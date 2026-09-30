//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application/apptest"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
	disputepg "github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/infrastructure/postgres"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/postgres/pgtest"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/principal"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/tenant"
)

func TestTeamScopeUnderRLS(t *testing.T) {
	owner, schema := pgtest.PoolWithSchema(t)
	app := pgtest.AppPool(t, schema)
	svc := apptest.NewService(t, disputepg.NewStore(app), nil)
	txn := seedFor(t, owner, apptest.TenantA, domain.RailCard, "EUR")
	seedFor(t, owner, apptest.TenantB, domain.RailCard, "EUR")
	mustExec(t, owner, `INSERT INTO teams (tenant_id, slug, name) VALUES ($1, 'fraud', 'Fraud')`, apptest.TenantA)
	mustExec(t, owner, `INSERT INTO team_members (tenant_id, team, subject, role) VALUES ($1, 'fraud', 'fraud-only', 'lead')`, apptest.TenantA)
	created, err := svc.CreateDispute(apptest.Ctx(), application.CreateDisputeInput{TransactionID: txn})
	if err != nil {
		t.Fatal(err)
	}
	id := created.View.ID

	outsider := apptest.CtxAs(principal.Principal{Kind: principal.Analyst, ID: "fraud-only", Display: "f@x"})
	if _, err := svc.GetDispute(outsider, id); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("other team read: %v", err)
	}
	page, err := svc.ListDisputes(outsider, application.ListQuery{Limit: 50})
	if err != nil || len(page.Items) != 0 {
		t.Fatalf("other team list: %d items, %v", len(page.Items), err)
	}
	if _, err := svc.ApplyEvent(outsider, application.ApplyEventInput{DisputeID: id, Event: domain.EventOpenInvestigation}); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("other team write: %v", err)
	}
	key := apptest.CtxAs(principal.Principal{Kind: principal.Key, ID: uuid.NewString(), Display: "key:tk_x"})
	if _, err := svc.GetDispute(key, id); err != nil {
		t.Fatalf("key read: %v", err)
	}
	// restrictive, not permissive: team scope must never widen tenant isolation
	otherTenant := principal.With(tenant.WithID(context.Background(), apptest.TenantB), apptest.Lead)
	if _, err := svc.GetDispute(otherTenant, id); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("cross-tenant read: %v", err)
	}
	// child rows follow their dispute: a raw read as the outsider sees none of its events
	var events int
	tx, err := app.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := tx.QueryRow(context.Background(), `SELECT set_config('app.tenant_id', $1, true), set_config('app.subject', 'fraud-only', true),
		set_config('app.principal_kind', 'analyst', true)`, apptest.TenantA.String()).Scan(new(string), new(string), new(string)); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(context.Background(), `SELECT count(*) FROM dispute_events WHERE dispute_id = $1`, id).Scan(&events); err != nil || events != 0 {
		t.Fatalf("outsider sees %d events (%v)", events, err)
	}
}

func TestReassignOutOfOwnTeamsThroughPostgres(t *testing.T) {
	owner, schema := pgtest.PoolWithSchema(t)
	app := pgtest.AppPool(t, schema)
	svc := apptest.NewService(t, disputepg.NewStore(app), nil)
	txn := seedFor(t, owner, apptest.TenantA, domain.RailCard, "EUR")
	mustExec(t, owner, `INSERT INTO teams (tenant_id, slug, name) VALUES ($1, 'fraud', 'Fraud')`, apptest.TenantA)
	mustExec(t, owner, `INSERT INTO team_members (tenant_id, team, subject, role) VALUES ($1, 'fraud', 'fraud-only', 'lead')`, apptest.TenantA)
	created, err := svc.CreateDispute(apptest.Ctx(), application.CreateDisputeInput{TransactionID: txn})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Reassign(apptest.Ctx(), created.View.ID, "fraud"); err != nil {
		t.Fatalf("reassign: %v", err)
	}
	fraud := apptest.CtxAs(principal.Principal{Kind: principal.Analyst, ID: "fraud-only", Display: "f@x"})
	view, err := svc.GetDispute(fraud, created.View.ID)
	if err != nil {
		t.Fatalf("new team read: %v", err)
	}
	if last := view.Events[len(view.Events)-1]; last.Event != application.EventReassigned {
		t.Fatalf("last event = %s", last.Event)
	}
	if _, err := svc.GetDispute(apptest.Ctx(), created.View.ID); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("old team still reads it: %v", err)
	}
}
