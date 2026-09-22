package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/exaring/otelpgx"
	pgxdecimal "github.com/jackc/pgx-shopspring-decimal"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect opens a pool and verifies it with one round trip.
func Connect(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("postgres: parse url: %w", err)
	}
	return ConnectWithConfig(ctx, cfg)
}

// ConnectWithConfig is Connect for a caller-built config (tests set search_path on it).
func ConnectWithConfig(ctx context.Context, cfg *pgxpool.Config) (*pgxpool.Pool, error) {
	// Every statement becomes a child span of the request; values are never recorded. Pool acquisition spans are
	// off (a quarter of all spans, microseconds each) and the SQL text too: the span name is the sqlc query name,
	// which is the text's address.
	cfg.ConnConfig.Tracer = otelpgx.NewTracer(otelpgx.WithSpanNameFunc(spanName), otelpgx.WithDisableAcquireTracer(),
		otelpgx.WithDisableSQLStatementInAttributes())
	cfg.AfterConnect = func(_ context.Context, conn *pgx.Conn) error {
		pgxdecimal.Register(conn.TypeMap())
		return nil
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("postgres: connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres: ping: %w", err)
	}
	return pool, nil
}

// spanName prefers sqlc's "-- name: X :one" header over the raw statement, which otelpgx would truncate to "--".
func spanName(stmt string) string {
	first, _, _ := strings.Cut(strings.TrimSpace(stmt), "\n")
	if rest, ok := strings.CutPrefix(first, "-- name: "); ok {
		name, _, _ := strings.Cut(rest, " ")
		return name
	}
	if len(first) > 64 {
		return first[:64]
	}
	return first
}
