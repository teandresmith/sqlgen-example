# 04 — Authorization hooks & membership administration

Status: done
Blocked by: 03-workspaces-membership-tenant-scoping

Parent: `.scratch/app-server/PRD.md`

## What to build

Role-based authorization beyond tenant scoping, plus the invitation and membership lifecycle it guards. A mutation/query hook keyed on the caller's Role for the Active Workspace rejects operations the Role does not permit (read/write authz is a hook concern, not schema). On top of it: a Workspace owner invites a person by email with a Role; the invited person accepts, creating a Membership and marking the Invitation `accepted`; an owner revokes a pending Invitation so it can no longer be accepted; and admins manage Memberships and Roles.

## Acceptance criteria

- [x] An authz hook keyed on the caller's Role for the Active Workspace rejects forbidden operations (story 50)
- [x] A Workspace owner can invite a person by email with a Role (story 5)
- [x] An invited person can accept an Invitation, creating a Membership and marking the Invitation `accepted` (story 6)
- [x] A Workspace owner can revoke a pending Invitation, after which it cannot be accepted (story 7)
- [x] A Workspace admin can manage Memberships and Roles (story 11)
- [x] Integration test proves an insufficient-Role caller is rejected for an owner/admin-only operation

## Comments

Implemented on branch `HEAD` (issue-04).

Authorization is a `hook.MutationHook` (`internal/authz`) registered via
`WithMutationHook` in both `cmd/server` and the test harness. It guards writes on
the `memberships` and `invitations` tables, requiring the caller's Active-Workspace
Role to be `owner` or `admin`. The Role is resolved once per request: the tenancy
middleware now `Get`s the caller's Membership (instead of `Exists`) and stashes the
Role in context alongside the Active Workspace, so authz costs no extra round-trip.

The invitation/membership lifecycle is three hand-written mutations
(`internal/graph/membership_admin.{graphqls,resolvers.go}`), preferred over the raw
generated CRUD the way `bootstrapWorkspace` is preferred over `createWorkspace`:

- `inviteToWorkspace(email, role)` — creates a pending Invitation in the Active
  Workspace; server owns the token/status/issuer/expiry. Guarded by the hook.
- `acceptInvitation(token)` — token-authorized (the acceptor is not yet a Member,
  so selects no Active Workspace); mints the Membership and marks the Invitation
  `accepted` atomically, requiring the caller's email to match the invitation.
- `revokeInvitation(id)` — tenant-scoped pending→revoked transition. Guarded.

Two design decisions worth flagging:

1. **Owner/admin, not owner-only, for invite/revoke.** Stories 5 and 7 name the
   owner, but the PRD's authorization decision groups owner/admin as the managers
   of Memberships (story 11). Admins can therefore invite and revoke too. Say the
   word if invite/revoke should be owner-only.
2. **SkipTenancy self-authorization.** `bootstrapWorkspace` and `acceptInvitation`
   mint Memberships in a Workspace that is *not* the request's Active Workspace, so
   the ambient Role is the wrong signal. The hook treats any `SkipTenancy` write on
   a guarded table as a trusted server-side path and skips the Role check
   (`SkipTenancy` is server-only, never client-settable). Every other guarded write
   is fail-closed. `TestAcceptWithForeignWorkspaceSelected` guards the case where an
   acceptor already has an unrelated Workspace selected.

Only the mutation half of the "mutation/query hook" is implemented — no read is
role-restricted beyond tenant scoping, so there is nothing for a query hook to
enforce yet.

## Blocked by

- 03-workspaces-membership-tenant-scoping
