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
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/infrastructure/mail"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/infrastructure/mockcore"
	disputepg "github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/infrastructure/postgres"
	disputehttp "github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/ports/http"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/auth"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/config"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/httpserver"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/postgres"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/ratelimit"
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
	// Tenants choose their core by kind in tenants.settings.core; only the simulated one exists in this build.
	cores := application.CoreRouter{Adapters: map[string]application.BankingCore{mockcore.Kind: mockcore.New(logger)}}
	// Customer notices: composed in the transition, delivered by the dispatcher from the outbox.
	var mailer application.Mailer = mail.Log{Log: logger}
	if cfg.SMTPAddr != "" {
		mailer = mail.SMTP{Addr: cfg.SMTPAddr, From: cfg.MailFrom}
	}
	dispatcher := application.NewDispatcher(store, mailer, application.RenderMail, logger)
	svc, err := application.NewService(store, nil, application.WithCore(cores), application.WithCoreTimeout(cfg.CoreTimeout),
		application.WithAfterCommit(dispatcher.Kick))
	if err != nil {
		return err
	}
	go dispatcher.Run(ctx)
	go application.RunIdempotencyPurge(ctx, store, cfg.IdempotencyTTL, logger)

	mux := http.NewServeMux()
	keys := disputepg.NewKeyStore(pool)
	sessions := websession.NewStore(pool)
	go websession.RunPurge(ctx, sessions, logger)
	if err := disputehttp.Mount(mux, svc, pool.Ping, disputehttp.WithSessions(sessions), disputehttp.WithTenants(keys), disputehttp.WithKeys(keys)); err != nil {
		return err
	}
	limiter := ratelimit.NewPGCounter(pool)
	go ratelimit.RunPurge(ctx, limiter, logger)
	srv := httpserver.New(cfg.Addr, logger, mux,
		httpserver.InternalOnly("/internal/", cfg.InternalCIDRs),
		httpserver.CORS(cfg.CORSOrigins),
		auth.Bearer(keys, auth.NewOIDC(keys)),
		auth.NewFailureLimiter(cfg.AuthFailuresPerMinute).Middleware,
		auth.ServiceKey(cfg.ServiceKey),
		ratelimit.Middleware(limiter, cfg.RatePerMinute, logger),
	)

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
