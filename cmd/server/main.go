// Command server is the task-tracker GraphQL API entrypoint. It reads its
// configuration from the environment, applies the ./migrations DDL, builds one
// *database.Client on a pgx pool, and serves the generated GraphQL handler.
// This is the walking skeleton (issue 01): the whole infra → config →
// migrations → client → handler → serve vertical, wired but featureless.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/teandresmith/sqlgen-example/internal/appdb"
	"github.com/teandresmith/sqlgen-example/internal/auth"
	"github.com/teandresmith/sqlgen-example/internal/config"
	"github.com/teandresmith/sqlgen-example/internal/events"
	"github.com/teandresmith/sqlgen-example/internal/migrate"
	"github.com/teandresmith/sqlgen-example/internal/observability"
	"github.com/teandresmith/sqlgen-example/internal/server"
)

// shutdownTimeout bounds how long graceful shutdown waits for in-flight
// requests to complete before forcing the listener closed.
const shutdownTimeout = 15 * time.Second

// metricsFlushTimeout bounds the best-effort final metric flush during
// shutdown. It is shorter than shutdownTimeout so an unreachable collector
// cannot hold the process open after requests have drained.
const metricsFlushTimeout = 5 * time.Second

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	if err := run(logger); err != nil {
		logger.Error("server exited with error", "err", err)
		os.Exit(1)
	}
}

// run performs the full boot sequence and blocks serving until a shutdown
// signal arrives. It returns an error rather than calling os.Exit so every
// deferred cleanup runs and the failure is logged in one place (main).
func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Migrations are the single source of truth (ADR-0002); apply them before
	// opening the app's pool so the schema is present when we start serving.
	logger.Info("applying migrations", "dir", cfg.MigrationsDir)
	if err := migrate.Up(cfg.DatabaseURL, cfg.MigrationsDir); err != nil {
		return err
	}

	// Observability first (PRD story 56): Setup installs the global OTel
	// MeterProvider that exports to the collector, so instruments built below —
	// the cache MetricsRecorder and the HTTP request middleware — reach it.
	// Export is optional; an empty OTEL_EXPORTER_OTLP_ENDPOINT yields a no-op
	// provider. Shutdown flushes the final metric batch (bounded by its own
	// context) after serving stops.
	ctx := context.Background()
	obs, err := observability.Setup(ctx, cfg.OTelEndpoint)
	if err != nil {
		return err
	}
	defer func() {
		flushCtx, cancel := context.WithTimeout(context.Background(), metricsFlushTimeout)
		defer cancel()
		if err := obs.Shutdown(flushCtx); err != nil {
			logger.Warn("flushing metrics on shutdown", "err", err)
		}
	}()
	if cfg.OTelEndpoint != "" {
		logger.Info("metrics export enabled", "endpoint", cfg.OTelEndpoint)
	}

	// One pgx pool, one Redis cache, one NATS bus, one *database.Client, shared
	// across the whole app — built through internal/appdb, the same construction
	// path the seeder uses (issue 16). config.Load already requires REDIS_URL and
	// NATS_URL, so the cache and bus are always wired; the cache recorder exports
	// the query-path metrics (PRD story 56). Stack.Close tears the resources down
	// in reverse order.
	stack, err := appdb.New(ctx, cfg, obs.CacheRecorder(), logger)
	if err != nil {
		return err
	}
	defer func() {
		if err := stack.Close(); err != nil {
			logger.Warn("closing data-access stack", "err", err)
		}
	}()
	client := stack.Client

	// Start the Activity projector: it subscribes to every mutation event and
	// writes the activity feed through the same client. Unsubscribe on shutdown
	// (bus.Close would also stop it, but the explicit stop keeps ordering clear).
	projectorSub, err := events.NewProjector(client).Subscribe(stack.Bus)
	if err != nil {
		return fmt.Errorf("subscribing activity projector: %w", err)
	}
	defer func() { _ = projectorSub.Unsubscribe() }()

	authSvc := auth.NewService(cfg.JWTSecret, auth.DefaultTokenTTL)

	srv := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: server.New(client, authSvc, obs.MeterProvider()),
	}

	// SIGINT/SIGTERM cancels serveCtx, which server.Serve treats as the signal
	// to drain in-flight requests and shut down gracefully (PRD story 58).
	serveCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger.Info("listening", "addr", cfg.ListenAddr, "graphql", server.GraphQLPath, "health", server.HealthPath)
	return server.Serve(serveCtx, srv, shutdownTimeout, logger)
}
