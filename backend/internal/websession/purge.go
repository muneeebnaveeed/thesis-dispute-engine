package websession

import (
	"context"
	"log/slog"
	"time"
)

// RunPurge deletes expired sessions every few minutes until ctx ends; expiry is already enforced on read, so this
// only keeps the table small.
func RunPurge(ctx context.Context, store *Store, log *slog.Logger) {
	t := time.NewTicker(5 * time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		n, err := store.PurgeExpired(ctx)
		switch {
		case err != nil && ctx.Err() == nil:
			log.Warn("web session purge", "err", err)
		case n > 0:
			// a sweep that removed nothing is the normal case and not worth a line at INFO
			level := slog.LevelDebug
			if n > 0 {
				level = slog.LevelInfo
			}
			log.Log(ctx, level, "web session purge", "deleted", n)
		}
	}
}
