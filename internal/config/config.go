// Package config reads and validates the server's runtime configuration from
// environment variables (twelve-factor, PRD story 55). Everything the server
// needs to boot — the backing-service URLs, the JWT signing secret, the OTel
// endpoint, and the listen address — is resolved here once at startup so the
// rest of the code takes a validated *Config rather than reaching into os.Getenv.
package config

import (
	"fmt"
	"os"
	"strings"
)

// Config is the validated server configuration. All fields are populated from
// environment variables by Load.
type Config struct {
	// DatabaseURL is the Postgres connection string (pgx pool + migrations).
	DatabaseURL string
	// RedisURL is the Redis connection string for the cache backend.
	RedisURL string
	// NATSURL is the NATS connection string for the event bus.
	NATSURL string
	// JWTSecret signs and verifies interactive-login JWTs.
	JWTSecret string
	// OTelEndpoint is the OpenTelemetry collector OTLP endpoint. Optional: an
	// empty value disables metric export.
	OTelEndpoint string
	// ListenAddr is the HTTP listen address (e.g. ":8080").
	ListenAddr string
	// MigrationsDir holds the golang-migrate *.up.sql files applied at startup.
	MigrationsDir string
}

// Environment variable names. Kept as constants so the server, the seeder, and
// docker-compose stay in agreement about the twelve-factor surface.
const (
	envDatabaseURL   = "DATABASE_URL"
	envRedisURL      = "REDIS_URL"
	envNATSURL       = "NATS_URL"
	envJWTSecret     = "JWT_SECRET"
	envOTelEndpoint  = "OTEL_EXPORTER_OTLP_ENDPOINT"
	envListenAddr    = "LISTEN_ADDR"
	envMigrationsDir = "MIGRATIONS_DIR"
)

// Defaults for the optional variables.
const (
	defaultListenAddr    = ":8080"
	defaultMigrationsDir = "./migrations"
)

// Load reads the configuration from the environment and validates it. The
// backing-service URLs and the JWT secret are required (the server cannot boot
// fail-open without them); the OTel endpoint is optional and the listen address
// and migrations directory fall back to sensible defaults. It returns an error
// naming every missing required variable at once, so an operator fixes them in
// one pass rather than one boot per mistake.
func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL:   os.Getenv(envDatabaseURL),
		RedisURL:      os.Getenv(envRedisURL),
		NATSURL:       os.Getenv(envNATSURL),
		JWTSecret:     os.Getenv(envJWTSecret),
		OTelEndpoint:  os.Getenv(envOTelEndpoint),
		ListenAddr:    os.Getenv(envListenAddr),
		MigrationsDir: os.Getenv(envMigrationsDir),
	}

	if cfg.ListenAddr == "" {
		cfg.ListenAddr = defaultListenAddr
	}
	if cfg.MigrationsDir == "" {
		cfg.MigrationsDir = defaultMigrationsDir
	}

	var missing []string
	required := []struct {
		name  string
		value string
	}{
		{envDatabaseURL, cfg.DatabaseURL},
		{envRedisURL, cfg.RedisURL},
		{envNATSURL, cfg.NATSURL},
		{envJWTSecret, cfg.JWTSecret},
	}
	for _, r := range required {
		if strings.TrimSpace(r.value) == "" {
			missing = append(missing, r.name)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("config: missing required environment variables: %s", strings.Join(missing, ", "))
	}

	return cfg, nil
}
