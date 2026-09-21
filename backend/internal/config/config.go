// Package config reads the service configuration from the environment.
//
// Every value has a default that works for local development against the
// compose file in deploy/, so `make run` needs no environment at all.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config is the fully resolved service configuration.
type Config struct {
	// Addr is the listen address of the HTTP API, e.g. ":8090".
	Addr string
	// DatabaseURL is the PostgreSQL connection string.
	DatabaseURL string
	// ShutdownTimeout bounds graceful shutdown on SIGINT/SIGTERM.
	ShutdownTimeout time.Duration
}

// Load resolves the configuration from environment variables.
func Load() (Config, error) {
	cfg := Config{
		Addr:            getenv("DISPUTE_ADDR", ":8090"),
		DatabaseURL:     getenv("DISPUTE_DATABASE_URL", "postgres://dispute:dispute@localhost:5432/dispute?sslmode=disable"),
		ShutdownTimeout: 10 * time.Second,
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
