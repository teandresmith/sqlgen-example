# 03 — Workspaces, Membership & tenant scoping

Status: done
Blocked by: 02-identity-authentication

Parent: `.scratch/app-server/PRD.md`

## What to build

The tenancy linchpin. An authenticated User can create a Workspace and becomes its `owner` via a Membership. Each request selects its Active Workspace through the `X-Workspace-Id` header; middleware validates the caller's Membership in that Workspace and places User + Active Workspace into the request context. The generated `TenantResolver` reads the Active Workspace from context and feeds sqlgen's structural tenant scoping (`workspace_id`), which must be fail-closed. `users` and `workspaces` remain shared (untenanted) so login and membership validation work before a tenant is established. Every subsequent tenant-scoped ticket depends on this.

## Acceptance criteria

- [x] A User can create a Workspace and is recorded as `owner` via a Membership (stories 4)
- [x] `X-Workspace-Id` selects the Active Workspace per request without re-authenticating (story 8)
- [x] A request whose `X-Workspace-Id` the caller is not a Member of is rejected (story 9)
- [x] A tenant-scoped operation with no valid Active Workspace is rejected — fail-closed (story 10)
- [x] `TenantResolver` drives structural `workspace_id` scoping so every tenant-scoped read/write is automatically filtered to the Active Workspace (story 43)
- [x] `users` and `workspaces` are treated as shared/untenanted
- [x] Integration test proves a User in Workspace A cannot read a row in Workspace B

## Comments

Implemented on branch `HEAD` (issue-03). Design decision (with repo owner): the
generated `createWorkspace` CRUD mutation is kept as a low-level primitive; a new
hand-written `bootstrapWorkspace` mutation (`internal/graph/tenancy.graphqls` +
`tenancy.resolvers.go`) creates the Workspace and the owner Membership atomically
in one `WithTx`, since the first Membership must be minted server-side (the
`X-Workspace-Id` middleware would reject selecting a Workspace the caller is not
yet a Member of). Runtime tenancy lives in `internal/tenancy` (Active-Workspace
context accessors, a fail-closed `TenantResolver`, and the membership-validating
middleware). Tenant isolation is proven for both reads and writes in
`test/tenancy_test.go`.

Follow-up for ticket 04 (authz): `createWorkspace` remains unguarded, so it can
mint an ownerless, unreachable Workspace — an authorization hook should restrict
or remove it in favor of `bootstrapWorkspace`.

## Blocked by

- 02-identity-authentication
