package application

import (
	"context"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

// RunIdempotencyPurge deletes stored idempotent responses older than ttl, once soon after start and then every
// ttl/24 (at least a minute); it returns when ctx ends. The store serialises replicas, so every instance may run it.
func RunIdempotencyPurge(ctx context.Context, store Store, ttl time.Duration, log *slog.Logger) {
	counter, err := otel.Meter(scopeName).Int64Counter("dispute.idempotency_purged", metric.WithDescription("Stored idempotent responses deleted by the sweep"))
	if err != nil {
		log.Error("idempotency purge: meter", "err", err)
		return
	}
	every := max(ttl/24, time.Minute)
	timer := time.NewTimer(time.Minute)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		n, err := store.PurgeIdempotencyKeys(ctx, time.Now().Add(-ttl))
		switch {
		case err != nil && ctx.Err() == nil:
			log.Warn("idempotency purge", "err", err)
		case n > 0:
			counter.Add(ctx, n)
			log.Info("idempotency purge", "deleted", n, "older_than", ttl)
		}
		timer.Reset(every)
	}
}
