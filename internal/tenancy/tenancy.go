// Package tenancy establishes the per-request Active Workspace and feeds it to
// sqlgen's structural tenant scoping. It is the runtime half of the tenancy
// linchpin (issue 03): a stdlib middleware reads the `X-Workspace-Id` header,
// validates the authenticated caller's Membership in that Workspace, and places
// the Active Workspace into the request context; the TenantResolver returned by
// Resolver reads it back out at every tenanted read/write so sqlgen injects the
// `workspace_id` filter automatically.
//
// Tenancy is fail-closed (sqlgen.yml `tenancy.required: true`): when no Active
// Workspace is in context the resolver returns tenancy.ErrMissing, so any
// tenant-scoped operation errors before touching the database. `users` and
// `workspaces` are shared (untenanted), so register, login, and this
// middleware's own Membership check all work before any tenant is established.
package tenancy

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	sqltenancy "github.com/teandresmith/sqlgen/tenancy"
	"uuid"

	"github.com/teandresmith/sqlgen-example/internal/auth"
	"github.com/teandresmith/sqlgen-example/internal/database"
)

// WorkspaceHeader selects the Active Workspace for a request. A caller switches
// Workspaces by changing this header — no re-authentication (PRD story 8).
const WorkspaceHeader = "X-Workspace-Id"

// ctxKey is the unexported context-key type for the Active Workspace id.
type ctxKey struct{}

// roleCtxKey is the unexported context-key type for the caller's Role in the
// Active Workspace.
type roleCtxKey struct{}

// WithActiveWorkspace returns a copy of ctx carrying the Active Workspace id.
func WithActiveWorkspace(ctx context.Context, workspaceID uuid.UUID) context.Context {
	return context.WithValue(ctx, ctxKey{}, workspaceID)
}

// ActiveWorkspace returns the Active Workspace id from ctx and whether one is
// present. The TenantResolver uses the ok result to fail closed.
func ActiveWorkspace(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(ctxKey{}).(uuid.UUID)
	return id, ok
}

// WithRole returns a copy of ctx carrying the caller's Role in the Active
// Workspace. The middleware sets it alongside the Active Workspace, from the
// same Membership it validates, so authorization never costs a second lookup.
func WithRole(ctx context.Context, role database.MembershipRole) context.Context {
	return context.WithValue(ctx, roleCtxKey{}, role)
}

// Role returns the caller's Role in the Active Workspace and whether one is
// present. A Role is present exactly when a valid Active Workspace was
// selected (the middleware resolves both from the caller's Membership), so the
// authorization hook can key on it. It is absent for identity-only and
// pre-tenant requests (register, login, bootstrapWorkspace, acceptInvitation).
func Role(ctx context.Context) (database.MembershipRole, bool) {
	role, ok := ctx.Value(roleCtxKey{}).(database.MembershipRole)
	return role, ok
}

// Resolver returns the sqlgen TenantResolver the client is constructed with
// (WithTenantResolver). It reads the Active Workspace the middleware placed in
// context and returns tenancy.ErrMissing when none is present, so a
// tenant-scoped operation with no valid Active Workspace is rejected before a
// database round-trip (fail-closed — PRD story 10).
func Resolver() sqltenancy.TenantResolver[uuid.UUID] {
	return func(ctx context.Context) (uuid.UUID, error) {
		id, ok := ActiveWorkspace(ctx)
		if !ok {
			return uuid.UUID{}, sqltenancy.ErrMissing
		}
		return id, nil
	}
}

// Middleware resolves the Active Workspace for each request. It runs after the
// bearer-token middleware (so the authenticated User is in context) and before
// the GraphQL handler.
//
// Behavior by request shape:
//   - No X-Workspace-Id: the request passes through with no Active Workspace.
//     Public and identity-only operations (register, login, me,
//     bootstrapWorkspace) still work; any tenant-scoped operation fails closed
//     later at the TenantResolver.
//   - X-Workspace-Id present but not a UUID: 400, the header is malformed.
//   - X-Workspace-Id present but unauthenticated: 401, there is no identity to
//     validate a Membership for.
//   - X-Workspace-Id present and the caller is not a Member: 403, so a caller
//     cannot reach another tenant's data (PRD story 9).
//   - X-Workspace-Id present and the caller is a Member: the Active Workspace
//     is placed into the request context.
//
// The Membership lookup bypasses tenancy (SkipTenancy) deliberately: it runs
// before any Active Workspace is established, and the composite primary key
// already pins the lookup to the requested (workspace, user) pair.
//
// The lookup fetches the whole Membership rather than just testing existence,
// so the caller's Role rides into context alongside the Active Workspace. The
// authorization hook (internal/authz) reads that Role to permit or reject
// management operations without a second round-trip (PRD story 50).
func Middleware(client *database.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := r.Header.Get(WorkspaceHeader)
			if raw == "" {
				next.ServeHTTP(w, r)
				return
			}
			workspaceID, err := uuid.Parse(raw)
			if err != nil {
				writeError(w, http.StatusBadRequest, "X-Workspace-Id is not a valid UUID")
				return
			}
			userID, ok := auth.UserID(r.Context())
			if !ok {
				writeError(w, http.StatusUnauthorized, "authentication is required to select a workspace")
				return
			}
			membership, err := client.Memberships().Get(r.Context(),
				database.MembershipPK{WorkspaceID: workspaceID, UserID: userID},
				skipTenancy,
			)
			if errors.Is(err, database.ErrNotFound) {
				writeError(w, http.StatusForbidden, "not a member of the requested workspace")
				return
			}
			if err != nil {
				writeError(w, http.StatusInternalServerError, "could not validate workspace membership")
				return
			}
			ctx := WithActiveWorkspace(r.Context(), workspaceID)
			ctx = WithRole(ctx, membership.Role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// skipTenancy bypasses tenant auto-filtering for a single call — used for the
// pre-tenant Membership lookup in Middleware.
func skipTenancy(o *database.CallOptions[database.MembershipFieldOptions]) {
	o.SkipTenancy = true
}

// writeError emits a small JSON error body with the given status, mirroring the
// shape the auth middleware uses for its 401.
func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
