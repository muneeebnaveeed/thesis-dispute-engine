// Command seed inserts fixed accounts and transactions for local development; rerunning is a no-op.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/shopspring/decimal"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/infrastructure/postgres/sqlcgen"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/auth"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/config"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/postgres"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/migrations"
)

type txn struct {
	id, tenant, account string
	rail                string
	amount              string
	currency            string
	merchant            string
	daysAgo             int
}

// Fixed IDs so scripts and docs can refer to them; the version nibble is 8 to keep them out of the v7 space.
var (
	// tenantA is the default tenant every pre-tenancy row was assigned to (migration 0003) and the one the API
	// serves in single-tenant mode; tenantB exists so isolation can be demonstrated.
	tenantA    = "00000000-0000-8000-8000-00000000a001"
	tenantB    = "00000000-0000-8000-8000-00000000a002"
	devKeyA    = "tk_dev_tenant_a"
	devKeyB    = "tk_dev_tenant_b"
	accountEUR = "00000000-0000-8000-8000-000000000001"
	accountUSD = "00000000-0000-8000-8000-000000000002"
	accountB   = "00000000-0000-8000-8000-000000000003"
	seedTxns   = []txn{
		{"00000000-0000-8000-8000-000000000101", tenantA, accountEUR, "CARD", "125.40", "EUR", "Ryanair", 3},
		{"00000000-0000-8000-8000-000000000102", tenantA, accountEUR, "CARD", "1899.00", "EUR", "MediaMarkt", 12},
		{"00000000-0000-8000-8000-000000000103", tenantA, accountEUR, "SEPA_DD", "49.99", "EUR", "Vodafone Hungary", 20},
		{"00000000-0000-8000-8000-000000000201", tenantA, accountUSD, "CARD", "89.10", "USD", "Amazon", 5},
		{"00000000-0000-8000-8000-000000000202", tenantA, accountUSD, "CREDIT_CARD", "412.00", "USD", "Delta Air Lines", 9},
		{"00000000-0000-8000-8000-000000000301", tenantB, accountB, "CARD", "64.00", "EUR", "Bolt", 2},
		{"00000000-0000-8000-8000-000000000302", tenantB, accountB, "SEPA_DD", "29.90", "EUR", "Telekom", 15},
	}
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "seed:", err)
		os.Exit(1)
	}
}

func run() error {
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
	files, err := postgres.Load(migrations.FS)
	if err != nil {
		return err
	}
	if _, err := postgres.Migrate(ctx, pool, files); err != nil {
		return err
	}
	q := sqlcgen.New(pool)

	// Slugs double as Keycloak realm names (deploy/keycloak/render-realms.sh renders one realm per row here).
	keycloak := getenv("KEYCLOAK_URL", "http://localhost:8180")
	for _, t := range []struct{ id, name, slug, domain string }{{tenantA, "OTP Bank", "otp", "otpbank.hu"}, {tenantB, "Erste Bank", "erste", "erstebank.hu"}} {
		issuer := keycloak + "/realms/" + t.slug
		if err := q.UpsertTenant(ctx, sqlcgen.UpsertTenantParams{ID: uuid.MustParse(t.id), Name: t.name, Slug: t.slug, OidcIssuer: &issuer}); err != nil {
			return err
		}
		// Work-email domains let the sign-in page find the tenant without a link (docs/authentication.md).
		if _, err := q.SetTenantEmailDomains(ctx, sqlcgen.SetTenantEmailDomainsParams{ID: uuid.MustParse(t.id), EmailDomains: []string{t.domain}}); err != nil {
			return err
		}
	}
	// Fixed dev keys so the probe, docs and curl examples can use them; production keys come from cmd/tenantkey.
	for _, k := range []struct{ id, tenant, secret string }{
		{"00000000-0000-8000-8000-00000000c001", tenantA, devKeyA},
		{"00000000-0000-8000-8000-00000000c002", tenantB, devKeyB},
	} {
		if err := q.InsertTenantKey(ctx, sqlcgen.InsertTenantKeyParams{ID: uuid.MustParse(k.id), TenantID: uuid.MustParse(k.tenant), KeyHash: auth.HashKey(k.secret), Prefix: auth.Prefix(k.secret), Label: "dev"}); err != nil && !isUniqueViolation(err) {
			return err
		}
	}
	for _, a := range []struct{ id, tenant, holder, currency string }{
		{accountEUR, tenantA, "Kovács Anna", "EUR"},
		{accountUSD, tenantA, "Jordan Lee", "USD"},
		{accountB, tenantB, "Nagy Péter", "EUR"},
	} {
		if err := q.InsertAccount(ctx, sqlcgen.InsertAccountParams{ID: uuid.MustParse(a.id), TenantID: uuid.MustParse(a.tenant), HolderName: a.holder, Currency: a.currency}); err != nil {
			return err
		}
	}
	for _, t := range seedTxns {
		if err := q.InsertTransaction(ctx, sqlcgen.InsertTransactionParams{
			ID: uuid.MustParse(t.id), TenantID: uuid.MustParse(t.tenant), AccountID: uuid.MustParse(t.account), Rail: t.rail,
			Amount: decimal.RequireFromString(t.amount), Currency: t.currency, Merchant: t.merchant,
			OccurredAt: time.Now().AddDate(0, 0, -t.daysAgo),
		}); err != nil {
			return err
		}
	}
	fmt.Printf("seeded 2 tenants (otp, erste; tenant keys %s, %s; analyst/analyst in each realm), 3 accounts and %d transactions\n", devKeyA, devKeyB, len(seedTxns))
	for _, t := range seedTxns {
		fmt.Printf("  %s  tenant %s  %-11s %8s %s  %s\n", t.id, t.tenant[len(t.tenant)-4:], t.rail, t.amount, t.currency, t.merchant)
	}
	return nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
