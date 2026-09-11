// Package appdb is the single, blessed construction path for the app's
// data-access stack: the pgx pool, the Redis-backed cache, the NATS event bus,
// and the *database.Client wired on top of them. Both entrypoints that need a
// production client — the server (cmd/server) and the seeder (cmd/seed) — build
// it here, so the seeder exercises byte-for-byte the same tenant scoping, authz
// hook, cache, and event publishing the server does rather than reconstructing a
// parallel wiring that could drift (issue 16 acceptance: the seeder uses the
// same *database.Client construction as the server).
//
// The server owns the concerns that are genuinely its own — observability,
// graceful shutdown, and the Activity projector — and layers them on the Stack
// this package returns. The projector consumes from Stack.Bus; everything else
// it needs is Stack.Client.
package appdb

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/teandresmith/sqlgen/cache"
	dbpgx "github.com/teandresmith/sqlgen/database/pgx"
	"github.com/teandresmith/sqlgen/event"
	"github.com/teandresmith/sqlgen/event/natsbus"

	"github.com/teandresmith/sqlgen-example/internal/authz"
	appcache "github.com/teandresmith/sqlgen-example/internal/cache"
	"github.com/teandresmith/sqlgen-example/internal/config"
	"github.com/teandresmith/sqlgen-example/internal/database"
	"github.com/teandresmith/sqlgen-example/internal/events"
	"github.com/teandresmith/sqlgen-example/internal/tenancy"
)

// Stack is the wired data-access stack: the *database.Client and the event bus
// it publishes on, plus the resources whose lifetimes the client depends on.
// Close releases those resources; the caller defers it for the client's
// lifetime.
type Stack struct {
	// Client is the shared client — tenant scoping, the authz hook, the cache,
	// and event publishing, exactly as the server wires it.
	Client *database.Client
	// Bus is the event bus the client publishes mutation events on. The server
	// subscribes its Activity projector to it; the seeder leaves it unconsumed
	// (published events are dropped by core NATS with no subscriber).
	Bus *natsbus.Bus

	pool       *pgxpool.Pool
	closeCache func() error
}

// New builds the data-access stack from cfg. It opens the pgx pool, builds the
// Redis cache, connects the NATS bus, assembles the client with the production
// hook chain, and verifies connectivity with a Ping. On any failure it releases
// whatever it already built, so the caller never has to clean up a partial
// Stack.
//
// recorder observes the cache's hit/miss/latency behavior (PRD story 56): pass
// the OpenTelemetry recorder from internal/observability to export cache
// metrics, or nil to disable them — the seeder, which has no metrics pipeline,
// passes nil.
func New(ctx context.Context, cfg *config.Config, recorder cache.MetricsRecorder, logger *slog.Logger) (*Stack, error) {
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("connecting to postgres: %w", err)
	}

	// The Redis-backed cache serves single-entity Get reads and is invalidated
	// on mutation (issue 05). A cache outage at runtime never breaks a request:
	// the generated read-through routes backend errors through a circuit breaker
	// and falls through to Postgres.
	appCache, closeCache, err := appcache.New(cfg.RedisURL, recorder)
	if err != nil {
		pool.Close()
		return nil, err
	}

	// The NATS event bus: generated mutation hooks publish an event per write
	// (WithEventPublisher below), deferred to Tx.OnCommit so only committed
	// writes fire. ActorMetadata stamps the authenticated caller onto each event.
	bus, err := events.Connect(cfg.NATSURL, logger)
	if err != nil {
		_ = closeCache()
		pool.Close()
		return nil, err
	}

	// The tenant resolver reads the Active Workspace from context, driving
	// sqlgen's structural workspace_id scoping (fail-closed). The authz hook
	// rejects management mutations the caller's Active-Workspace Role does not
	// permit (issue 04). WithCache attaches the cache as the outermost hook so a
	// hit short-circuits before any user or event hook runs.
	client := database.New(dbpgx.New(pool),
		database.WithTenantResolver(tenancy.Resolver()),
		database.WithMutationHook(authz.MutationHook()),
		database.WithCache(appCache),
		database.WithEventPublisher(bus, func(c *event.Config) { c.MetadataFunc = events.ActorMetadata }),
	)

	stack := &Stack{Client: client, Bus: bus, pool: pool, closeCache: closeCache}
	if err := client.Ping(ctx); err != nil {
		_ = stack.Close()
		return nil, fmt.Errorf("pinging postgres: %w", err)
	}
	return stack, nil
}

// Close releases the stack's resources in reverse order of construction: the
// bus drains first, then the cache's Redis connection, then the pgx pool. It
// joins every error so one failing teardown does not mask another.
func (s *Stack) Close() error {
	var errs []error
	if s.Bus != nil {
		if err := s.Bus.Close(); err != nil {
			errs = append(errs, fmt.Errorf("closing event bus: %w", err))
		}
	}
	if s.closeCache != nil {
		if err := s.closeCache(); err != nil {
			errs = append(errs, fmt.Errorf("closing cache: %w", err))
		}
	}
	if s.pool != nil {
		s.pool.Close()
	}
	return errors.Join(errs...)
}
