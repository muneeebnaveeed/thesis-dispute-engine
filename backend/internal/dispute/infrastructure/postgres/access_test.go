//go:build integration

package postgres_test

import (
	"context"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application/apptest"
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
