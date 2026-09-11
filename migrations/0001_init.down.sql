-- 0001_init (down). Auto-skipped by sqlgen's parser (*.down.sql); applied only
-- by golang-migrate on rollback. Drop in reverse dependency order.

DROP TABLE IF EXISTS activity;
DROP TABLE IF EXISTS comments;
DROP TABLE IF EXISTS task_labels;
DROP TABLE IF EXISTS task_assignees;
DROP TABLE IF EXISTS tasks;
DROP TABLE IF EXISTS labels;
DROP TABLE IF EXISTS projects;
DROP TABLE IF EXISTS memberships;
DROP TABLE IF EXISTS workspace_settings;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS workspaces;

DROP TYPE IF EXISTS task_priority;
DROP TYPE IF EXISTS task_status;
DROP TYPE IF EXISTS membership_role;
