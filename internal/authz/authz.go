// Package authz enforces role-based authorization on top of tenant scoping. It
// is the authorization half of the tenancy story (issue 04): tenant scoping
// (internal/tenancy) decides which Workspace's rows a request can touch; authz
// decides whether the caller's Role in that Workspace permits the operation at
// all.
//
// Enforcement is a mutation hook, not schema (PRD Implementation Decisions):
// the generated client runs MutationHook in its hook chain for every write, and
// for the guarded tables — the ones that administer who has access and at what
// level — it rejects callers whose Active-Workspace Role is below owner/admin.
// A `member` can create Projects and Tasks all day but cannot invite, revoke,
// or re-grade Memberships; that is the whole point of story 50 and story 11.
//
// The Role is resolved once per request by the tenancy middleware (from the
// same Membership it already validates) and read back here from context, so
// authorization costs no extra database round-trip.
//
// Server-side bootstrap paths are the one exception. bootstrapWorkspace mints
// the founding Membership, and acceptInvitation mints the invitee's Membership
// and marks the Invitation accepted, for a caller who is not (yet) a manager of
// the target Workspace — indeed the target Workspace is not the request's Active
// Workspace at all. Those writes are self-authorized (workspace ownership; a
// valid invitation token) and mark themselves as trusted in one of two ways,
// both settable only by server code and never by a client:
//
//   - SkipTenancy in CallOptions — for writes that also opt out of tenant
//     scoping (e.g. the founding Membership, whose workspace_id is a row created
//     in the same transaction).
//   - A system grant on the context (WithSystemGrant) — for writes that must run
//     tenant-resolved so the cache can invalidate their entries (e.g. marking an
//     Invitation accepted, which must evict the entry cached when it was sent).
//     SkipTenancy leaves the tenant unresolved, so the per-tenant cache key can
//     never be built; the system grant carries the trust without giving up
//     tenant resolution.
//
// Either signal bypasses the Role check on a guarded table: the ambient
// Active-Workspace Role is the wrong signal for a write that targets a different
// Workspace. Every other guarded write is fail-closed — a caller with no manager
// Role is rejected here, before the terminal runs.
package authz

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/teandresmith/sqlgen/hook"

	"github.com/teandresmith/sqlgen-example/internal/database"
	"github.com/teandresmith/sqlgen-example/internal/tenancy"
)

// systemGrantKey is the unexported context key marking a request as a trusted
// server-side operation that has done its own authorization out of band.
type systemGrantKey struct{}

// WithSystemGrant marks ctx as a trusted server-side operation, authorizing
// guarded-table writes without an Active-Workspace manager Role. It is for
// bootstrap paths self-authorized by other means (a valid invitation token)
// that must nonetheless run tenant-resolved — not SkipTenancy — so the cache
// invalidates their writes. Only server code can set a context value, so this is
// as unforgeable as the SkipTenancy signal it complements; a client can never
// grant itself the bypass.
func WithSystemGrant(ctx context.Context) context.Context {
	return context.WithValue(ctx, systemGrantKey{}, struct{}{})
}

// systemGranted reports whether ctx carries a system grant.
func systemGranted(ctx context.Context) bool {
	return ctx.Value(systemGrantKey{}) != nil
}

// ErrForbidden is the sentinel a guarded mutation returns when the caller's
// Role does not permit it. It surfaces to the client as a GraphQL error.
var ErrForbidden = errors.New("forbidden: your role does not permit this operation")

// managerRoles are the Roles allowed to administer access — invite, revoke, and
// manage Memberships and Roles (PRD stories 5, 7, 11). Owner and admin manage;
// member and guest do not. (Stories 5 and 7 name the owner; the PRD's
// authorization decision groups owner/admin as the managers of Memberships, so
// an admin may invite and revoke too — consistent with story 11.)
var managerRoles = map[database.MembershipRole]bool{
	database.MembershipRoleOwner: true,
	database.MembershipRoleAdmin: true,
}

// guardedTables are the tables whose writes administer workspace access and so
// require a manager Role. Membership grades who belongs and at what level;
// Invitation offers access to a new email. Add a table here (Labels, SSO
// Connections, …) as later tickets bring administrative surfaces online — and
// add the matching case to selfAuthorized so its bootstrap paths keep working.
var guardedTables = map[hook.TableName]bool{
	database.TableMemberships: true,
	database.TableInvitations: true,
	// Labels are Workspace-administered: only an owner/admin creates, renames,
	// recolors, or deletes them (issue 08, story 28). Applying an existing Label
	// to a Task is NOT guarded — task_labels is deliberately absent here, so any
	// member can tag work (story 27); the guard is on the Label catalog itself,
	// not on its use.
	database.TableLabels: true,
	// Cycles are Workspace-administered: only an owner/admin defines an iteration
	// — its date range and status (issue 10, story 32). Scheduling a Task INTO a
	// Cycle is NOT guarded — that is a write on tasks (unguarded), so any member
	// may plan work via updateCycleWithRelated's tasks.connect (story 31), whose
	// empty parent update passes setsNoColumn; the guard is on the Cycle catalog
	// itself, not on its use. Mirrors the Labels split above.
	database.TableCycles: true,
	// Teams are Workspace-administered: only an owner/admin creates a Team (issue
	// 12, story 16). Assigning a Project TO a Team is NOT guarded — that is a
	// write on projects (unguarded), so any member may assign ownership via
	// updateTeamWithRelated's projects.connect (story 18), whose empty parent
	// update passes setsNoColumn; the guard is on the Team catalog itself, not on
	// its use. Mirrors the Cycles/Labels split above.
	database.TableTeams: true,
	// team_members administers who belongs to a Team: only an owner/admin adds or
	// removes Users (issue 12, story 17). The junction carries no workspace_id, so
	// the generated write mutations are subtracted from the API (sqlgen.yml) and
	// the only write path is the blessed addUserToTeam / removeUserFromTeam, which
	// resolve the Team tenant-scoped first. Guarding the table here makes those
	// blessed writes owner/admin-only through the same hook.
	database.TableTeamMembers: true,
	// SSO Connections are Workspace-administered: only an owner/admin configures
	// single sign-on for a Workspace (issue 13, story 12). The blessed
	// configureSSOConnection upserts the provider config and the generated
	// deleteSsoConnection removes it — both write sso_connections, so guarding the
	// table here makes them owner/admin-only through the same hook. API Tokens are
	// deliberately NOT guarded here: creating one is member self-service (a User
	// mints a token to act on their own behalf, stories 13–14), so it is
	// authorized by identity + Active Workspace alone, and revocation is
	// authorized in the blessed resolver (owner of the token, or a manager).
	database.TableSsoConnections: true,
}

