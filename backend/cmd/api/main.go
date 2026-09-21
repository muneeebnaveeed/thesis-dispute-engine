// Command api runs the dispute engine HTTP API.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/config"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/httpserver"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/telemetry"
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

	srv := httpserver.New(cfg.Addr, logger)

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
