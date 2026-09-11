// Package test is the full-stack integration harness (ADR-0005). A shared
// TestMain boots the entire backing stack — Postgres, Redis, and NATS — in
// testcontainers, applies the ./migrations DDL, and stands up the exact wired
// HTTP handler the server binary serves (via internal/server.New) behind an
// httptest server. Tests drive that one seam — the GraphQL HTTP endpoint — by
// POSTing operations, so they read like a client of the running service and
// exercise the real wiring rather than reconstructing it.
package test

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	dbpgx "github.com/teandresmith/sqlgen/database/pgx"
	"github.com/teandresmith/sqlgen/event"
	sqlgenotel "github.com/teandresmith/sqlgen/metrics/otel"
	"github.com/testcontainers/testcontainers-go"
	natstc "github.com/testcontainers/testcontainers-go/modules/nats"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	redistc "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/teandresmith/sqlgen-example/internal/auth"
	"github.com/teandresmith/sqlgen-example/internal/authz"
	appcache "github.com/teandresmith/sqlgen-example/internal/cache"
	database "github.com/teandresmith/sqlgen-example/internal/database"
	"github.com/teandresmith/sqlgen-example/internal/events"
	"github.com/teandresmith/sqlgen-example/internal/migrate"
	"github.com/teandresmith/sqlgen-example/internal/server"
	"github.com/teandresmith/sqlgen-example/internal/tenancy"
)

// testJWTSecret signs tokens in the harness. The test auth service is built
// with it and shared into both server.New and the JWT-minting test helper, so
// tests authenticate through the exact signing code the server verifies with.
const testJWTSecret = "integration-test-secret"

var (
	testPool   *pgxpool.Pool
	testClient *database.Client
	testServer *httptest.Server

	// Connection strings for the amortized backing services. Postgres is wired
	// into the client today; Redis and NATS are booted here so later tickets
	// (cache, events) ride on the same shared harness without a compose change.
	redisURL string
	natsURL  string

	// testAuth is the auth service the harness wires into the server; the JWT
	// test helper mints tokens with it (the server's own signing code).
	testAuth *auth.Service

	// testMetricReader backs the harness MeterProvider. It is a ManualReader so
	// the observability test can snapshot the request and cache/query metrics
	// the wired server emitted, without an OTLP collector or a timed export
	// loop (PRD story 56). Every integration test drives the metrics path for
	// real because the same provider is wired into the client and HTTP server.
	testMetricReader *sdkmetric.ManualReader
)

func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		// Container-backed integration tests need Docker; -short runs unit
		// tests elsewhere without it.
		os.Exit(0)
	}
	os.Exit(run(m))
}

