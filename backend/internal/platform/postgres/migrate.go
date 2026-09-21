// Package postgres owns the connection pool and forward-only SQL migrations.
package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Arbitrary but fixed: every instance of this service takes the same advisory lock while migrating.
const migrationLockID = 7204_0001

var migrationName = regexp.MustCompile(`^(\d{4})_[a-z0-9_]+\.sql$`)

// ErrMigrationModified means an already-applied file no longer matches its recorded checksum.
var ErrMigrationModified = errors.New("postgres: applied migration was modified")

// Migration is one SQL file from the migrations directory.
type Migration struct {
	Version  int
	Name     string
	SQL      string
	Checksum string
}

// Load reads and orders migrations from an fs.FS (the embedded directory or a test fixture).
func Load(dir fs.FS) ([]Migration, error) {
	entries, err := fs.ReadDir(dir, ".")
	if err != nil {
		return nil, err
	}
	var out []Migration
	for _, e := range entries {
		m := migrationName.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		version, _ := strconv.Atoi(m[1])
		body, err := fs.ReadFile(dir, e.Name())
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(body)
		out = append(out, Migration{Version: version, Name: e.Name(), SQL: string(body), Checksum: hex.EncodeToString(sum[:])})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	for i := 1; i < len(out); i++ {
		if out[i].Version == out[i-1].Version {
			return nil, fmt.Errorf("postgres: duplicate migration version %04d", out[i].Version)
		}
	}
	return out, nil
}

// Migrate applies pending migrations in order, each in its own transaction, and returns how many ran.
func Migrate(ctx context.Context, pool *pgxpool.Pool, migrations []Migration) (int, error) {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return 0, err
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, migrationLockID); err != nil {
		return 0, err
	}
	defer conn.Exec(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock($1)`, migrationLockID) //nolint:errcheck // best effort; the session end releases it anyway

	if _, err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version int PRIMARY KEY, name text NOT NULL, checksum text NOT NULL, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return 0, err
	}

	applied := map[int]string{}
	rows, err := conn.Query(ctx, `SELECT version, checksum FROM schema_migrations`)
	if err != nil {
		return 0, err
	}
	for rows.Next() {
		var v int
		var sum string
		if err := rows.Scan(&v, &sum); err != nil {
			rows.Close()
			return 0, err
		}
		applied[v] = sum
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	ran := 0
	for _, m := range migrations {
		if sum, ok := applied[m.Version]; ok {
			if sum != m.Checksum {
				return ran, fmt.Errorf("%w: %s", ErrMigrationModified, m.Name)
			}
			continue
		}
		if err := pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, m.SQL); err != nil {
				return fmt.Errorf("postgres: migration %s: %w", m.Name, err)
			}
			_, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version, name, checksum) VALUES ($1, $2, $3)`, m.Version, m.Name, m.Checksum)
			return err
		}); err != nil {
			return ran, err
		}
		ran++
	}
	return ran, nil
}
