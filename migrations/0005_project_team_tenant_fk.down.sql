-- Reverse 0005 in the opposite order: restore the single-column FK on
-- projects.team_id, then drop the composite FK and the unique key it targeted.
ALTER TABLE projects DROP CONSTRAINT IF EXISTS projects_team_same_ws;

ALTER TABLE projects ADD CONSTRAINT projects_team_id_fkey
    FOREIGN KEY (team_id) REFERENCES teams (id);

ALTER TABLE teams DROP CONSTRAINT IF EXISTS teams_ws_id_key;