// run boots the stack, runs the tests, and tears everything down. It is split
// out from TestMain so deferred cleanup runs before os.Exit (which TestMain
// must call with the test result code).
func run(m *testing.M) int {
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("taskr_e2e"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
		),
	)
	if err != nil {
		panic(fmt.Sprintf("starting postgres container: %v", err))
	}
	defer func() { _ = pgContainer.Terminate(ctx) }()

	redisContainer, err := redistc.Run(ctx, "redis:7-alpine")
	if err != nil {
		panic(fmt.Sprintf("starting redis container: %v", err))
	}
	defer func() { _ = redisContainer.Terminate(ctx) }()

	natsContainer, err := natstc.Run(ctx, "nats:2-alpine")
	if err != nil {
		panic(fmt.Sprintf("starting nats container: %v", err))
	}
	defer func() { _ = natsContainer.Terminate(ctx) }()

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		panic(fmt.Sprintf("postgres connection string: %v", err))
	}
	if redisURL, err = redisContainer.ConnectionString(ctx); err != nil {
		panic(fmt.Sprintf("redis connection string: %v", err))
	}
	if natsURL, err = natsContainer.ConnectionString(ctx); err != nil {
		panic(fmt.Sprintf("nats connection string: %v", err))
	}

	// Migrations run against the fresh container through the same runner the
	// server uses, so the schema under test is byte-for-byte the app's schema.
	if err := migrate.Up(connStr, "../migrations"); err != nil {
		panic(fmt.Sprintf("applying migrations: %v", err))
	}

	testPool, err = pgxpool.New(ctx, connStr)
	if err != nil {
		panic(fmt.Sprintf("creating pool: %v", err))
	}
	defer testPool.Close()

	// A MeterProvider backed by a ManualReader so tests can snapshot emitted
	// metrics directly (no OTLP collector). The same provider feeds the cache
	// recorder and the HTTP request middleware, so the metrics wiring under
	// test is the production path with only the exporter swapped (PRD story 56).
	testMetricReader = sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(testMetricReader))
	defer func() { _ = meterProvider.Shutdown(ctx) }()

	// The Redis-backed cache, built exactly as the binary builds it, so cache
	// hit/invalidation behavior under test is the production wiring (issue 05).
	// The OTel recorder is attached so cache/query metrics reach the reader.
	appCache, closeCache, err := appcache.New(redisURL, sqlgenotel.New(meterProvider))
	if err != nil {
		panic(fmt.Sprintf("building cache: %v", err))
	}
	defer func() { _ = closeCache() }()

	// The NATS event bus, wired exactly as the binary wires it (issue 06): the
	// client publishes mutation events onto it, and the Activity projector
	// consumes them into the activity feed. Built against the same NATS
	// testcontainer, so the events → projector → feed path under test is the
	// production wiring end to end.
	bus, err := events.Connect(natsURL, slog.Default())
	if err != nil {
		panic(fmt.Sprintf("connecting event bus: %v", err))
	}
	defer func() { _ = bus.Close() }()

	// Same wiring the binary builds (ADR-0005): tenant scoping, the authz hook,
	// the Redis cache, and event publishing (with actor metadata), so tests
	// exercise authorization, caching, and the activity feed exactly as
	// production does.
	testClient = database.New(dbpgx.New(testPool),
		database.WithTenantResolver(tenancy.Resolver()),
		database.WithMutationHook(authz.MutationHook()),
		database.WithCache(appCache),
		database.WithEventPublisher(bus, func(c *event.Config) { c.MetadataFunc = events.ActorMetadata }),
	)
	testAuth = auth.NewService(testJWTSecret, auth.DefaultTokenTTL)

	projectorSub, err := events.NewProjector(testClient).Subscribe(bus)
	if err != nil {
		panic(fmt.Sprintf("subscribing activity projector: %v", err))
	}
	defer func() { _ = projectorSub.Unsubscribe() }()

	// The seam under test: the exact handler cmd/server serves, wrapped with the
	// same request-metrics middleware via the shared MeterProvider.
	testServer = httptest.NewServer(server.New(testClient, testAuth, meterProvider))
	defer testServer.Close()

	return m.Run()
}

// gqlResponse is the standard GraphQL response envelope. `data` is kept raw so
// each test unmarshals into its own typed struct.
type gqlResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []gqlError      `json:"errors"`
}

type gqlError struct {
	Message    string         `json:"message"`
	Path       []any          `json:"path"`
	Extensions map[string]any `json:"extensions"`
}

// gqlExec POSTs a GraphQL operation to the wired server's /query endpoint and
// returns the parsed envelope. It does not fail on GraphQL errors — callers
// inspect resp.Errors to assert success or an expected code.
func gqlExec(t *testing.T, query string, variables map[string]any, headers map[string]string) gqlResponse {
	t.Helper()
	body, err := json.Marshal(map[string]any{"query": query, "variables": variables})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, testServer.URL+server.GraphQLPath, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("send request: %v", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected HTTP status %d: %s", resp.StatusCode, raw)
	}
	var out gqlResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode response (raw=%s): %v", raw, err)
	}
	return out
}

// gqlExecData runs gqlExec with the given headers, asserts no GraphQL errors,
// and unmarshals data. Pass nil headers for an unauthenticated request.
func gqlExecData(t *testing.T, query string, variables map[string]any, headers map[string]string, out any) {
	t.Helper()
	resp := gqlExec(t, query, variables, headers)
	if len(resp.Errors) > 0 {
		t.Fatalf("graphql errors: %+v", resp.Errors)
	}
	if err := json.Unmarshal(resp.Data, out); err != nil {
		t.Fatalf("decode data (raw=%s): %v", resp.Data, err)
	}
}

// decode unmarshals a gqlResponse's data into out. It is for callers that first
// inspected resp.Errors themselves — e.g. to assert an expected error path — and
// then want to decode the data of a success.
func decode(t *testing.T, resp gqlResponse, out any) {
	t.Helper()
	if err := json.Unmarshal(resp.Data, out); err != nil {
		t.Fatalf("decode data (raw=%s): %v", resp.Data, err)
	}
}

// postStatus POSTs a GraphQL operation with the given headers and returns only
// the HTTP status code. Used to assert transport-layer rejection (e.g. a 401
// from the bearer-token middleware before the GraphQL layer runs).
func postStatus(t *testing.T, query string, headers map[string]string) int {
	t.Helper()
	body, err := json.Marshal(map[string]any{"query": query})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, testServer.URL+server.GraphQLPath, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("send request: %v", err)
	}
	defer resp.Body.Close()
	return resp.StatusCode
}
