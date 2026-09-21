// Command apikey issues and revokes tenant API keys as the schema owner. The secret is printed once.
//
//	apikey create --tenant <uuid> --label <text>
//	apikey revoke --id <uuid>
//	apikey list
package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"os"

	"github.com/google/uuid"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/infrastructure/postgres/sqlcgen"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/auth"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/config"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/postgres"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "apikey:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: apikey create|revoke|list")
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
		fs := flag.NewFlagSet("create", flag.ContinueOnError)
		tenantRaw := fs.String("tenant", "", "tenant UUID")
		label := fs.String("label", "", "who or what holds this key")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		tenantID, err := uuid.Parse(*tenantRaw)
		if err != nil || *label == "" {
			return fmt.Errorf("create needs --tenant <uuid> and --label <text>")
		}
		secret, err := NewSecret()
		if err != nil {
			return err
		}
		id, err := uuid.NewV7()
		if err != nil {
			return err
		}
		if err := q.InsertAPIKey(ctx, sqlcgen.InsertAPIKeyParams{ID: id, TenantID: tenantID, KeyHash: auth.HashKey(secret), Label: *label}); err != nil {
			return err
		}
		fmt.Printf("id:  %s\nkey: %s\n\nStore the key now; it is not recoverable.\n", id, secret)
	case "revoke":
		fs := flag.NewFlagSet("revoke", flag.ContinueOnError)
		idRaw := fs.String("id", "", "key id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		id, err := uuid.Parse(*idRaw)
		if err != nil {
			return fmt.Errorf("revoke needs --id <uuid>")
		}
		n, err := q.RevokeAPIKey(ctx, id)
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("no live key with id %s", id)
		}
		fmt.Println("revoked", id)
	case "list":
		keys, err := q.ListAPIKeys(ctx)
		if err != nil {
			return err
		}
		for _, k := range keys {
			state := "live"
			if k.RevokedAt.Valid {
				state = "revoked " + k.RevokedAt.Time.Format("2006-01-02")
			}
			fmt.Printf("%s  tenant %s  %-20s %s\n", k.ID, k.TenantID, k.Label, state)
		}
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
	return nil
}

// NewSecret returns a 32-byte random key with a recognisable prefix so leaked keys can be grepped for.
func NewSecret() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "dk_" + base64.RawURLEncoding.EncodeToString(b[:]), nil
}
