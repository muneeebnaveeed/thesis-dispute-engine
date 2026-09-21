// Command api runs the dispute engine HTTP API.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	disputepg "github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/infrastructure/postgres"
	disputehttp "github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/ports/http"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/auth"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/config"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/httpserver"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/postgres"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/telemetry"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/websession"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/migrations"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
	// Records are redacted, then go to stdout and, once Setup installs a provider, to OTLP with trace context.
	logger := slog.New(telemetry.Redact(telemetry.Handler(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))))
	slog.SetDefault(logger)

	if err := run(cfg, logger); err != nil {
		logger.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(cfg config.Config, logger *slog.Logger) error {

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	shutdownTelemetry, err := telemetry.Setup(ctx, logger)
	if err != nil {
		return err
	}
	defer func() {
		flushCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := shutdownTelemetry(flushCtx); err != nil {
			logger.Error("telemetry shutdown", "err", err)
		}
	}()

	pool, err := postgres.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	files, err := postgres.Load(migrations.FS)
	if err != nil {
		return err
	}
	// Fail fast rather than serve against a stale schema; cmd/migrate owns the schema, this process cannot.
	if err := postgres.Check(ctx, pool, files); err != nil {
		return fmt.Errorf("schema: %w (run cmd/migrate)", err)
	}

	store := disputepg.NewStore(pool)
	svc, err := application.NewService(store, nil)
	if err != nil {
		return err
	}
	go application.RunIdempotencyPurge(ctx, store, cfg.IdempotencyTTL, logger)

	mux := http.NewServeMux()
	sessions := websession.NewStore(pool)
	go websession.RunPurge(ctx, sessions, logger)
	if err := disputehttp.Mount(mux, svc, pool.Ping, disputehttp.WithSessions(sessions)); err != nil {
		return err
	}
	keys := disputepg.NewKeyStore(pool)
	srv := httpserver.New(cfg.Addr, logger, mux, httpserver.CORS(cfg.CORSOrigins), auth.Bearer(keys, auth.NewOIDC(keys)), auth.ServiceKey(cfg.ServiceKey))

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		logger.Info("shutting down", "timeout", cfg.ShutdownTimeout)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}
