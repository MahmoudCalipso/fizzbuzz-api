package config_test

import (
	"testing"
	"time"

	"fizzbuzz-api/internal/config"
)

func TestLoad_Defaults(t *testing.T) {
	for _, k := range []string{"PORT", "MAX_LIMIT", "MAX_STATS_ENTRIES", "SHUTDOWN_TIMEOUT"} {
		t.Setenv(k, "")
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != "8080" || cfg.MaxLimit != 10000 || cfg.MaxStatsEntries != 100000 || cfg.ShutdownTimeout != 10*time.Second {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestLoad_Overrides(t *testing.T) {
	t.Setenv("PORT", "9000")
	t.Setenv("MAX_LIMIT", "500")
	t.Setenv("MAX_STATS_ENTRIES", "0")
	t.Setenv("SHUTDOWN_TIMEOUT", "3s")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != "9000" || cfg.MaxLimit != 500 || cfg.MaxStatsEntries != 0 || cfg.ShutdownTimeout != 3*time.Second {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestLoad_Invalid(t *testing.T) {
	tests := []struct{ name, key, value string }{
		{"non numeric limit", "MAX_LIMIT", "abc"},
		{"zero limit", "MAX_LIMIT", "0"},
		{"negative entries", "MAX_STATS_ENTRIES", "-1"},
		{"bad duration", "SHUTDOWN_TIMEOUT", "soon"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, k := range []string{"PORT", "MAX_LIMIT", "MAX_STATS_ENTRIES", "SHUTDOWN_TIMEOUT"} {
				t.Setenv(k, "")
			}
			t.Setenv(tc.key, tc.value)

			if _, err := config.Load(); err == nil {
				t.Fatal("expected an error, got nil")
			}
		})
	}
}
