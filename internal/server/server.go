// Package server wires the generated GraphQL surface into a runnable
// http.Handler. It is the single blessed bootstrap path shared by the server
// entrypoint (cmd/server) and the full-stack integration harness (ADR-0005):
// tests drive exactly the handler the binary serves, so auth, resolvers, the
// generated client, and the per-request CallOptions bridge are all exercised
// for real rather than reconstructed in the test.
package server

import (
	"encoding/json"
	"net/http"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"go.opentelemetry.io/otel/metric"

	"github.com/teandresmith/sqlgen-example/internal/apitoken"
	"github.com/teandresmith/sqlgen-example/internal/auth"
	database "github.com/teandresmith/sqlgen-example/internal/database"
	"github.com/teandresmith/sqlgen-example/internal/graph"
	"github.com/teandresmith/sqlgen-example/internal/graph/sqlgenresolver"
	"github.com/teandresmith/sqlgen-example/internal/observability"
	"github.com/teandresmith/sqlgen-example/internal/tenancy"
)

// Route paths for the HTTP surface.
const (
	// GraphQLPath is where GraphQL operations are POSTed.
	GraphQLPath = "/query"
	// HealthPath is the readiness endpoint orchestration polls.
	HealthPath = "/healthz"
)

// New builds the fully-wired HTTP handler for the given database client and
// auth service. The generated Resolver is initialized with the shared client,
// the sqlgen query/mutation delegates (Q/M), and the auth service the
// register/login/me resolvers use. The executable schema is served by the
// gqlgen handler, wrapped (innermost to outermost) by WithCallOptionsMiddleware
// (the per-request HTTP → CallOptions bridge, PRD §26.11), the Active-Workspace
// middleware (resolves X-Workspace-Id into the request context so the client's
// TenantResolver can scope tenanted operations), and the bearer-token
// middleware. Order matters: bearer-token runs first so the authenticated User
// is in context when the tenancy middleware validates the caller's Membership.
// A readiness endpoint that pings the database is mounted alongside.
//
// Two authentication front doors share the endpoint. An API-Token request
// (issue 13) is self-contained — the token pins its Workspace and, when personal,
// its User — so the outermost apitoken.Middleware validates it and dispatches
// straight to the inner handler with the Active Workspace (and Role) already in
// context. Every other request falls through to the interactive chain: the
// bearer-JWT middleware (identity) then the Active-Workspace middleware
// (X-Workspace-Id → validated Membership). Both paths converge on the same inner
// handler, so resolvers, tenancy, and authz behave identically whichever
// credential authenticated the caller.
//
// mp instruments the HTTP surface with request metrics (PRD story 56): the
// whole mux is wrapped so every route — GraphQL and health — is measured with
// its matched pattern as the route label. Pass observability.Setup's
// MeterProvider (a no-op provider when export is disabled); it is never nil.
func New(client *database.Client, authSvc *auth.Service, mp metric.MeterProvider) http.Handler {
	resolver := &graph.Resolver{
		Client: client,
		Q:      &sqlgenresolver.Q{Client: client},
		M:      &sqlgenresolver.M{Client: client},
		Auth:   authSvc,
	}
	es := graph.NewExecutableSchema(graph.Config{Resolvers: resolver})

	// Query safety (PRD story 51): reject expensive nested queries before they
	// reach a resolver. Complexity bounds the total fields a query resolves;
	// depth bounds how far a single relationship path recurses. Both run as
	// operation-context mutators after validation, so an over-budget query never
	// touches the client, tenant lookup, or database.
	gqlSrv := handler.NewDefaultServer(es)
	gqlSrv.Use(extension.FixedComplexityLimit(QueryComplexityLimit))
	gqlSrv.Use(FixedDepthLimit{Limit: QueryDepthLimit})
	// Typed error mapping across every resolver, generated or hand-written
	// (issue 14): a not-found/unique/foreign-key/check violation surfaces with a
	// stable extensions.code rather than a leaked internal string.
	gqlSrv.SetErrorPresenter(errorPresenter)

	inner := sqlgenresolver.WithCallOptionsMiddleware(gqlSrv)
	interactive := authSvc.Middleware(tenancy.Middleware(client)(inner))
	gql := apitoken.Middleware(client, inner)(interactive)

	mux := http.NewServeMux()
	mux.Handle(GraphQLPath, gql)
	mux.Handle(HealthPath, healthHandler(client))

	// Wrap the mux (not each handler) so the request-metrics middleware reads
	// the matched route pattern for its label. This is the outermost layer, so
	// it times the full request including auth and tenancy middleware.
	return observability.Middleware(mp)(mux)
}

// healthHandler reports readiness. It pings the database on each call so the
// endpoint reflects real liveness of the backing store, not just that the
// process is up — fail-closed with 503 if the ping fails.
func healthHandler(client *database.Client) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := client.Ping(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "unavailable"})
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
}
