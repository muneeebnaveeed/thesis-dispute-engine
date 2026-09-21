// Command tenantkey issues, inspects and revokes tenant keys as the schema owner. The secret is printed once.
//
//	tenantkey create --tenant <uuid> --label <text> [--expires 2027-03-01]
//	tenantkey list
//	tenantkey find <key or prefix>      which row a leaked value belongs to; prints nothing secret
//	tenantkey revoke --id <uuid> | --prefix <text>
//	tenantkey audit [--stale 180] [--idle 30]   exit 1 when live keys are older than --stale days or unused for --idle days
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/infrastructure/postgres/sqlcgen"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/auth"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/config"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/postgres"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "tenantkey:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: tenantkey create|list|find|revoke")
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
	case "create":
		return create(ctx, q, args[1:])
	case "list":
		keys, err := q.ListTenantKeys(ctx)
		if err != nil {
			return err
		}
		fmt.Printf("%-36s  %-36s  %-12s  %-20s  %-10s  %-10s  %s\n", "id", "tenant", "prefix", "label", "last used", "expires", "state")
		for _, k := range keys {
			fmt.Printf("%s  %s  %-12s  %-20s  %-10s  %-10s  %s\n", k.ID, k.TenantID, k.Prefix, k.Label, day(k.LastUsedAt), day(k.ExpiresAt), state(k.RevokedAt, k.ExpiresAt))
		}
	case "find":
		if len(args) != 2 {
			return fmt.Errorf("find needs the key or its prefix")
		}
		return find(ctx, q, args[1])
	case "revoke":
		return revoke(ctx, q, args[1:])
	case "audit":
		return audit(ctx, q, args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
	return nil
}

func create(ctx context.Context, q *sqlcgen.Queries, args []string) error {
	fs := flag.NewFlagSet("create", flag.ContinueOnError)
	tenantRaw := fs.String("tenant", "", "tenant UUID")
	label := fs.String("label", "", "who or what holds this key")
	expires := fs.String("expires", "", "YYYY-MM-DD after which the key stops working")
	if err := fs.Parse(args); err != nil {
		return err
	}
	tenantID, err := uuid.Parse(*tenantRaw)
	if err != nil || *label == "" {
		return fmt.Errorf("create needs --tenant <uuid> and --label <text>")
	}
	var expiresAt pgtype.Timestamptz
	if *expires != "" {
		t, err := time.Parse("2006-01-02", *expires)
		if err != nil || !t.After(time.Now()) {
			return fmt.Errorf("--expires must be a future YYYY-MM-DD")
		}
		expiresAt = pgtype.Timestamptz{Time: t, Valid: true}
	}
	secret, err := auth.NewSecret()
	if err != nil {
		return err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return err
	}
	if err := q.InsertTenantKey(ctx, sqlcgen.InsertTenantKeyParams{
		ID: id, TenantID: tenantID, KeyHash: auth.HashKey(secret), Prefix: auth.Prefix(secret), Label: *label, ExpiresAt: expiresAt,
	}); err != nil {
		return err
	}
	fmt.Printf("id:     %s\nprefix: %s\nkey:    %s\n\nStore the key now; it is not recoverable.\n", id, auth.Prefix(secret), secret)
	return nil
}

func find(ctx context.Context, q *sqlcgen.Queries, value string) error {
	value = strings.TrimSpace(value)
	// A full key matches exactly through its hash; anything shorter is treated as a prefix.
	if len(value) > auth.PrefixLen {
		row, err := q.FindTenantKeyByHash(ctx, auth.HashKey(value))
		if err == nil {
			fmt.Printf("%s  tenant %s  %-20s %s  (exact match)\n", row.ID, row.TenantID, row.Label, state(row.RevokedAt, pgtype.Timestamptz{}))
			return nil
		}
		value = auth.Prefix(value)
	}
	rows, err := q.FindTenantKeysByPrefix(ctx, value)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return fmt.Errorf("no key with prefix %q", value)
	}
	for _, row := range rows {
		fmt.Printf("%s  tenant %s  %-20s %s\n", row.ID, row.TenantID, row.Label, state(row.RevokedAt, pgtype.Timestamptz{}))
	}
	return nil
}

func revoke(ctx context.Context, q *sqlcgen.Queries, args []string) error {
	fs := flag.NewFlagSet("revoke", flag.ContinueOnError)
	idRaw := fs.String("id", "", "key id")
	prefix := fs.String("prefix", "", "key prefix, as shown by list or find")
	if err := fs.Parse(args); err != nil {
		return err
	}
	var id uuid.UUID
	switch {
	case *idRaw != "":
		var err error
		if id, err = uuid.Parse(*idRaw); err != nil {
			return fmt.Errorf("--id must be a UUID")
		}
	case *prefix != "":
		rows, err := q.FindTenantKeysByPrefix(ctx, *prefix)
		if err != nil {
			return err
		}
		var live []uuid.UUID
		for _, r := range rows {
			if !r.RevokedAt.Valid {
				live = append(live, r.ID)
			}
		}
		if len(live) != 1 {
			return fmt.Errorf("prefix %q matches %d live keys; revoke by --id", *prefix, len(live))
		}
		id = live[0]
	default:
		return fmt.Errorf("revoke needs --id <uuid> or --prefix <text>")
	}
	n, err := q.RevokeTenantKey(ctx, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("no live key with id %s", id)
	}
	fmt.Println("revoked", id)
	return nil
}

// audit is meant for a cron: it lists live keys that should be rotated or revoked and fails when there are any.
func audit(ctx context.Context, q *sqlcgen.Queries, args []string) error {
	fs := flag.NewFlagSet("audit", flag.ContinueOnError)
	stale := fs.Int("stale", 180, "days after which a live key should be rotated")
	idle := fs.Int("idle", 30, "days without use after which a live key should be revoked")
	if err := fs.Parse(args); err != nil {
		return err
	}
	keys, err := q.ListTenantKeys(ctx)
	if err != nil {
		return err
	}
	now := time.Now()
	findings := 0
	for _, k := range keys {
		if k.RevokedAt.Valid || (k.ExpiresAt.Valid && !k.ExpiresAt.Time.After(now)) {
			continue
		}
		var why []string
		if now.Sub(k.CreatedAt) > time.Duration(*stale)*24*time.Hour {
			why = append(why, fmt.Sprintf("created %d days ago", int(now.Sub(k.CreatedAt).Hours()/24)))
		}
		last := k.CreatedAt
		if k.LastUsedAt.Valid {
			last = k.LastUsedAt.Time
		}
		if now.Sub(last) > time.Duration(*idle)*24*time.Hour {
			why = append(why, fmt.Sprintf("unused for %d days", int(now.Sub(last).Hours()/24)))
		}
		if len(why) > 0 {
			findings++
			fmt.Printf("%s  tenant %s  %-12s  %-20s %s\n", k.ID, k.TenantID, k.Prefix, k.Label, strings.Join(why, ", "))
		}
	}
	if findings > 0 {
		return fmt.Errorf("%d key(s) need attention", findings)
	}
	fmt.Println("no stale or idle keys")
	return nil
}

func day(t pgtype.Timestamptz) string {
	if !t.Valid {
		return "-"
	}
	return t.Time.Format("2006-01-02")
}

func state(revoked, expires pgtype.Timestamptz) string {
	switch {
	case revoked.Valid:
		return "revoked " + revoked.Time.Format("2006-01-02")
	case expires.Valid && !expires.Time.After(time.Now()):
		return "expired"
	default:
		return "live"
	}
}
