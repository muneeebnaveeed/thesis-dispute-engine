// Command tenant manages tenant rows as the schema owner: the API-side half of onboarding and offboarding
// (scripts/tenant drives it together with Keycloak and tenantkey).
//
//	tenant upsert --id <uuid> --slug <slug> --name <text> [--issuer <url>] [--domains a.com,b.com]
//	tenant disable --slug <slug>       credentials stop resolving on the next request; data is kept
//	tenant enable  --slug <slug>
//	tenant list
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/infrastructure/postgres/sqlcgen"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/config"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/postgres"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "tenant:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: tenant upsert|disable|enable|list")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx := context.Background()
	pool, err := postgres.Connect(ctx, cfg.MigrateDatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	q := sqlcgen.New(pool)

	switch args[0] {
	case "upsert":
		fs := flag.NewFlagSet("upsert", flag.ContinueOnError)
		idRaw := fs.String("id", "", "tenant UUID (a new v7 id when omitted)")
		slug := fs.String("slug", "", "realm name and URL segment")
		name := fs.String("name", "", "display name")
		issuer := fs.String("issuer", "", "OIDC issuer; defaults to KEYCLOAK_URL/realms/<slug>")
		domains := fs.String("domains", "", "comma-separated work-email domains")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *slug == "" || *name == "" {
			return fmt.Errorf("upsert needs --slug and --name")
		}
		id, err := parseOrNewID(*idRaw)
		if err != nil {
			return err
		}
		iss := *issuer
		if iss == "" {
			iss = strings.TrimRight(getenv("KEYCLOAK_URL", "http://localhost:8180"), "/") + "/realms/" + *slug
		}
		if err := q.UpsertTenant(ctx, sqlcgen.UpsertTenantParams{ID: id, Name: *name, Slug: *slug, OidcIssuer: &iss}); err != nil {
			return err
		}
		if _, err := q.SetTenantEmailDomains(ctx, sqlcgen.SetTenantEmailDomainsParams{ID: id, EmailDomains: splitList(*domains)}); err != nil {
			return err
		}
		fmt.Printf("id:     %s\nslug:   %s\nissuer: %s\n", id, *slug, iss)
	case "disable", "enable":
		fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
		slug := fs.String("slug", "", "tenant slug")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		id, err := idForSlug(ctx, q, *slug)
		if err != nil {
			return err
		}
		n, err := q.SetTenantDisabled(ctx, sqlcgen.SetTenantDisabledParams{ID: id, Disabled: args[0] == "disable"})
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("no tenant %q", *slug)
		}
		fmt.Printf("%sd %s (%s)\n", args[0], *slug, id)
	case "list":
		rows, err := q.ListTenants(ctx)
		if err != nil {
			return err
		}
		for _, r := range rows {
			iss, state := "-", "active"
			if r.OidcIssuer != nil {
				iss = *r.OidcIssuer
			}
			if r.DisabledAt.Valid {
				state = "disabled " + r.DisabledAt.Time.Format("2006-01-02")
			}
			fmt.Printf("%s  %-10s %-20s %-45s %s\n", r.ID, r.Slug, r.Name, iss, state)
		}
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
	return nil
}

func idForSlug(ctx context.Context, q *sqlcgen.Queries, slug string) (uuid.UUID, error) {
	if slug == "" {
		return uuid.Nil, fmt.Errorf("--slug is required")
	}
	rows, err := q.ListTenants(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	for _, r := range rows {
		if r.Slug == slug {
			return r.ID, nil
		}
	}
	return uuid.Nil, fmt.Errorf("no tenant %q", slug)
}

func parseOrNewID(raw string) (uuid.UUID, error) {
	if raw == "" {
		return uuid.NewV7()
	}
	return uuid.Parse(raw)
}

func splitList(raw string) []string {
	out := []string{}
	for _, s := range strings.Split(raw, ",") {
		if s = strings.ToLower(strings.TrimSpace(s)); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
