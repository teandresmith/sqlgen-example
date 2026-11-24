-- 0006_tenant_junctions — give the three guarded link tables a tenant column
-- and make each link's workspace agree with its parent, at the database.
--
-- task_assignees, task_watchers and team_members carried no workspace_id, so
-- nothing but app code stopped a raw write from linking a row to a Task/Team in
-- another Workspace, and the projector had to look the tenant up. A composite
-- FK (workspace_id, task_id) -> tasks (workspace_id, id) makes the link's
-- workspace the parent's workspace for every writer, and sqlgen then stamps and
-- scopes workspace_id on these tables like any tenanted table.

ALTER TABLE tasks ADD CONSTRAINT tasks_ws_id_key UNIQUE (workspace_id, id);
-- teams_ws_id_key already exists (0005).

ALTER TABLE task_assignees ADD COLUMN workspace_id UUID;
ALTER TABLE task_watchers  ADD COLUMN workspace_id UUID;
ALTER TABLE team_members   ADD COLUMN workspace_id UUID;

UPDATE task_assignees j SET workspace_id = t.workspace_id FROM tasks t WHERE t.id = j.task_id;
UPDATE task_watchers  j SET workspace_id = t.workspace_id FROM tasks t WHERE t.id = j.task_id;
UPDATE team_members   j SET workspace_id = t.workspace_id FROM teams t WHERE t.id = j.team_id;

ALTER TABLE task_assignees ALTER COLUMN workspace_id SET NOT NULL;
ALTER TABLE task_watchers  ALTER COLUMN workspace_id SET NOT NULL;
ALTER TABLE team_members   ALTER COLUMN workspace_id SET NOT NULL;

-- The composite FKs subsume the single-column ones (see 0004).
ALTER TABLE task_assignees DROP CONSTRAINT task_assignees_task_id_fkey;
ALTER TABLE task_watchers  DROP CONSTRAINT task_watchers_task_id_fkey;
ALTER TABLE team_members   DROP CONSTRAINT team_members_team_id_fkey;

-- ON DELETE CASCADE as before: deleting a task/team removes its links.
ALTER TABLE task_assignees ADD CONSTRAINT task_assignees_task_same_ws
    FOREIGN KEY (workspace_id, task_id) REFERENCES tasks (workspace_id, id) ON DELETE CASCADE;
ALTER TABLE task_watchers ADD CONSTRAINT task_watchers_task_same_ws
    FOREIGN KEY (workspace_id, task_id) REFERENCES tasks (workspace_id, id) ON DELETE CASCADE;
ALTER TABLE team_members ADD CONSTRAINT team_members_team_same_ws
    FOREIGN KEY (workspace_id, team_id) REFERENCES teams (workspace_id, id) ON DELETE CASCADE;

ALTER TABLE task_assignees ADD CONSTRAINT task_assignees_workspace_fkey FOREIGN KEY (workspace_id) REFERENCES workspaces (id);
ALTER TABLE task_watchers  ADD CONSTRAINT task_watchers_workspace_fkey  FOREIGN KEY (workspace_id) REFERENCES workspaces (id);
ALTER TABLE team_members   ADD CONSTRAINT team_members_workspace_fkey   FOREIGN KEY (workspace_id) REFERENCES workspaces (id);

-- sqlgen detects a junction only when a PRIMARY KEY or UNIQUE contains both
-- foreign keys' columns, workspace_id included (PRD 5.3 rule 4). The primary
-- keys stay (task_id, user_id) / (team_id, user_id); these UNIQUEs are implied
-- by them and exist for detection.
ALTER TABLE task_assignees ADD CONSTRAINT task_assignees_ws_key UNIQUE (workspace_id, task_id, user_id);
ALTER TABLE task_watchers  ADD CONSTRAINT task_watchers_ws_key  UNIQUE (workspace_id, task_id, user_id);
ALTER TABLE team_members   ADD CONSTRAINT team_members_ws_key   UNIQUE (workspace_id, team_id, user_id);

COMMENT ON COLUMN task_assignees.workspace_id IS 'Owning workspace (tenant column); equals the task''s';
COMMENT ON COLUMN task_watchers.workspace_id  IS 'Owning workspace (tenant column); equals the task''s';
COMMENT ON COLUMN team_members.workspace_id   IS 'Owning workspace (tenant column); equals the team''s';
