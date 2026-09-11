package server

import (
	"context"
	"errors"

	"github.com/99designs/gqlgen/graphql"
	"github.com/teandresmith/sqlgen/tenancy"
	"github.com/vektah/gqlparser/v2/gqlerror"

	database "github.com/teandresmith/sqlgen-example/internal/database"
)

// errorPresenter is the server-boundary translation of a sqlgen runtime error
// into a typed GraphQL error (PRD story 48/49; issue 14). It is the companion of
// the generated per-resolver `mapErrorToGQL`: the generated Q/M resolvers already
// tag their errors, but the app's hand-written blessed resolvers (bootstrapWorkspace,
// inviteToWorkspace, addTaskDependency, …) return the raw runtime error wrapped
// with context. Registering this as gqlgen's ErrorPresenter closes that gap so
// every mutation — generated or hand-written — surfaces a not-found, unique,
// foreign-key, or check-constraint violation as the same typed error a client can
// branch on, rather than a leaked INTERNAL string.
//
// The mapping is intentionally identical to the generated `mapErrorToGQL`
// (internal/graph/sqlgenresolver/errors_gen.go): a duplicate here is unavoidable
// because that function is unexported, but the two must agree on codes. Errors an
// upstream resolver already mapped arrive as a *gqlerror.Error that no longer
// wraps the sentinel, so they fall straight through to graphql.DefaultErrorPresenter,
// which preserves their code — this presenter never double-maps them.
func errorPresenter(ctx context.Context, err error) *gqlerror.Error {
	switch {
	case errors.Is(err, database.ErrNotFound):
		return typedError(ctx, err, "not found", "NOT_FOUND")
	case errors.Is(err, tenancy.ErrMissing):
		return typedError(ctx, err, "tenant required", "UNAUTHENTICATED")
	case errors.Is(err, tenancy.ErrMismatch):
		return typedError(ctx, err, "tenant mismatch", "FORBIDDEN")
	}

	var ce *database.ConstraintError
	if errors.As(err, &ce) {
		switch ce.Type {
		case database.ConstraintUnique:
			return typedError(ctx, err, "duplicate", "CONFLICT")
		case database.ConstraintForeignKey:
			return typedError(ctx, err, "fk constraint", "BAD_REFERENCE")
		case database.ConstraintCheck:
			return typedError(ctx, err, "check failed", "INVALID_INPUT")
		case database.ConstraintNotNull:
			return typedError(ctx, err, "missing required", "INVALID_INPUT")
		}
	}

	// An error a generated resolver already mapped arrives as a *gqlerror.Error —
	// message, path, and extensions.code intact — as does a depth/complexity/
	// validation rejection carrying its own code. Preserve it untouched.
	var gqlErr *gqlerror.Error
	if errors.As(err, &gqlErr) {
		return graphql.DefaultErrorPresenter(ctx, err)
	}

	// A raw, unmapped error from a hand-written blessed resolver: tag it INTERNAL,
	// matching the generated mapErrorToGQL's default branch (errors_gen.go), so no
	// resolver — generated or hand-written — ever surfaces an untyped error.
	return typedError(ctx, err, err.Error(), "INTERNAL")
}

// typedError builds a GraphQL error carrying a stable code in extensions, while
// preserving the resolver path gqlgen recorded for the failing field so clients
// can still locate which selection failed.
func typedError(ctx context.Context, err error, message, code string) *gqlerror.Error {
	gErr := gqlerror.WrapPath(graphql.GetPath(ctx), err)
	gErr.Message = message
	if gErr.Extensions == nil {
		gErr.Extensions = map[string]any{}
	}
	gErr.Extensions["code"] = code
	return gErr
}
