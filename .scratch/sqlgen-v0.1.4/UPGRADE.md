# Upgrade sqlgen v0.1.2 → v0.1.4

Branch `chore/sqlgen-v0.1.4` (off `chore/sqlgen-v0.1.2`). Not pushed.

## Re-check of the v0.1.2 findings

| # | Finding | v0.1.4 |
|---|---|---|
| 1 | prose `@pk` warns "unrecognized annotation directive" | fixed, warning gone |
| 2 | CHANGELOG omits the reseed/relaxed inputs | not fixed: 0.1.3/0.1.4 entries are one line each |
| 3 | configuration.md lacks relationship config | fixed (Relationships section, naming rules, `server_scaffold`) |
| 4 | unused server.go scaffold, limits 6/1000 vs 12/200 | fixed: `api.graphql.server_scaffold: false`, file deleted |
| 5 | no rename of a detected edge | fixed: a declared edge over the same key takes the place of the detected one |
| 6 | `ErrAlreadyRelated` doc | fixed, doc now covers the pinned-member case |
| 7 | nested-create comment wrong for composite keys | fixed |
| 8 | GraphQL filters have no `id` | fixed: `id: IDComparator` on every filter |

## Config changes
- Removed the three `exclude_relationships` entries: regenerating without them
  changes nothing (only `generated_at`). The three declarations stay: removing
  them renames Subtasks/Assignees/Watchers back to the detected names.
- `server_scaffold: false`; `internal/graph/server.go` removed.
- Took the `WorkspaceSetting.workspaces` -> `workspace` rename (approved).

## New behaviour that needs a decision
1. Belongs-to detection (0.1.3): every FK now also gives a to-one field, so
   Task gains `cycle`, `parentTask`, `project`, `reporter`, `workspace`, and so
   on, across the schema. Additive, but it widens the public GraphQL and the
   relationship filters (`TaskFilter.reporter` etc.).
2. It would rename `WorkspaceSetting.workspaces` to `workspace` (Go field and
   GraphQL). The brief forbids public renames without approval, so the old name
   is pinned with a declared o2o. Delete that entry to take the new name.

## Gates
generate 0, `make diff` clean, build, vet, `go test ./...` all ok (including the
18 composite-edge subtests), manifest-validate ok (0.2.0).

## Task 2.3: compensating code review (v0.1.4)

Nothing was removed. Each item is a candidate for you to decide.

| Code | What it guards | Can sqlgen v0.1.4 replace it? | Verdict |
|---|---|---|---|
| `AddUserToTeam` / `RemoveUserFromTeam` team lookup (`team_management.resolvers.go`) | `team_members` has no `workspace_id`, so a raw create/delete is unscoped | Only with `workspace_id` + composite FKs on `team_members` (the tenant-junction shape in PRD 5.3). Not today | keep |
| `AddUserToTeam` membership check | user must hold a Membership in the active workspace; `users` is global, so no FK can express it | No | keep |
| `AssignUserToTask` / `WatchTask` task lookup (`task_relationships.resolvers.go`) | same junction gap (`task_assignees`, `task_watchers`) | Same as above. A nested `updateTaskWithRelated(assignees: connect)` would scope the parent, but changes the public mutation and I haven't verified it passes the authz hook | candidate after a schema change |
| `AssignUserToTask` user lookup | friendlier error than a raw FK violation; does not check membership (open issue from the v0.1.2 report) | No | keep; decide on the membership gap |
| Projector `tenantOfTask` / `tenantOfTeam` (`internal/events/projector.go`) | derives the activity workspace for junction events, which carry no tenant | Would go away with a `workspace_id` on the junctions | candidate after a schema change |
| Migrations 0004/0005 composite FKs and the raw-update backstop tests | `updateTask.cycleID` raw writes still unscoped by sqlgen | No | keep |

v0.1.4's belongs-to edges add no new removals: the app has no hand-written parent lookups beyond the above.
Single follow-up that unlocks three candidates: add `workspace_id` and composite FKs to
`task_assignees`, `task_watchers`, `team_members` (and optionally `task_labels`/`task_dependencies`).

## Trial: tenant junctions (branch `feat/tenant-junctions`)

Migration 0006 adds `workspace_id` + composite FKs to `task_assignees`,
`task_watchers`, `team_members` (public API change: only an added `workspaceID`
field/filter/sort on the three link types; no renames).

What worked: sqlgen scopes and stamps the tenant on the three tables; the DB
refuses a cross-workspace link for every writer. `test/tenant_junctions_test.go`
runs 12 subtests (load, cross-workspace create via client/skip/raw SQL,
delete) as A, as B and with SkipTenancy. All pass.

Removed: the Team lookup in `RemoveUserFromTeam` (the delete is tenant-scoped).
Kept as error-message UX only, comments updated: the Team/Task lookups in
`AddUserToTeam`, `AssignUserToTask`, `WatchTask`. Not removed: projector
`tenantOfTask`/`tenantOfTeam` (task_labels and task_dependencies still need
them; the three converted tables already carry a tenant stamp on the event).

Needed: junction detection requires a UNIQUE/PK containing both FKs' columns
plus workspace_id (PRD 5.3 rule 4), so each table got `UNIQUE (workspace_id,
task_id, user_id)`.

### Finding 9 (sqlgen limitation, medium): a declared edge can't take over a composite-key M2M
`junction_local_fk: [workspace_id, task_id]` is rejected: "has 2 columns, but it
references the primary key of "tasks", which has 1 column". A declared key maps
onto the target's primary key only (DETECTED_EDGE_RENAME.md non-goal: no
non-PK UNIQUE targets). So `Assignees`/`Watchers` can't take the detected edge's
place, and the exclusions are needed again; the declared edges then have a
manifest `local_fk` with no workspace_id and no constraint. Repro: any
tenant junction `j(workspace_id, a_id, b_id)` with `(workspace_id,a_id) → a
(workspace_id,id)` and a rename of the detected M2M. Classification: sqlgen
limitation / feature request (declared keys should accept a UNIQUE target).
Not blocking: names are preserved with exclusions.

### Still to decide
- Same change for `task_labels` and `task_dependencies` would let the projector
  tenant lookups go.
- The API masks on the three tables could be relaxed (raw writes are now
  tenant-safe) but that changes the public API.

## Migrations folded into one file
`migrations/0001_init.{up,down}.sql` now holds the whole schema (previously
0001-0006). The composite FKs and the `*_ws_key` UNIQUEs are inline in the
tables they belong to, with no add-then-drop of the plain FKs. Verified: a
`pg_dump -s` of the old chain and the new file match (apart from nothing), the
generated code is byte-identical, up/down/up round-trips, and all tests pass.
