package postgres_test

import (
	"context"
	"errors"
	"testing"
	"testing/fstest"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/postgres"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/postgres/pgtest"
)

func TestLoadOrdersAndRejectsDuplicates(t *testing.T) {
	fsys := fstest.MapFS{
		"0002_b.sql":     {Data: []byte("select 2;")},
		"0001_a.sql":     {Data: []byte("select 1;")},
		"README.md":      {Data: []byte("ignored")},
		"0003_c.sql.bak": {Data: []byte("ignored")},
	}
	ms, err := postgres.Load(fsys)
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 2 || ms[0].Version != 1 || ms[1].Version != 2 {
		t.Errorf("migrations = %+v", ms)
	}
	fsys["0002_dup.sql"] = &fstest.MapFile{Data: []byte("select 22;")}
	if _, err := postgres.Load(fsys); err == nil {
		t.Error("duplicate version accepted")
	}
}

func TestMigrateIsIdempotentAndDetectsDrift(t *testing.T) {
	pool := pgtest.Pool(t) // already migrated once
	ctx := context.Background()

	fsys := fstest.MapFS{"0001_x.sql": {Data: []byte("create table drift_probe (id int);")}}
	ms, _ := postgres.Load(fsys)
	// Version 1 is applied with a different checksum: refuse.
	if _, err := postgres.Migrate(ctx, pool, ms); !errors.Is(err, postgres.ErrMigrationModified) {
		t.Fatalf("modified migration: err = %v", err)
	}

	real, _ := postgres.Load(migrationsFS(t))
	n, err := postgres.Migrate(ctx, pool, real)
	if err != nil || n != 0 {
		t.Fatalf("second run: n=%d err=%v", n, err)
	}
}

func TestCheckReportsPendingAndModified(t *testing.T) {
	pool := pgtest.Pool(t) // already migrated once
	ctx := context.Background()

	real, _ := postgres.Load(migrationsFS(t))
	if err := postgres.Check(ctx, pool, real); err != nil {
		t.Fatalf("up to date: err = %v", err)
	}

	ahead := append(append([]postgres.Migration{}, real...), postgres.Migration{Version: 9999, Name: "9999_future.sql", Checksum: "x"})
	if err := postgres.Check(ctx, pool, ahead); !errors.Is(err, postgres.ErrMigrationsPending) {
		t.Fatalf("binary ahead of database: err = %v", err)
	}

	modified := append([]postgres.Migration{}, real...)
	modified[0].Checksum = "tampered"
	if err := postgres.Check(ctx, pool, modified); !errors.Is(err, postgres.ErrMigrationModified) {
		t.Fatalf("modified: err = %v", err)
	}
}
