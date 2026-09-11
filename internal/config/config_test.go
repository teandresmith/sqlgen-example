package config

import (
	"strings"
	"testing"
)

// setEnv wipes every config variable, then applies the given overrides, so
// each case starts from a known-empty environment regardless of the host's.
func setEnv(t *testing.T, vars map[string]string) {
	t.Helper()
	for _, name := range []string{
		envDatabaseURL, envRedisURL, envNATSURL, envJWTSecret,
		envOTelEndpoint, envListenAddr, envMigrationsDir,
	} {
		t.Setenv(name, "")
	}
	for k, v := range vars {
		t.Setenv(k, v)
	}
}

func fullEnv() map[string]string {
	return map[string]string{
		envDatabaseURL: "postgres://u:p@localhost:5432/taskr?sslmode=disable",
		envRedisURL:    "redis://localhost:6379",
		envNATSURL:     "nats://localhost:4222",
		envJWTSecret:   "test-secret",
	}
}

func TestLoadReadsAllValues(t *testing.T) {
	env := fullEnv()
	env[envOTelEndpoint] = "otel-collector:4317"
	env[envListenAddr] = ":9090"
	env[envMigrationsDir] = "/srv/migrations"
	setEnv(t, env)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DatabaseURL != env[envDatabaseURL] {
		t.Errorf("DatabaseURL = %q, want %q", cfg.DatabaseURL, env[envDatabaseURL])
	}
	if cfg.RedisURL != env[envRedisURL] {
		t.Errorf("RedisURL = %q, want %q", cfg.RedisURL, env[envRedisURL])
	}
	if cfg.NATSURL != env[envNATSURL] {
		t.Errorf("NATSURL = %q, want %q", cfg.NATSURL, env[envNATSURL])
	}
	if cfg.JWTSecret != env[envJWTSecret] {
		t.Errorf("JWTSecret = %q, want %q", cfg.JWTSecret, env[envJWTSecret])
	}
	if cfg.OTelEndpoint != env[envOTelEndpoint] {
		t.Errorf("OTelEndpoint = %q, want %q", cfg.OTelEndpoint, env[envOTelEndpoint])
	}
	if cfg.ListenAddr != ":9090" {
		t.Errorf("ListenAddr = %q, want %q", cfg.ListenAddr, ":9090")
	}
	if cfg.MigrationsDir != "/srv/migrations" {
		t.Errorf("MigrationsDir = %q, want %q", cfg.MigrationsDir, "/srv/migrations")
	}
}

func TestLoadAppliesDefaults(t *testing.T) {
	setEnv(t, fullEnv())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ListenAddr != defaultListenAddr {
		t.Errorf("ListenAddr = %q, want default %q", cfg.ListenAddr, defaultListenAddr)
	}
	if cfg.MigrationsDir != defaultMigrationsDir {
		t.Errorf("MigrationsDir = %q, want default %q", cfg.MigrationsDir, defaultMigrationsDir)
	}
	if cfg.OTelEndpoint != "" {
		t.Errorf("OTelEndpoint = %q, want empty (export disabled)", cfg.OTelEndpoint)
	}
}

func TestLoadRejectsMissingRequired(t *testing.T) {
	env := fullEnv()
	delete(env, envRedisURL)
	delete(env, envJWTSecret)
	setEnv(t, env)

	_, err := Load()
	if err == nil {
		t.Fatal("Load succeeded, want error for missing required vars")
	}
	// Fail-closed: the error must name every missing variable in one pass.
	if !strings.Contains(err.Error(), envRedisURL) {
		t.Errorf("error %q does not mention %s", err, envRedisURL)
	}
	if !strings.Contains(err.Error(), envJWTSecret) {
		t.Errorf("error %q does not mention %s", err, envJWTSecret)
	}
}
