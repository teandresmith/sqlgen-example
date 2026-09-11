-- 0004_task_cycle_tenant_fk — enforce that a Task can only be scheduled into a
-- Cycle in its OWN Workspace, at the database, for every writer.
--
-- tasks.cycle_id referenced cycles(id) with no Workspace constraint, so any path
-- that sets cycle_id — the generated updateTask, a future mutation, raw SQL —
-- could point a Task at a Cycle in another Workspace. The blessed
-- scheduleTaskIntoCycle (issue 10) guarded this at the app layer, but nothing
-- stopped the raw updateTask. A composite foreign key makes the invariant
-- referential: (workspace_id, cycle_id) must name a real (workspace_id, id) row
-- in cycles, and since cycles.id is globally unique, that is satisfiable only
-- when the two Workspaces match. Enforced by the storage engine, so it cannot be
-- bypassed by SkipTenancy, a resolver bug, or a mutation added later.
--
-- MATCH SIMPLE (the default) skips the check whenever cycle_id IS NULL, so an
-- unscheduled Task is exempt with no extra predicate.
--
-- LOCK NOTE: ADD CONSTRAINT ... FOREIGN KEY validates existing rows under a
-- SHARE ROW EXCLUSIVE lock on both tables. Fine for this size; on a large, hot
-- table you would ADD ... NOT VALID and then VALIDATE CONSTRAINT in a second step
-- to weaken the lock.

-- A composite FK must target a UNIQUE (or PK) on exactly its referenced columns.
-- workspace_id-leading so the index also serves tenant-scoped cycle lookups
-- (cycles filtered by workspace_id), not just the FK.
ALTER TABLE cycles ADD CONSTRAINT cycles_ws_id_key UNIQUE (workspace_id, id);

-- Drop the single-column FK the composite one subsumes: any state satisfying
-- (workspace_id, cycle_id) -> (workspace_id, id) also satisfies cycle_id -> id.
-- Leaving both would double-check every write and hand sqlgen two tasks->cycles
-- FKs to reconcile. The reverse-lookup index idx_tasks_cycle is a separate
-- object and is unaffected.
ALTER TABLE tasks DROP CONSTRAINT tasks_cycle_id_fkey;

ALTER TABLE tasks ADD CONSTRAINT tasks_cycle_same_ws
    FOREIGN KEY (workspace_id, cycle_id) REFERENCES cycles (workspace_id, id);
