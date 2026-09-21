package migrations_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/postgres/pgtest"
)

// The API role reads everything (within its tenant), appends, and nothing else; cross-tenant work goes through the
// owner-defined view and function. A new table without grants fails here.
func TestAppRolePrivileges(t *testing.T) {
	owner, schema := pgtest.PoolWithSchema(t)
	app := pgtest.AppPool(t, schema)
	ctx := context.Background()

	rows, err := owner.Query(ctx, `SELECT tablename FROM pg_tables WHERE schemaname = $1`, schema)
	if err != nil {
		t.Fatal(err)
	}
	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		tables = append(tables, name)
	}
	rows.Close()
	if len(tables) < 5 {
		t.Fatalf("expected the schema tables, got %v", tables)
	}
	for _, table := range tables {
		var ok bool
		if err := owner.QueryRow(ctx, `SELECT has_table_privilege('dispute_app', $1, 'SELECT')`, schema+"."+table).Scan(&ok); err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Errorf("dispute_app cannot SELECT %s; add grants to the migration that created it", table)
		}
	}

	denied := map[string]string{
		"delete events":  `DELETE FROM dispute_events`,
		"update events":  `UPDATE dispute_events SET actor = 'x'`,
		"delete dispute": `DELETE FROM disputes`,
		"delete keys":    `DELETE FROM idempotency_keys`,
		"truncate":       `TRUNCATE disputes`,
		"ddl":            `CREATE TABLE smuggled (id int)`,
		"migrations":     `INSERT INTO schema_migrations (version, name, checksum) VALUES (999, 'x', 'y')`,
		"key label":      `UPDATE tenant_keys SET label = 'x'`,
		"key unrevoke":   `UPDATE tenant_keys SET expires_at = NULL, key_hash = '\x00'`,
	}
	for name, stmt := range denied {
		_, err := app.Exec(ctx, stmt)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
			t.Errorf("%s: want insufficient_privilege (42501), got %v", name, err)
		}
	}

	allowed := []string{
		`SELECT count(*) FROM disputes`,
		`SELECT count(*) FROM disputes_by_state`,
		`SELECT purge_idempotency_keys(now() - interval '1 year')`,
		`UPDATE tenant_keys SET last_used_at = now()`,
		// Self-service (migration 0009): the API issues keys for the tenant in context and can end them, nothing else.
		`INSERT INTO tenant_keys (id, tenant_id, key_hash, prefix, label) VALUES (gen_random_uuid(), '00000000-0000-8000-8000-00000000a001', '\x00', 'p', 'l')`,
		`UPDATE tenant_keys SET revoked_at = now() WHERE label = 'l'`,
	}
	for _, stmt := range allowed {
		if _, err := app.Exec(ctx, stmt); err != nil {
			t.Errorf("%s: %v", stmt, err)
		}
	}
}
