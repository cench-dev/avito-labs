package config

import (
	"testing"
	"time"
)

func setRequiredEnv(t *testing.T) {
	t.Helper()

	values := map[string]string{
		"HTTP_ADDR":                  ":8080",
		"LOG_LEVEL":                  "debug",
		"SHUTDOWN_TIMEOUT":           "10s",
		"DATABASE_URL":               "postgres://localhost:5432/test",
		"DATABASE_MAX_CONNS":         "10",
		"DATABASE_MIN_CONNS":         "2",
		"DATABASE_MAX_CONN_LIFETIME": "1h",
		"DATABASE_CONNECT_TIMEOUT":   "5s",
		"DATABASE_QUERY_TIMEOUT":     "3s",
		"POSITION_CHECK_INTERVAL":    "",
		"POSITION_STALE_AFTER":       "",
		"POSITION_WORKERS":           "",
		"POSITION_QUEUE_SIZE":        "",
		"PUSH_HTTP_URL":              "",
		"PUSH_ADMIN_URL":             "",
		"PUSH_REQUEST_TIMEOUT":       "",
	}

	for key, value := range values {
		t.Setenv(key, value)
	}
}

func TestLoadPositionDefaults(t *testing.T) {
	setRequiredEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.PositionCheckInterval != 30*time.Second {
		t.Errorf("PositionCheckInterval = %v, want 30s", cfg.PositionCheckInterval)
	}
	if cfg.PositionStaleAfter != 60*time.Second {
		t.Errorf("PositionStaleAfter = %v, want 60s", cfg.PositionStaleAfter)
	}
	if cfg.PositionWorkers != 4 {
		t.Errorf("PositionWorkers = %d, want 4", cfg.PositionWorkers)
	}
	if cfg.PositionQueueSize != 100 {
		t.Errorf("PositionQueueSize = %d, want 100", cfg.PositionQueueSize)
	}
	if cfg.PushHTTPURL != "http://localhost:8090" {
		t.Errorf("PushHTTPURL = %q, want default URL", cfg.PushHTTPURL)
	}
	if cfg.PushAdminURL != "http://localhost:8091" {
		t.Errorf("PushAdminURL = %q, want default URL", cfg.PushAdminURL)
	}
	if cfg.PushRequestTimeout != time.Second {
		t.Errorf("PushRequestTimeout = %v, want 1s", cfg.PushRequestTimeout)
	}
}

func TestLoadPositionOverrides(t *testing.T) {
	setRequiredEnv(t)

	t.Setenv("POSITION_CHECK_INTERVAL", "15s")
	t.Setenv("POSITION_STALE_AFTER", "45s")
	t.Setenv("POSITION_WORKERS", "8")
	t.Setenv("POSITION_QUEUE_SIZE", "200")
	t.Setenv("PUSH_HTTP_URL", "http://push:8090")
	t.Setenv("PUSH_ADMIN_URL", "http://push:8091")
	t.Setenv("PUSH_REQUEST_TIMEOUT", "2s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.PositionCheckInterval != 15*time.Second ||
		cfg.PositionStaleAfter != 45*time.Second ||
		cfg.PositionWorkers != 8 ||
		cfg.PositionQueueSize != 200 ||
		cfg.PushHTTPURL != "http://push:8090" ||
		cfg.PushAdminURL != "http://push:8091" ||
		cfg.PushRequestTimeout != 2*time.Second {
		t.Errorf("unexpected config values: %+v", cfg)
	}
}
