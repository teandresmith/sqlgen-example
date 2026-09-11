package server

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/errcode"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

// Default query-safety limits (PRD story 51). Both are deliberately generous
// relative to the app's real read shape — the deepest relationship-loading query
// the server answers today nests ~4 levels and selects ~12 fields — so a
// legitimate client is never clipped, while a pathologically nested or sprawling
// query (the DoS vector an anonymous-friendly GraphQL endpoint must reject) is
// refused before it reaches a resolver. They are the ceiling, not a budget.
const (
	// QueryDepthLimit caps how deeply a query may nest selection sets. Depth
	// counts field hops only: `task { subtasks { id } }` is depth 2.
	QueryDepthLimit = 12
	// QueryComplexityLimit caps a query's gqlgen-computed complexity, which —
	// absent per-field cost overrides — is the number of fields resolved.
	QueryComplexityLimit = 200
)

// errDepthLimit is the extensions.code tagged on a depth-limit rejection,
// mirroring gqlgen's own COMPLEXITY_LIMIT_EXCEEDED for the complexity extension.
const errDepthLimit = "DEPTH_LIMIT_EXCEEDED"

// FixedDepthLimit rejects any operation whose selection set nests deeper than
// Limit. gqlgen ships a complexity limiter but no depth limiter, so this is the
// missing half of query safety (PRD story 51): complexity bounds the total work
// a query asks for, depth bounds how far a single nested path recurses (the
// relationship-walk explosion a complexity budget alone can under-count).
//
// It runs as an OperationContextMutator — after parse and validation, before
// execution — so an over-deep query is refused without any resolver, tenant
// lookup, or database call running. The check walks the validated AST, so
// fragment spreads are followed through their resolved definitions and only
// fields add depth (inline and named fragments are transparent).
type FixedDepthLimit struct {
	Limit int
}

var _ interface {
	graphql.OperationContextMutator
	graphql.HandlerExtension
} = FixedDepthLimit{}

// ExtensionName identifies the extension in gqlgen's registry.
func (FixedDepthLimit) ExtensionName() string { return "DepthLimit" }

// Validate is a no-op: a fixed limit needs no schema-time preparation.
func (FixedDepthLimit) Validate(graphql.ExecutableSchema) error { return nil }

// MutateOperationContext computes the operation's depth and rejects it when it
// exceeds the limit, tagging the error with DEPTH_LIMIT_EXCEEDED so clients can
// distinguish a safety rejection from a resolver error.
func (d FixedDepthLimit) MutateOperationContext(ctx context.Context, opCtx *graphql.OperationContext) *gqlerror.Error {
	op := opCtx.Doc.Operations.ForName(opCtx.OperationName)
	if op == nil {
		return nil
	}
	depth := selectionSetDepth(op.SelectionSet)
	if depth > d.Limit {
		err := gqlerror.Errorf("operation has depth %d, which exceeds the limit of %d", depth, d.Limit)
		errcode.Set(err, errDepthLimit)
		return err
	}
	return nil
}

// selectionSetDepth returns the maximum field-nesting depth of a selection set.
// Only fields add a level; inline fragments and fragment spreads are transparent
// (their contents' fields are counted at the fragment's own level). Fragment
// spreads are followed through their resolved Definition — validation, which runs
// before this mutator, has already populated it and rejected any fragment cycle,
// so the recursion terminates.
func selectionSetDepth(set ast.SelectionSet) int {
	maxDepth := 0
	for _, sel := range set {
		var d int
		switch s := sel.(type) {
		case *ast.Field:
			d = 1 + selectionSetDepth(s.SelectionSet)
		case *ast.InlineFragment:
			d = selectionSetDepth(s.SelectionSet)
		case *ast.FragmentSpread:
			if s.Definition != nil {
				d = selectionSetDepth(s.Definition.SelectionSet)
			}
		}
		if d > maxDepth {
			maxDepth = d
		}
	}
	return maxDepth
}
