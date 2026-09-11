# 10 — Cycles & scheduling

Status: ready-for-human
Blocked by: 07-projects-tasks-core

Parent: `.scratch/app-server/PRD.md`

## What to build

Iteration planning. A Workspace admin creates Cycles with a date range and status so iterations are defined, and a Workspace member schedules a Task into a Cycle so it is planned into an iteration.

## Acceptance criteria

- [x] A Workspace admin can create a Cycle with a date range and status (story 32)
- [x] A Workspace member can schedule a Task into a Cycle (story 31)
- [x] Integration tests exercise cycle creation and task scheduling through the HTTP seam

## Blocked by

- 07-projects-tasks-core

## Notes for review

Two halves, split along the same Role boundary as Labels (issue 08):

- **Defining a Cycle (story 32)** uses the **generated `createCycle`** mutation,
  now guarded owner/admin-only by adding `cycles` to `guardedTables` in
  `internal/authz/authz.go` (plus the matching `CycleFieldOptions` case in
  `selfAuthorized`, kept in sync with the guarded set). No bespoke create
  mutation — this mirrors how Label management reuses the generated `createLabel`
  behind the authz hook. `createCycle`'s input `workspaceID` is validated against
  the Active Workspace by tenant scoping (cross-tenant create rejected, as with
  Projects/Labels).
- **Scheduling a Task into a Cycle (story 31)** is a blessed mutation,
  `scheduleTaskIntoCycle(taskID, cycleID)` (`internal/graph/cycle_scheduling.*`).
  It resolves both the Task and the Cycle tenant-scoped, then writes only
  `cycle_id`. Because `tasks.cycle_id` references `cycles(id)` with no Workspace
  constraint, this closes the cross-tenant hole a raw `updateTask` would leave —
  the guard the repo applies to every FK-less / cross-referencable link
  (addTaskDependency, applyLabelToTask). Scheduling is a write on `tasks`
  (unguarded), so any member may plan work; only the Cycle catalog is Role-gated.

Tests (`test/cycles_test.go`, through the HTTP seam): admin cycle creation with a
date range + status, the owner/admin Role guard (member rejected, owner control
succeeds), a member scheduling a Task into a Cycle with read-back via both
`task.cycleID` and `cycle.tasks`, a cross-Workspace guard on the blessed mutation,
and a regression proving the raw `updateTask` path is rejected at the DB (see the
FK hardening below).

Only the authz guard, the blessed mutation, and tests were hand-written; then
`make generate`. `go build ./... && go vet ./... && go test ./...` all green.

## Cross-tenant `cycleID` — closed at the database (migration 0004)

The generated `updateTask` accepts a `cycleID` field, and `updateTask` is
tenant-scoped only on the Task — nothing validated that the supplied `cycleID`
belonged to the Task's Workspace, so a raw `updateTask` could point a Task at a
Cycle in *another* Workspace (the blessed `scheduleTaskIntoCycle` guarded this,
but the raw path did not). sqlgen's `api.operations` mask is table/operation-
granular, not field-granular, so `updateTask` can't be narrowed to drop just
`cycleID`.

Rather than an app-layer check, the fix is **referential**, since the invariant
is a relationship both tables can express through the shared tenant column
(`migrations/0004_task_cycle_tenant_fk.up.sql`):

```sql
ALTER TABLE cycles ADD CONSTRAINT cycles_ws_id_key UNIQUE (workspace_id, id);
ALTER TABLE tasks  DROP CONSTRAINT tasks_cycle_id_fkey;          -- the single-column FK
ALTER TABLE tasks  ADD CONSTRAINT tasks_cycle_same_ws
    FOREIGN KEY (workspace_id, cycle_id) REFERENCES cycles (workspace_id, id);
```

Because `cycles.id` is globally unique, `(workspace_id, cycle_id)` can only match
a `(workspace_id, id)` row when the two Workspaces agree — Postgres enforces it on
every write, so it cannot be bypassed by `SkipTenancy`, raw SQL, a resolver bug,
or a mutation added later. `MATCH SIMPLE` skips the check when `cycle_id IS NULL`,
so unscheduled Tasks are exempt. The `blessed` mutation stays as the nice-error
front door; the FK is the backstop.

Notes: the old single-column FK is dropped (the composite one subsumes it, and
leaving both would give sqlgen two `tasks→cycles` FKs). Regeneration was verified
to leave `Task.cycleID` / `Cycle.tasks` unchanged — the composite FK only added
two Go client helpers on Cycle (a `FindByWorkspaceIdAndId` finder + an upsert
target) from the new unique key; **no** GraphQL surface or relationship change, so
sqlgen handles a tenant-scoping composite FK gracefully (no upstream change
needed). The `ADD CONSTRAINT ... FOREIGN KEY` validates existing rows under a
`SHARE ROW EXCLUSIVE` lock — fine here; on a large hot table use `NOT VALID` +
`VALIDATE CONSTRAINT`.
