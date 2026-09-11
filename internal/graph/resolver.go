package graph

import (
	"github.com/teandresmith/sqlgen-example/internal/auth"
	database "github.com/teandresmith/sqlgen-example/internal/database"
	"github.com/teandresmith/sqlgen-example/internal/graph/sqlgenresolver"
)

// Resolver is the root resolver struct. The Client / Q / M fields are
// pre-populated by sqlgen (FIX-064) — initialize them at app boot before
// serving traffic so the per-table delegations have a Client to dispatch
// through. Add additional dependency fields (loggers, auth clients, etc.)
// below the Q / M lines as your consumer code requires; sqlgen never
// overwrites resolver.go after the first run, so consumer edits are
// preserved verbatim.
//
// gqlgen owns the Query() / Mutation() interface methods and the
// queryResolver / mutationResolver type defs — those are emitted into the
// schema-source resolver file (typically shared_gen.resolvers.go).
type Resolver struct {
	Client *database.Client
	Q      *sqlgenresolver.Q
	M      *sqlgenresolver.M
	// Auth signs/verifies JWTs and hashes passwords for the hand-written
	// register/login/me resolvers (issue 02). Consumer-added field; sqlgen
	// preserves it verbatim across regeneration.
	Auth *auth.Service
}
