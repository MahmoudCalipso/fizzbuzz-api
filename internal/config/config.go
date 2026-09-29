// Package config loads the runtime configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds every tunable of the server.
type Config struct {
	Port            string        // PORT              (default "8080")
	MaxLimit        int           // MAX_LIMIT         (default 10000)
	MaxStatsEntries int           // MAX_STATS_ENTRIES (default 100000, 0 = unbounded)
	ShutdownTimeout time.Duration // SHUTDOWN_TIMEOUT  (default 10s)
}

// Load reads the configuration, applying defaults and validating values.
func Load() (Config, error) {
	cfg := Config{
		Port:            getenv("PORT", "8080"),
		MaxLimit:        10000,
		MaxStatsEntries: 100000,
		ShutdownTimeout: 10 * time.Second,
	}

	var err error
	if cfg.MaxLimit, err = getenvInt("MAX_LIMIT", cfg.MaxLimit); err != nil {
		return Config{}, err
	}
	if cfg.MaxStatsEntries, err = getenvInt("MAX_STATS_ENTRIES", cfg.MaxStatsEntries); err != nil {
		return Config{}, err
	}
	if v := os.Getenv("SHUTDOWN_TIMEOUT"); v != "" {
		if cfg.ShutdownTimeout, err = time.ParseDuration(v); err != nil {
			return Config{}, fmt.Errorf("SHUTDOWN_TIMEOUT: %w", err)
		}
	}

	if cfg.MaxLimit <= 0 {
		return Config{}, fmt.Errorf("MAX_LIMIT must be > 0, got %d", cfg.MaxLimit)
	}
	if cfg.MaxStatsEntries < 0 {
		return Config{}, fmt.Errorf("MAX_STATS_ENTRIES must be >= 0, got %d", cfg.MaxStatsEntries)
	}
	return cfg, nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvInt(key string, def int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return n, nil
}
