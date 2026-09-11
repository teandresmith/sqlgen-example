# 08 — Task assignment & labels

Status: ready-for-human
Blocked by: 07-projects-tasks-core, 04-authorization-membership-admin

Parent: `.scratch/app-server/PRD.md`

## What to build

The relationship and tagging surface on Tasks. A member can assign multiple Users to a Task (the `Assignees` M2M) and watch a Task to follow updates without being an Assignee (the `Watchers` M2M). A Workspace admin creates and manages Labels for the Workspace (guarded by the authz hook), and any member applies Labels to a Task (the `task_labels` M2M) to categorize and filter work. Uses the relationship names as generated (`Assignees`, `Watchers`).

## Acceptance criteria

- [x] A member can assign multiple Users to a Task via the `Assignees` M2M (story 25) — blessed `assignUserToTask` mutation; `TestAssignMultipleUsersToTask`
- [x] A member can watch a Task via the `Watchers` M2M without being an Assignee (story 26) — blessed `watchTask` (caller-stamped); `TestWatchTaskWithoutBeingAssignee`
- [x] A Workspace admin can create and manage Workspace Labels, enforced by the authz hook (story 28) — `labels` added to `guardedTables`; `TestLabelManagementGuardedByRole`
- [x] A member can apply Labels to a Task via the `task_labels` M2M (story 27) — blessed `applyLabelToTask` (`task_labels` deliberately unguarded); `TestMemberAppliesLabelToTask`
- [x] Integration tests exercise assignment, watching, and labeling through the HTTP seam — `test/task_relationships_test.go` (5 tests, incl. cross-workspace label guard)

## Notes for review

- **Blessed mutations (issue-07 pattern).** The generated M2M create mutations can't link junctions — their GraphQL inputs omit the composite-PK FKs, and `task_labels` has no create mutation at all. So assignment/watch/tagging use hand-written blessed mutations (`internal/graph/task_relationships.{graphqls,resolvers.go}`) that name both sides, resolve the Task (and, for a Label, the Label) tenant-scoped to guard isolation, and thread the per-request HTTP call options. Removals use the generated `deleteTaskAssignee`/`deleteTaskWatcher`/`deleteTaskLabel`.
- **Authz split (story 28 vs 27).** `labels` is guarded (owner/admin manage the catalog); `task_labels` is intentionally *not* guarded (any member tags work). A `LabelFieldOptions` case was added to `selfAuthorized` to keep the switch in sync per the package's documented convention, though no server-side Label bootstrap path exists today.
- **Assignee scope.** `assignUserToTask` permits any existing User (validated via `Users().Get`); tenant isolation is enforced via the Task, not the assignee. Restricting assignees to Workspace members was considered out of scope for story 25.

## Blocked by

- 07-projects-tasks-core
- 04-authorization-membership-admin
