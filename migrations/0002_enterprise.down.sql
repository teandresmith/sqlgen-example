-- 0002_enterprise (down). Auto-skipped by sqlgen; applied by golang-migrate on
-- rollback. Reverse the ALTERs, drop tables in reverse dependency order, then
-- drop the new enums and domain.

ALTER TABLE tasks DROP COLUMN IF EXISTS cycle_id;
ALTER TABLE projects DROP COLUMN IF EXISTS team_id;

DROP TABLE IF EXISTS attachments;
DROP TABLE IF EXISTS time_entries;
DROP TABLE IF EXISTS task_watchers;
DROP TABLE IF EXISTS task_dependencies;
DROP TABLE IF EXISTS cycles;
DROP TABLE IF EXISTS invitations;
DROP TABLE IF EXISTS api_tokens;
DROP TABLE IF EXISTS sso_connections;
DROP TABLE IF EXISTS team_members;
DROP TABLE IF EXISTS teams;

DROP DOMAIN IF EXISTS email;

DROP TYPE IF EXISTS attachment_entity_type;
DROP TYPE IF EXISTS dependency_type;
DROP TYPE IF EXISTS cycle_status;
DROP TYPE IF EXISTS invitation_status;
DROP TYPE IF EXISTS sso_provider;
