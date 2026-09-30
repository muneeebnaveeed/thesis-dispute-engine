//go:build integration

package postgres_test

import (
	"context"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application/apptest"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
	disputepg "github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/infrastructure/postgres"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/postgres/pgtest"
)

func mustExec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func TestAccessReadsGrantsAndBumpsVersion(t *testing.T) {
	owner, schema := pgtest.PoolWithSchema(t)
	app := pgtest.AppPool(t, schema)
	store := disputepg.NewStore(app)
	ctx := apptest.CtxFor(apptest.TenantA)
	mustExec(t, owner, `INSERT INTO teams (tenant_id, slug, name) VALUES ($1, 'cb', 'Chargebacks')`, apptest.TenantA)
	mustExec(t, owner, `INSERT INTO team_members (tenant_id, team, subject, role) VALUES ($1, 'cb', $2, 'senior')`, apptest.TenantA, apptest.Lead.ID)
	mustExec(t, owner, `INSERT INTO role_grants (tenant_id, team, role, action, amount_limit) VALUES ($1, 'cb', 'junior', 'ISSUE_FINAL_CREDIT', 500)`, apptest.TenantA)

	read := func() application.Access {
		t.Helper()
		var a application.Access
		if err := store.WithTx(ctx, func(tx application.Tx) (err error) { a, err = tx.Access(ctx, apptest.Lead); return err }); err != nil {
			t.Fatal(err)
		}
		return a
	}
	before := read()
	if !slices.Contains(before.Teams, "cb") || before.DefaultTeam != "general" {
		t.Fatalf("teams = %v default %q", before.Teams, before.DefaultTeam)
	}
	if !slices.Contains(before.Members, application.Membership{Team: "cb", Role: application.RoleSenior}) {
		t.Fatalf("members = %v", before.Members)
	}
	var limited bool
	for _, g := range before.Grants {
		limited = limited || (g.Team == "cb" && g.AmountLimit != nil && g.AmountLimit.String() == "500")
	}
	if !limited {
		t.Fatalf("grants = %+v", before.Grants)
	}
	mustExec(t, owner, `DELETE FROM role_grants WHERE tenant_id = $1 AND team = 'cb'`, apptest.TenantA)
	if after := read(); after.Version <= before.Version {
		t.Fatalf("version %d -> %d", before.Version, after.Version)
	}
}

// countingTracer counts statements that read the grants table.
type countingTracer struct{ grants atomic.Int64 }

func (c *countingTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	if strings.Contains(d.SQL, "role_grants") {
		c.grants.Add(1)
	}
	return ctx
}

func (c *countingTracer) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

// One transaction reads a tenant's access data once, however many decisions it makes (the apply and every allowed
// event in the view it returns).
func TestAccessIsReadOncePerTransaction(t *testing.T) {
	owner, schema := pgtest.PoolWithSchema(t)
	app := pgtest.AppPool(t, schema)
	cfg := app.Config().Copy()
	tracer := &countingTracer{}
	cfg.ConnConfig.Tracer = tracer
	traced, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(traced.Close)
	svc := apptest.NewService(t, disputepg.NewStore(traced), nil)
	txn := seedFor(t, owner, apptest.TenantA, domain.RailCard, "EUR")
	created, err := svc.CreateDispute(apptest.Ctx(), application.CreateDisputeInput{TransactionID: txn})
	if err != nil {
		t.Fatal(err)
	}
	tracer.grants.Store(0)
	if _, err := svc.ApplyEvent(apptest.Ctx(), application.ApplyEventInput{DisputeID: created.View.ID, Event: domain.EventOpenInvestigation}); err != nil {
		t.Fatal(err)
	}
	if n := tracer.grants.Load(); n != 1 {
		t.Fatalf("grants read %d times in one apply", n)
	}
}

// The migration seeds every new tenant's default grants in SQL; they must be exactly Go's DefaultAccess, or a new
// domain event would silently be missing from new tenants.
func TestNewTenantGrantsMatchDefaultAccess(t *testing.T) {
	owner, _ := pgtest.PoolWithSchema(t)
	fresh := uuid.New()
	mustExec(t, owner, `INSERT INTO tenants (id, name, slug) VALUES ($1, 'fresh', $2)`, fresh, "t"+strings.ReplaceAll(fresh.String(), "-", "")[:20])
	rows, err := owner.Query(context.Background(), `SELECT team, role, action FROM role_grants WHERE tenant_id = $1 AND amount_limit IS NULL`, fresh)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for rows.Next() {
		var team, role, action string
		if err := rows.Scan(&team, &role, &action); err != nil {
			t.Fatal(err)
		}
		got = append(got, team+"/"+role+"/"+action)
	}
	defaults := application.DefaultAccess(apptest.Lead).Grants
	want := make([]string, 0, len(defaults))
	for _, g := range defaults {
		want = append(want, g.Team+"/"+string(g.Role)+"/"+string(g.Action))
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("migration default grants differ from DefaultAccess\n sql: %v\n  go: %v", got, want)
	}
}
