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
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/config"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/httpserver"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/postgres"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/telemetry"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/migrations"
)

func main() {
	// Records go to stdout and, once Setup installs a provider, to OTLP with trace context attached.
	logger := slog.New(telemetry.Handler(slog.NewJSONHandler(os.Stdout, nil)))
	slog.SetDefault(logger)

	if err := run(logger); err != nil {
		logger.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

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
	ran, err := postgres.Migrate(ctx, pool, files)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	logger.Info("migrations", "applied", ran, "total", len(files))

	svc, err := application.NewService(disputepg.NewStore(pool), nil)
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	if err := disputehttp.Mount(mux, svc, pool.Ping); err != nil {
		return err
	}
	srv := httpserver.New(cfg.Addr, logger, mux)

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
