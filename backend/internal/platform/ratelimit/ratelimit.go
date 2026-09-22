// Package ratelimit is the per-tenant request budget from ADR 0007: a fixed one-minute window counted in PostgreSQL,
// so every replica shares it and the browser and tenant systems of one customer cannot starve another's.
package ratelimit

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/infrastructure/postgres/sqlcgen"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/errs"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/httpserver"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/tenant"
)

// ErrRateLimited is the problem clients get; retryAfterSeconds tells them when the window turns over.
var ErrRateLimited = errs.New(errs.RateLimited, "rate-limited", "this tenant has used its request budget for the current minute")

// Counter records one request in a tenant's current window and returns the running count.
type Counter interface {
	Bump(ctx context.Context, tenantID uuid.UUID, window time.Time) (int32, error)
}

// PGCounter counts in tenant_rate_windows.
type PGCounter struct{ pool *pgxpool.Pool }

// NewPGCounter wraps a pool.
func NewPGCounter(pool *pgxpool.Pool) *PGCounter { return &PGCounter{pool: pool} }

// Bump implements Counter.
func (c *PGCounter) Bump(ctx context.Context, tenantID uuid.UUID, window time.Time) (int32, error) {
	return sqlcgen.New(c.pool).BumpTenantRateWindow(ctx, sqlcgen.BumpTenantRateWindowParams{TenantID: tenantID, WindowStart: window})
}

// Purge drops windows older than an hour; the table only ever needs the current minute.
func (c *PGCounter) Purge(ctx context.Context) (int64, error) {
	return sqlcgen.New(c.pool).PurgeTenantRateWindows(ctx, time.Now().Add(-time.Hour))
}

// Middleware enforces perMinute requests per tenant on requests that resolved a tenant; anonymous and service
// requests pass through (they are refused or trusted elsewhere). A counter failure lets the request through: the
// limiter protects capacity, it is not an authorisation control.
func Middleware(counter Counter, perMinute int, log *slog.Logger) httpserver.Middleware {
	return func(next http.Handler) http.Handler {
		if perMinute <= 0 {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, ok := tenant.IDFrom(r.Context())
			if !ok {
				next.ServeHTTP(w, r)
				return
			}
			now := time.Now()
			window := now.Truncate(time.Minute)
			bumpCtx, span := otel.Tracer("ratelimit").Start(r.Context(), "ratelimit.bump")
			n, err := counter.Bump(bumpCtx, id, window)
			span.SetAttributes(attribute.Int("ratelimit.count", int(n)), attribute.Int("ratelimit.limit", perMinute))
			span.End()
			if err != nil {
				log.WarnContext(r.Context(), "rate limit counter", "err", err)
				next.ServeHTTP(w, r)
				return
			}
			remaining := max(perMinute-int(n), 0)
			w.Header().Set("RateLimit-Limit", strconv.Itoa(perMinute))
			w.Header().Set("RateLimit-Remaining", strconv.Itoa(remaining))
			if int(n) > perMinute {
				retry := int(window.Add(time.Minute).Sub(now).Seconds()) + 1
				w.Header().Set("Retry-After", strconv.Itoa(retry))
				httpserver.Annotate(r.Context(), "rate_limited", true)
				httpserver.WriteProblem(w, r, httpserver.ProblemFrom(r.Context(), r.URL.Path, ErrRateLimited).WithRetryAfter(retry), nil)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RunPurge trims old windows every ten minutes until ctx ends.
func RunPurge(ctx context.Context, c *PGCounter, log *slog.Logger) {
	t := time.NewTicker(10 * time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		if n, err := c.Purge(ctx); err != nil && ctx.Err() == nil {
			log.Warn("rate window purge", "err", err)
		} else if n > 0 {
			// a sweep that removed nothing is the normal case and not worth a line at INFO
			level := slog.LevelDebug
			if n > 0 {
				level = slog.LevelInfo
			}
			log.Log(ctx, level, "rate window purge", "deleted", n)
		}
	}
}
