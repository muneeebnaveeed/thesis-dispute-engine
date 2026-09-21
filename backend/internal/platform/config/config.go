// Package config reads service configuration from the environment.
package config

import (
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
	// CORSOrigins are browser origins allowed to call the API directly with an analyst token.
	CORSOrigins []string
	// ServiceKey guards /internal/*, used only by the frontend server; empty disables those routes' authentication.
	ServiceKey string
	LogLevel   slog.Level
}

// Load resolves Config from DISPUTE_* variables; defaults match deploy/compose.yml.
func Load() (Config, error) {
	cfg := Config{
		Addr: getenv("DISPUTE_ADDR", ":8090"),
		// The API runs as the least-privileged dispute_api login; only migrate and seed use the owner.
		DatabaseURL:        getenv("DISPUTE_DATABASE_URL", "postgres://dispute_api:dispute_api@localhost:5432/dispute?sslmode=disable"),
		MigrateDatabaseURL: getenv("DISPUTE_MIGRATE_DATABASE_URL", "postgres://dispute:dispute@localhost:5432/dispute?sslmode=disable"),
		ShutdownTimeout:    10 * time.Second,
		IdempotencyTTL:     24 * time.Hour,
		CORSOrigins:        strings.Split(getenv("DISPUTE_CORS_ORIGINS", "http://localhost:3002"), ","),
		ServiceKey:         getenv("DISPUTE_SERVICE_KEY", "dev-service-key"),
	}

	if raw := os.Getenv("DISPUTE_IDEMPOTENCY_TTL"); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil || d <= 0 {
			return Config{}, fmt.Errorf("config: DISPUTE_IDEMPOTENCY_TTL must be a positive duration such as 24h, got %q", raw)
		}
		cfg.IdempotencyTTL = d
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