// MutationHook returns the authorization hook to register with WithMutationHook.
// For a write on a guarded table it reads the caller's Active-Workspace Role
// from context and rejects any Role below owner/admin. Writes on unguarded
// tables pass straight through, as do self-authorized server-side writes (see
// selfAuthorized and the package doc), and updates that set no column
// (setsNoColumn).
func MutationHook() hook.MutationHook {
	return func(next hook.MutationHandler) hook.MutationHandler {
		return func(ctx context.Context, m *hook.MutationContext) (any, error) {
			if !guardedTables[m.Table] {
				return next(ctx, m)
			}
			if selfAuthorized(ctx, m.CallOptions) {
				return next(ctx, m)
			}
			if m.Op == hook.OpUpdate && setsNoColumn(m.Input) {
				return next(ctx, m)
			}
			role, ok := tenancy.Role(ctx)
			if !ok || !managerRoles[role] {
				return nil, fmt.Errorf("%w: %s %s requires an owner or admin role", ErrForbidden, m.Op, m.Table)
			}
			return next(ctx, m)
		}
	}
}

// setsNoColumn reports whether an update input sets no column at all. Such an
// update writes nothing — the client returns the row unchanged (PRD §9.5) — so
// it administers nothing and needs no manager Role. This is what lets a member
// run a nested update that only links children to a guarded parent, e.g.
// updateCycleWithRelated(id, { tasks: { connect: [...] } }) to schedule a Task
// (story 31) or updateTeamWithRelated to assign a Project (story 18): the
// nested executor first calls the parent's Update with an empty input, and its
// tenant-scoped read is what proves the Cycle/Team is in the Active Workspace.
// Any set field — renaming the Cycle, re-dating it — is still guarded.
//
// Every generated Update<T>Input field is an omittable.Value, which reports
// IsSet. Fail closed on anything else: an input that is not a struct pointer,
// or a field that is unexported or cannot say whether it is set, counts as
// setting a column.
func setsNoColumn(input any) bool {
	v := reflect.ValueOf(input)
	if v.Kind() != reflect.Pointer || v.IsNil() || v.Elem().Kind() != reflect.Struct {
		return false
	}
	v = v.Elem()
	for i := range v.NumField() {
		if !v.Type().Field(i).IsExported() {
			return false
		}
		f, ok := v.Field(i).Interface().(interface{ IsSet() bool })
		if !ok || f.IsSet() {
			return false
		}
	}
	return true
}

// selfAuthorized reports whether a mutation is a trusted server-side bootstrap
// path (bootstrapWorkspace, acceptInvitation) that has already done its own
// authorization. It is trusted when the context carries a system grant, or when
// the mutation opts out of tenant scoping (SkipTenancy) — both settable only by
// server code, never by a client, so either is a safe bypass signal. The type
// switch enumerates the guarded tables' CallOptions — keep it in sync with
// guardedTables.
func selfAuthorized(ctx context.Context, callOptions any) bool {
	if systemGranted(ctx) {
		return true
	}
	switch o := callOptions.(type) {
	case database.CallOptions[database.MembershipFieldOptions]:
		return o.SkipTenancy
	case database.CallOptions[database.InvitationFieldOptions]:
		return o.SkipTenancy
	case database.CallOptions[database.LabelFieldOptions]:
		// No server-side Label bootstrap path exists today; this case keeps the
		// switch in sync with guardedTables so a future SkipTenancy Label write
		// (e.g. seeding default Labels in bootstrapWorkspace) is trusted.
		return o.SkipTenancy
	case database.CallOptions[database.CycleFieldOptions]:
		// No server-side Cycle bootstrap path exists today; this case keeps the
		// switch in sync with guardedTables so a future SkipTenancy Cycle write
		// (e.g. seeding a default Cycle) is trusted.
		return o.SkipTenancy
	case database.CallOptions[database.TeamFieldOptions]:
		// No server-side Team bootstrap path exists today; this case keeps the
		// switch in sync with guardedTables so a future SkipTenancy Team write
		// (e.g. seeding default Teams in bootstrapWorkspace) is trusted.
		return o.SkipTenancy
	case database.CallOptions[database.TeamMemberFieldOptions]:
		// No server-side team_members bootstrap path exists today; this case keeps
		// the switch in sync with guardedTables so a future SkipTenancy membership
		// write is trusted.
		return o.SkipTenancy
	case database.CallOptions[database.SsoConnectionFieldOptions]:
		// No server-side SSO bootstrap path exists today; this case keeps the switch
		// in sync with guardedTables so a future SkipTenancy SSO write (e.g. seeding
		// a default connection) is trusted.
		return o.SkipTenancy
	default:
		return false
	}
}
