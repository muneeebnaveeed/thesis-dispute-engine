package main

import (
	"context"
	"flag"
	"fmt"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/infrastructure/postgres/sqlcgen"
)

// member is the operator's way to place a person in a team until tenant admins can do it themselves (ADR 0027).
func member(ctx context.Context, pool *pgxpool.Pool, q *sqlcgen.Queries, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: tenant member add|remove|list")
	}
	fs := flag.NewFlagSet("member "+args[0], flag.ContinueOnError)
	slug := fs.String("slug", "", "tenant slug")
	subject := fs.String("subject", "", "the analyst's OIDC subject (their Keycloak user id)")
	team := fs.String("team", "", "team slug")
	role := fs.String("role", "", "junior, senior or lead")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	id, err := idForSlug(ctx, q, *slug)
	if err != nil {
		return err
	}
	switch args[0] {
	case "add":
		if *subject == "" || *team == "" {
			return fmt.Errorf("member add needs --subject, --team and --role")
		}
		if err := addMember(ctx, pool, id, *subject, *team, *role); err != nil {
			return err
		}
		fmt.Printf("%s is %s of %s in %s\n", *subject, *role, *team, *slug)
	case "remove":
		n, err := removeMember(ctx, pool, id, *subject, *team)
		if err != nil {
			return err
		}
		fmt.Printf("removed %d membership(s)\n", n)
	case "list":
		ms, err := listMembers(ctx, pool, id)
		if err != nil {
			return err
		}
		for _, m := range ms {
			fmt.Printf("%-16s %-40s %s\n", m[0], m[1], m[2])
		}
	default:
		return fmt.Errorf("unknown member command %q", args[0])
	}
	return nil
}

// addMember puts a subject in a team at a role, or changes their role there; the team must already exist.
func addMember(ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID, subject, team, role string) error {
	if !slices.Contains(application.Roles, application.Role(role)) {
		return fmt.Errorf("role %q is not one of %v", role, application.Roles)
	}
	_, err := pool.Exec(ctx, `INSERT INTO team_members (tenant_id, team, subject, role) VALUES ($1, $2, $3, $4)
		ON CONFLICT (tenant_id, team, subject) DO UPDATE SET role = EXCLUDED.role`, tenantID, team, subject, role)
	return err
}

func removeMember(ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID, subject, team string) (int64, error) {
	tag, err := pool.Exec(ctx, `DELETE FROM team_members WHERE tenant_id = $1 AND team = $2 AND subject = $3`, tenantID, team, subject)
	return tag.RowsAffected(), err
}

// listMembers returns team, subject and role for every membership in the tenant.
func listMembers(ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID) ([][3]string, error) {
	rows, err := pool.Query(ctx, `SELECT team, subject, role FROM team_members WHERE tenant_id = $1 ORDER BY team, subject`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out [][3]string
	for rows.Next() {
		var m [3]string
		if err := rows.Scan(&m[0], &m[1], &m[2]); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
