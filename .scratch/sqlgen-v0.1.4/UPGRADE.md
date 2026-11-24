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
