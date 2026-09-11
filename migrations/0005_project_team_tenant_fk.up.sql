-- 0005_project_team_tenant_fk — enforce that a Project can only be assigned to a
-- Team in its OWN Workspace, at the database, for every writer.
--
-- projects.team_id referenced teams(id) with no Workspace constraint, so any path
-- that sets team_id — the generated updateProject, a future mutation, raw SQL —
-- could point a Project at a Team in another Workspace. The blessed
-- assignProjectToTeam (issue 12) guards this at the app layer, but nothing stops
-- the raw updateProject. A composite foreign key makes the invariant referential:
-- (workspace_id, team_id) must name a real (workspace_id, id) row in teams, and
-- since teams.id is globally unique, that is satisfiable only when the two
-- Workspaces match. Enforced by the storage engine, so it cannot be bypassed by
-- SkipTenancy, a resolver bug, or a mutation added later. Mirrors migration 0004
-- (tasks.cycle_id → cycles).
--
-- MATCH SIMPLE (the default) skips the check whenever team_id IS NULL, so a
-- Project with no owning Team is exempt with no extra predicate.
--
-- LOCK NOTE: ADD CONSTRAINT ... FOREIGN KEY validates existing rows under a
-- SHARE ROW EXCLUSIVE lock on both tables. Fine for this size; on a large, hot
-- table you would ADD ... NOT VALID and then VALIDATE CONSTRAINT in a second step
-- to weaken the lock.

-- A composite FK must target a UNIQUE (or PK) on exactly its referenced columns.
-- workspace_id-leading so the index also serves tenant-scoped team lookups
-- (teams filtered by workspace_id), not just the FK.
ALTER TABLE teams ADD CONSTRAINT teams_ws_id_key UNIQUE (workspace_id, id);

-- Drop the single-column FK the composite one subsumes: any state satisfying
-- (workspace_id, team_id) -> (workspace_id, id) also satisfies team_id -> id.
-- Leaving both would double-check every write and hand sqlgen two projects->teams
-- FKs to reconcile. The reverse-lookup index idx_projects_team is a separate
-- object and is unaffected.
ALTER TABLE projects DROP CONSTRAINT projects_team_id_fkey;

ALTER TABLE projects ADD CONSTRAINT projects_team_same_ws
    FOREIGN KEY (workspace_id, team_id) REFERENCES teams (workspace_id, id);
