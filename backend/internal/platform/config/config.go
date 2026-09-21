// Package config reads service configuration from the environment.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config is the resolved service configuration.
type Config struct {
	Addr            string
	DatabaseURL     string
	ShutdownTimeout time.Duration
}

// Load resolves Config from DISPUTE_* variables; defaults match deploy/compose.yml.
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
