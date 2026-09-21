package config

import (
	"log/slog"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Addr != ":8090" {
		t.Errorf("Addr = %q, want %q", cfg.Addr, ":8090")
	}
	if cfg.ShutdownTimeout != 10*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 10s", cfg.ShutdownTimeout)
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("DISPUTE_ADDR", "127.0.0.1:9090")
	t.Setenv("DISPUTE_SHUTDOWN_TIMEOUT_SECONDS", "3")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Addr != "127.0.0.1:9090" {
		t.Errorf("Addr = %q, want override", cfg.Addr)
	}
	if cfg.ShutdownTimeout != 3*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 3s", cfg.ShutdownTimeout)
	}
}

func TestLoadLogLevel(t *testing.T) {
	t.Setenv("DISPUTE_LOG_LEVEL", "debug")
	cfg, err := Load()
	if err != nil || cfg.LogLevel != slog.LevelDebug {
		t.Errorf("LogLevel = %v, err = %v", cfg.LogLevel, err)
	}
	t.Setenv("DISPUTE_LOG_LEVEL", "loud")
	if _, err := Load(); err == nil {
		t.Error("bad level accepted")
	}
}

func TestLoadRejectsBadTimeout(t *testing.T) {
	for _, raw := range []string{"abc", "0", "-5"} {
		t.Setenv("DISPUTE_SHUTDOWN_TIMEOUT_SECONDS", raw)
		if _, err := Load(); err == nil {
			t.Errorf("Load() with timeout %q: want error, got nil", raw)
		}
	}
}
