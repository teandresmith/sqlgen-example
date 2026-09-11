# 12 — Teams

Status: ready-for-human
Blocked by: 03-workspaces-membership-tenant-scoping, 04-authorization-membership-admin

Parent: `.scratch/app-server/PRD.md`

## What to build

Team structure within a Workspace. A Workspace admin creates Teams to group Users and adds/removes Users on a Team so membership reflects reality (guarded by the authz hook); a Workspace member assigns a Project to a Team so ownership is clear.

## Acceptance criteria

- [x] A Workspace admin can create Teams within a Workspace (story 16)
- [x] A Workspace admin can add and remove Users on a Team (story 17)
- [x] A Workspace member can assign a Project to a Team (story 18)
- [x] Team-management operations are enforced by the authz hook
- [x] Integration tests exercise team creation, membership, and project assignment through the HTTP seam

## Blocked by

- 03-workspaces-membership-tenant-scoping
- 04-authorization-membership-admin

## Notes for review

Three stories along the now-familiar Role boundary (mirrors Cycles, issue 10):

- **Creating a Team (story 16)** uses the **generated `createTeam`**, guarded
  owner/admin-only by adding `teams` to `guardedTables` in `internal/authz/authz.go`
  (plus the matching `TeamFieldOptions` case in `selfAuthorized`, kept in sync
  with the guarded set). No bespoke create mutation — same as Cycle/Label
  management. `createTeam`'s input `workspaceID` is validated against the Active
  Workspace by tenant scoping.
- **Managing membership (story 17)** is two blessed mutations,
  `addUserToTeam(teamID, userID)` and `removeUserFromTeam(teamID, userID)`
  (`internal/graph/team_management.*`). `team_members` carries no `workspace_id`
  and has no generated create, so the generated write surface is subtracted
  entirely in `sqlgen.yml` (`create/create_many/upsert` as before, now also
  `update/update_many/hard_delete`) and these blessed mutations are the only write
  path. Each resolves the Team tenant-scoped first (so a Team in another Workspace
  is invisible — closing the cross-tenant hole a raw composite-PK delete would
  leave), and both write to `team_members`, a **guarded** table, so the authz hook
  makes them owner/admin-only. `addUserToTeam` also requires the User to hold a
  Membership in the Active Workspace (checked via the tenant-scoped Memberships
  client), since a Team groups Users *within* a Workspace (CONTEXT.md) — so Team
  membership cannot diverge from the Workspace roster. `removeUserFromTeam` is
  idempotent.
- **Assigning a Project to a Team (story 18)** is the blessed
  `assignProjectToTeam(projectID, teamID)`, the exact analog of
  `scheduleTaskIntoCycle`. It resolves both the Project and the Team tenant-scoped,
  then writes only `team_id`. Assigning is a write on `projects` (unguarded), so
  any member may set ownership; only the Team catalog is Role-gated.

Tests (`test/teams_test.go`, through the HTTP seam): admin team creation with
workspace auto-scoping, the owner/admin guard on creation and on membership
management (member rejected, owner control succeeds), add/remove round-trip via
`team.users`, a member assigning a Project with read-back via `project.teamID` and
`team.projects`, a cross-Workspace guard on the blessed assign, and a regression
proving the raw `updateProject` path is rejected at the DB (see below).

Only the authz guard, the `sqlgen.yml` mask, the blessed mutations, the migration,
and tests were hand-written; then `make generate`. `go build ./... && go vet ./...
&& go test ./...` all green.

## Cross-tenant `teamID` — closed at the database (migration 0005)

Same shape as migration 0004 (Task↔Cycle): the generated `updateProject` accepts a
`teamID` and is tenant-scoped only on the Project, so a raw `updateProject` could
point a Project at a Team in another Workspace (the blessed mutation guards this;
the raw path did not). The `api.operations` mask is table/operation-granular, not
field-granular, so `updateProject` can't be narrowed to drop just `teamID`.

The fix is referential (`migrations/0005_project_team_tenant_fk.up.sql`):

```sql
ALTER TABLE teams    ADD CONSTRAINT teams_ws_id_key UNIQUE (workspace_id, id);
ALTER TABLE projects DROP CONSTRAINT projects_team_id_fkey;          -- the single-column FK
ALTER TABLE projects ADD CONSTRAINT projects_team_same_ws
    FOREIGN KEY (workspace_id, team_id) REFERENCES teams (workspace_id, id);
```

Because `teams.id` is globally unique, `(workspace_id, team_id)` matches only when
the two Workspaces agree — Postgres enforces it on every write. `MATCH SIMPLE`
skips the check when `team_id IS NULL`, so unassigned Projects are exempt.
Regeneration left the GraphQL surface unchanged — the new unique key only added a
`TeamConflictWorkspaceIDID` conflict target to the Team Go client, no relationship
or schema change (same graceful handling issue 10 observed for cycles).
