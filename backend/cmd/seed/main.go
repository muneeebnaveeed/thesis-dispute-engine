// Command seed inserts fixed accounts and transactions for local development; rerunning is a no-op.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/infrastructure/postgres/sqlcgen"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/config"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/postgres"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/migrations"
)

type txn struct {
	id, account string
	rail        string
	amount      string
	currency    string
	merchant    string
	daysAgo     int
}

// Fixed IDs so scripts and docs can refer to them; the version nibble is 8 to keep them out of the v7 space.
var (
	accountEUR = "00000000-0000-8000-8000-000000000001"
	accountUSD = "00000000-0000-8000-8000-000000000002"
	seedTxns   = []txn{
		{"00000000-0000-8000-8000-000000000101", accountEUR, "CARD", "125.40", "EUR", "Ryanair", 3},
		{"00000000-0000-8000-8000-000000000102", accountEUR, "CARD", "1899.00", "EUR", "MediaMarkt", 12},
		{"00000000-0000-8000-8000-000000000103", accountEUR, "SEPA_DD", "49.99", "EUR", "Vodafone Hungary", 20},
		{"00000000-0000-8000-8000-000000000201", accountUSD, "CARD", "89.10", "USD", "Amazon", 5},
		{"00000000-0000-8000-8000-000000000202", accountUSD, "CREDIT_CARD", "412.00", "USD", "Delta Air Lines", 9},
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
	pool, err := postgres.Connect(ctx, cfg.DatabaseURL)
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

	for _, a := range []struct{ id, holder, currency string }{
		{accountEUR, "Kovács Anna", "EUR"},
		{accountUSD, "Jordan Lee", "USD"},
	} {
		if err := q.InsertAccount(ctx, sqlcgen.InsertAccountParams{ID: uuid.MustParse(a.id), HolderName: a.holder, Currency: a.currency}); err != nil {
			return err
		}
	}
	for _, t := range seedTxns {
		if err := q.InsertTransaction(ctx, sqlcgen.InsertTransactionParams{
			ID: uuid.MustParse(t.id), AccountID: uuid.MustParse(t.account), Rail: t.rail,
			Amount: decimal.RequireFromString(t.amount), Currency: t.currency, Merchant: t.merchant,
			OccurredAt: time.Now().AddDate(0, 0, -t.daysAgo),
		}); err != nil {
			return err
		}
	}
	fmt.Printf("seeded 2 accounts and %d transactions\n", len(seedTxns))
	for _, t := range seedTxns {
		fmt.Printf("  %s  %-11s %8s %s  %s\n", t.id, t.rail, t.amount, t.currency, t.merchant)
	}
	return nil
}
