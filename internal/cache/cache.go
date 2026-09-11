// Package cache wires a Redis-backed sqlgen cache.Backend into the generated
// *database.Cache, the object the client accepts via database.WithCache. It is
// the blessed construction path shared by the server entrypoint (cmd/server)
// and the full-stack integration harness: both build the cache the same way so
// caching behaves identically under test and in production.
//
// The serializer (JSON) and key prefix (taskr) are baked into database.NewCache
// at codegen time from sqlgen.yml — they are not passed here. This package only
// owns the runtime concern the generator cannot know: the Redis connection.
package cache

import (
	"fmt"

	goredis "github.com/redis/go-redis/v9"
	"github.com/teandresmith/sqlgen/cache"
	cacheredis "github.com/teandresmith/sqlgen/cache/redis"

	database "github.com/teandresmith/sqlgen-example/internal/database"
)

// New builds the generated *database.Cache backed by a Redis connection parsed
// from redisURL. Attach the returned Cache to the client with
// database.WithCache. The returned close func releases the Redis connection —
// the Backend is constructed WithOwnedClient, so closing the Cache closes the
// underlying client — and should be deferred for the client's lifetime.
//
// recorder observes the cache's hit/miss/latency behavior (PRD story 56): pass
// the OpenTelemetry recorder from internal/observability to export the
// query-path metrics, or nil to disable them (a zero-cost path). It is passed
// here rather than set globally because the recorder is a construction-time
// property of this specific cache instance.
//
// A single-entity Get is served through this cache (read-through, populated on
// create and by background hydration); a mutation invalidates the affected
// entry so the next read reflects the change. A cache outage never breaks a
// caller: the generated read-through routes backend errors through a circuit
// breaker and falls through to the database.
func New(redisURL string, recorder cache.MetricsRecorder) (*database.Cache, func() error, error) {
	opts, err := goredis.ParseURL(redisURL)
	if err != nil {
		return nil, nil, fmt.Errorf("cache: parsing redis url: %w", err)
	}
	rdb := goredis.NewClient(opts)

	// WithOwnedClient transfers connection lifecycle to the Backend so the
	// returned close func (Cache.Close → Backend.Close) shuts the pool down.
	backend := cacheredis.New(rdb, cacheredis.WithOwnedClient())

	c, err := database.NewCache(backend, database.WithMetricsRecorder(recorder))
	if err != nil {
		return nil, nil, fmt.Errorf("cache: constructing cache: %w", err)
	}
	return c, c.Close, nil
}
