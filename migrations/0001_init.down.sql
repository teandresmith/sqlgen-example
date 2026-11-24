-- 0001_init (down). Auto-skipped by sqlgen's parser (*.down.sql); applied only
-- by golang-migrate on rollback. The view first, then tables in reverse
-- dependency order (CASCADE-free: every dependent is dropped before its
-- referent), then the types.

DROP VIEW IF EXISTS project_stats;

DROP TABLE IF EXISTS attachments;
DROP TABLE IF EXISTS time_entries;
DROP TABLE IF EXISTS task_watchers;
DROP TABLE IF EXISTS task_dependencies;
DROP TABLE IF EXISTS activity;
DROP TABLE IF EXISTS comments;
DROP TABLE IF EXISTS task_labels;
DROP TABLE IF EXISTS task_assignees;
DROP TABLE IF EXISTS tasks;
DROP TABLE IF EXISTS cycles;
DROP TABLE IF EXISTS labels;
DROP TABLE IF EXISTS projects;
DROP TABLE IF EXISTS team_members;
DROP TABLE IF EXISTS teams;
DROP TABLE IF EXISTS invitations;
DROP TABLE IF EXISTS api_tokens;
DROP TABLE IF EXISTS sso_connections;
DROP TABLE IF EXISTS memberships;
DROP TABLE IF EXISTS workspace_settings;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS workspaces;

DROP DOMAIN IF EXISTS email;

DROP TYPE IF EXISTS attachment_entity_type;
DROP TYPE IF EXISTS dependency_type;
DROP TYPE IF EXISTS cycle_status;
DROP TYPE IF EXISTS invitation_status;
DROP TYPE IF EXISTS sso_provider;
DROP TYPE IF EXISTS task_priority;
DROP TYPE IF EXISTS task_status;
DROP TYPE IF EXISTS membership_role;
