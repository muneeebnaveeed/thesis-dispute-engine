package main

import (
	"context"
	"embed"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/authz"
)

//go:embed access/*.yaml
var accessFixtures embed.FS

func accessFixture(slug string) (authz.Fixture, error) {
	b, err := accessFixtures.ReadFile("access/" + slug + ".yaml")
	if err != nil {
		return authz.Fixture{}, err
	}
	return authz.ParseFixture(b)
}

// seedAccess replaces a tenant's access data with its fixture in one transaction, so rerunning gives the same rows.
func seedAccess(ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID, slug string) error {
	f, err := accessFixture(slug)
	if err != nil {
		return fmt.Errorf("access %s: %w", slug, err)
	}
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		// members added by an operator (cmd/tenant member add) survive a reseed; only the fixture's own subjects are replaced
		subjects := make([]string, 0, len(f.Members))
		for _, m := range f.Members {
			subjects = append(subjects, m.Subject)
		}
		for _, stmt := range []string{`DELETE FROM routing_rules WHERE tenant_id = $1`, `DELETE FROM role_grants WHERE tenant_id = $1`,
			`UPDATE teams SET is_default = false WHERE tenant_id = $1`} {
			if _, err := tx.Exec(ctx, stmt, tenantID); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `DELETE FROM team_members WHERE tenant_id = $1 AND subject = ANY($2)`, tenantID, subjects); err != nil {
			return err
		}
		for _, t := range f.Teams {
			if _, err := tx.Exec(ctx, `INSERT INTO teams (tenant_id, slug, name, is_default) VALUES ($1, $2, $3, $4)
				ON CONFLICT (tenant_id, slug) DO UPDATE SET name = EXCLUDED.name, is_default = EXCLUDED.is_default`,
				tenantID, t.Slug, t.Name, t.Default); err != nil {
				return err
			}
		}
		for _, m := range f.Members {
			if _, err := tx.Exec(ctx, `INSERT INTO team_members (tenant_id, team, subject, role) VALUES ($1, $2, $3, $4)`,
				tenantID, m.Team, m.Subject, string(m.Role)); err != nil {
				return err
			}
		}
		for _, g := range f.Grants {
			if _, err := tx.Exec(ctx, `INSERT INTO role_grants (tenant_id, team, role, action, amount_limit) VALUES ($1, $2, $3, $4, $5)`,
				tenantID, g.Team, string(g.Role), string(g.Action), g.AmountLimit); err != nil {
				return err
			}
		}
		for i, r := range f.Routing {
			if _, err := tx.Exec(ctx, `INSERT INTO routing_rules (tenant_id, position, rail, reason, risk_tier, min_amount, team)
				VALUES ($1, $2, $3, $4, $5, $6, $7)`, tenantID, i+1, r.Rail, r.Reason, r.RiskTier, r.MinAmount, r.Team); err != nil {
				return err
			}
		}
		return nil
	})
}
