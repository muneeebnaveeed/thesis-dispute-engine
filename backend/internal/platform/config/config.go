// Package config reads service configuration from the environment.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the resolved service configuration.
type Config struct {
	Addr               string
	DatabaseURL        string
	MigrateDatabaseURL string
	ShutdownTimeout    time.Duration
	IdempotencyTTL     time.Duration
	// CoreTimeout bounds one instruction to a tenant's banking core; past it the transition rolls back as unavailable.
	CoreTimeout time.Duration
	// SMTPAddr is the mail relay for customer notices (host:port, no authentication); empty logs instead of sending.
	SMTPAddr string
	// MailFrom is the sender on every notice email.
	MailFrom string
	// CORSOrigins are browser origins allowed to call the API directly with an analyst token.
	CORSOrigins []string
	// ServiceKey guards /internal/*, used only by the frontend server; empty disables those routes' authentication.
	ServiceKey string
	// InternalCIDRs are the peers allowed to see /internal/* at all; everyone else gets 404.
	InternalCIDRs []string
	// RatePerMinute is each tenant's request budget; 0 disables the limiter.
	RatePerMinute int
	// AuthFailuresPerMinute is how many bad credentials one peer may present per minute before being refused.
	AuthFailuresPerMinute int
	// DecisionsKey enables the typed-decision model that proposes values an analyst confirms (ADR 0024);
	// empty leaves every such field blank, which is the supported case.
	DecisionsKey string
	// DecisionsEndpoint and DecisionsModel override the hosted defaults; DecisionsTimeout bounds one call,
	// past which the analyst simply gets an empty field.
	DecisionsEndpoint string
	DecisionsModel    string
	DecisionsTimeout  time.Duration
	// Production turns dev defaults into startup errors.
	Production bool
	LogLevel   slog.Level
}

// Load resolves Config from DISPUTE_* variables; defaults match deploy/compose.yml.
func Load() (Config, error) {
	cfg := Config{
		Addr: getenv("DISPUTE_ADDR", ":8090"),
		// The API runs as the least-privileged dispute_api login; only migrate and seed use the owner.
		DatabaseURL:           getenv("DISPUTE_DATABASE_URL", "postgres://dispute_api:dispute_api@localhost:5432/dispute?sslmode=disable"),
		MigrateDatabaseURL:    getenv("DISPUTE_MIGRATE_DATABASE_URL", "postgres://dispute:dispute@localhost:5432/dispute?sslmode=disable"),
		ShutdownTimeout:       10 * time.Second,
		IdempotencyTTL:        24 * time.Hour,
		CoreTimeout:           5 * time.Second,
		SMTPAddr:              os.Getenv("DISPUTE_SMTP_ADDR"),
		MailFrom:              getenv("DISPUTE_MAIL_FROM", "disputes@localhost"),
		CORSOrigins:           strings.Split(getenv("DISPUTE_CORS_ORIGINS", "http://localhost:3002"), ","),
		ServiceKey:            getenv("DISPUTE_SERVICE_KEY", "dev-service-key"),
		DecisionsKey:          os.Getenv("DISPUTE_DECISIONS_KEY"),
		DecisionsEndpoint:     os.Getenv("DISPUTE_DECISIONS_ENDPOINT"),
		DecisionsModel:        os.Getenv("DISPUTE_DECISIONS_MODEL"),
		DecisionsTimeout:      3 * time.Second,
		InternalCIDRs:         strings.Split(getenv("DISPUTE_INTERNAL_CIDRS", "127.0.0.0/8,::1/128"), ","),
		RatePerMinute:         600,
		AuthFailuresPerMinute: 30,
		Production:            os.Getenv("DISPUTE_ENV") == "production",
	}

	if raw := os.Getenv("DISPUTE_AUTH_FAILURES_PER_MINUTE"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			return Config{}, fmt.Errorf("config: DISPUTE_AUTH_FAILURES_PER_MINUTE must be a non-negative integer, got %q", raw)
		}
		cfg.AuthFailuresPerMinute = n
	}
	if raw := os.Getenv("DISPUTE_RATE_PER_MINUTE"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			return Config{}, fmt.Errorf("config: DISPUTE_RATE_PER_MINUTE must be a non-negative integer, got %q", raw)
		}
		cfg.RatePerMinute = n
	}

	// Dev defaults are fine on a laptop and a breach in production; refuse to start rather than hope.
	if cfg.Production {
		if cfg.ServiceKey == "dev-service-key" || len(cfg.ServiceKey) < 32 {
			return Config{}, errors.New("config: DISPUTE_ENV=production needs DISPUTE_SERVICE_KEY of at least 32 characters, not the dev default")
		}
		if strings.Contains(cfg.DatabaseURL, "dispute_api:dispute_api@") || strings.Contains(cfg.MigrateDatabaseURL, "dispute:dispute@") {
			return Config{}, errors.New("config: DISPUTE_ENV=production with the dev database credentials")
		}
		if strings.Contains(cfg.DatabaseURL, "sslmode=disable") {
			return Config{}, errors.New("config: DISPUTE_ENV=production must not disable TLS to the database")
		}
	}

	if raw := os.Getenv("DISPUTE_IDEMPOTENCY_TTL"); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil || d <= 0 {
			return Config{}, fmt.Errorf("config: DISPUTE_IDEMPOTENCY_TTL must be a positive duration such as 24h, got %q", raw)
		}
		cfg.IdempotencyTTL = d
	}

	if raw := os.Getenv("DISPUTE_CORE_TIMEOUT"); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil || d <= 0 {
			return Config{}, fmt.Errorf("config: DISPUTE_CORE_TIMEOUT must be a positive duration such as 5s, got %q", raw)
		}
		cfg.CoreTimeout = d
	}

	if raw := os.Getenv("DISPUTE_LOG_LEVEL"); raw != "" {
		if err := cfg.LogLevel.UnmarshalText([]byte(raw)); err != nil {
			return Config{}, fmt.Errorf("config: DISPUTE_LOG_LEVEL must be debug, info, warn or error, got %q", raw)
		}
	}

	if raw := os.Getenv("DISPUTE_SHUTDOWN_TIMEOUT_SECONDS"); raw != "" {
		seconds, err := strconv.Atoi(raw)
		if err != nil || seconds <= 0 {
			return Config{}, fmt.Errorf("config: DISPUTE_SHUTDOWN_TIMEOUT_SECONDS must be a positive integer, got %q", raw)
		}
		cfg.ShutdownTimeout = time.Duration(seconds) * time.Second
	}

	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
