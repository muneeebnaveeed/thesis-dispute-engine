// Package pgtest gives integration tests a migrated, isolated schema; tests skip when DISPUTE_TEST_DATABASE_URL is unset.
package pgtest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/postgres"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/migrations"
)

// EnvVar names the owner connection string integration tests use; AppEnvVar the dispute_api login for privilege tests.
const (
	EnvVar    = "DISPUTE_TEST_DATABASE_URL"
	AppEnvVar = "DISPUTE_TEST_APP_DATABASE_URL"
)

// Pool returns a pool whose search_path is a fresh schema with all migrations applied; the schema is dropped on cleanup.
func Pool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, _ := PoolWithSchema(t)
	return pool
}

// AppPool connects as the least-privileged API login to the schema an owner pool was created in; skips without AppEnvVar.
func AppPool(t *testing.T, schema string) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv(AppEnvVar)
	if url == "" {
		t.Skipf("%s not set", AppEnvVar)
	}
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := postgres.ConnectWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("connect as app role: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// PoolWithSchema is Pool plus the schema name, for tests that open a second connection into it.
func PoolWithSchema(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	pool, schema := EmptyPool(t)
	files, err := postgres.Load(migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := postgres.Migrate(context.Background(), pool, files); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return pool, schema
}

// EmptyPool is PoolWithSchema without the migrations, for tests that apply them step by step.
func EmptyPool(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	url := os.Getenv(EnvVar)
	if url == "" {
		t.Skipf("%s not set", EnvVar)
	}
	ctx := context.Background()

	var b [6]byte
	_, _ = rand.Read(b[:])
	schema := "t_" + hex.EncodeToString(b[:])

	admin, err := postgres.Connect(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if _, err := admin.Exec(ctx, fmt.Sprintf(`CREATE SCHEMA %s`, schema)); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), fmt.Sprintf(`DROP SCHEMA %s CASCADE`, schema))
		admin.Close()
	})

	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := postgres.ConnectWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("connect to schema: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool, schema
}
