-- 0001_init — core schema for the multi-tenant task tracker.
--
-- This file is the SINGLE SOURCE OF TRUTH: golang-migrate applies it to
-- Postgres, and sqlgen parses it (input.paths) to generate Go. The matching
-- 0001_init.down.sql is auto-skipped by sqlgen's parser.
--
-- Tenancy: every tenant-scoped table carries `workspace_id` (sqlgen auto-detects
-- tenanted tables by column presence). `workspaces` and `users` are global
-- (shared) identities and intentionally have no `workspace_id`.

-- ============================================================
-- Enums
-- ============================================================

CREATE TYPE membership_role AS ENUM ('owner', 'admin', 'member', 'guest');
CREATE TYPE task_status AS ENUM ('backlog', 'todo', 'in_progress', 'in_review', 'done', 'canceled');
CREATE TYPE task_priority AS ENUM ('low', 'medium', 'high', 'urgent');

COMMENT ON TYPE membership_role IS 'A user''s permission level within a workspace';
COMMENT ON TYPE task_status IS 'Lifecycle state of a task';
COMMENT ON TYPE task_priority IS 'Relative urgency of a task';

-- ============================================================
-- Shared (non-tenant-scoped) identities
-- ============================================================

-- workspaces: the tenant root. Shared table (no workspace_id on itself).
CREATE TABLE workspaces (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT NOT NULL,
    slug       TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
COMMENT ON TABLE workspaces IS 'Tenant root; the unit of isolation';
COMMENT ON COLUMN workspaces.id IS 'Unique workspace identifier';
COMMENT ON COLUMN workspaces.name IS 'Human-readable workspace name';
COMMENT ON COLUMN workspaces.slug IS 'URL-safe unique short name';
COMMENT ON COLUMN workspaces.created_at IS 'Row creation timestamp';
COMMENT ON COLUMN workspaces.updated_at IS 'Last modification timestamp';

-- users: global identity. Shared table; joins workspaces via memberships.
CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT NOT NULL UNIQUE,
    display_name  TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
COMMENT ON TABLE users IS 'Global user identity, spanning workspaces';
COMMENT ON COLUMN users.id IS 'Unique user identifier';
COMMENT ON COLUMN users.email IS 'Login email address (unique across the system)';
COMMENT ON COLUMN users.display_name IS 'Name shown in the UI';
COMMENT ON COLUMN users.password_hash IS 'Hashed credential for dev login';
COMMENT ON COLUMN users.created_at IS 'Row creation timestamp';
COMMENT ON COLUMN users.updated_at IS 'Last modification timestamp';

-- workspace_settings: O2O with workspaces (workspace_id is UNIQUE FK → one-to-one).
-- Tenant-scoped (has workspace_id).
CREATE TABLE workspace_settings (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL UNIQUE REFERENCES workspaces(id),
    settings     JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
COMMENT ON TABLE workspace_settings IS 'Per-workspace configuration (one row per workspace)';
COMMENT ON COLUMN workspace_settings.id IS 'Unique identifier';
COMMENT ON COLUMN workspace_settings.workspace_id IS 'Owning workspace (unique → one-to-one)';
COMMENT ON COLUMN workspace_settings.settings IS 'Arbitrary configuration payload';
COMMENT ON COLUMN workspace_settings.created_at IS 'Row creation timestamp';
COMMENT ON COLUMN workspace_settings.updated_at IS 'Last modification timestamp';

-- ============================================================
-- Membership: M2M user <-> workspace, composite PK, role enum.
-- Composite PK over exactly the two FKs → sqlgen detects the M2M and it is
-- also the tenant column (workspace_id).
-- ============================================================

CREATE TABLE memberships (
    workspace_id UUID NOT NULL REFERENCES workspaces(id),
    user_id      UUID NOT NULL REFERENCES users(id),
    role         membership_role NOT NULL DEFAULT 'member',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, user_id)
);
COMMENT ON TABLE memberships IS 'Links a user to a workspace with a role (M2M)';
COMMENT ON COLUMN memberships.workspace_id IS 'Workspace side of the membership (tenant column)';
COMMENT ON COLUMN memberships.user_id IS 'User side of the membership';
COMMENT ON COLUMN memberships.role IS 'The user''s permission level in this workspace';
COMMENT ON COLUMN memberships.created_at IS 'When the user joined the workspace';
COMMENT ON COLUMN memberships.updated_at IS 'Last modification timestamp';

-- ============================================================
-- Projects (O2M from workspace; soft-delete via deleted_at)
-- ============================================================

CREATE TABLE projects (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id),
    name         TEXT NOT NULL,
    description  TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at   TIMESTAMPTZ
);
COMMENT ON TABLE projects IS 'A container for tasks, owned by one workspace';
COMMENT ON COLUMN projects.id IS 'Unique project identifier';
COMMENT ON COLUMN projects.workspace_id IS 'Owning workspace (tenant column)';
COMMENT ON COLUMN projects.name IS 'Project display name';
COMMENT ON COLUMN projects.description IS 'Optional long-form description';
COMMENT ON COLUMN projects.created_at IS 'Row creation timestamp';
COMMENT ON COLUMN projects.updated_at IS 'Last modification timestamp';
COMMENT ON COLUMN projects.deleted_at IS 'Soft-delete (archive) timestamp; NULL when active';

-- ============================================================
-- Labels (O2M from workspace; unique name per workspace)
-- ============================================================

CREATE TABLE labels (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id),
    name         TEXT NOT NULL,
    color        TEXT NOT NULL DEFAULT '#888888',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, name)
);
COMMENT ON TABLE labels IS 'A workspace-scoped tag applied to tasks';
COMMENT ON COLUMN labels.id IS 'Unique label identifier';
COMMENT ON COLUMN labels.workspace_id IS 'Owning workspace (tenant column)';
COMMENT ON COLUMN labels.name IS 'Label text (unique per workspace)';
COMMENT ON COLUMN labels.color IS 'Hex display color';
COMMENT ON COLUMN labels.created_at IS 'Row creation timestamp';

-- ============================================================
-- Tasks
--   - O2M from projects
--   - self-referential subtasks (parent_task_id → tasks.id)
--   - reporter_id → users (O2M)
--   - status / priority enums, JSONB custom_fields, soft-delete
-- ============================================================

CREATE TABLE tasks (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id   UUID NOT NULL REFERENCES workspaces(id),
    project_id     UUID NOT NULL REFERENCES projects(id),
    parent_task_id UUID REFERENCES tasks(id),
    reporter_id    UUID NOT NULL REFERENCES users(id),
    title          TEXT NOT NULL,
    description    TEXT,
    status         task_status NOT NULL DEFAULT 'backlog',
    priority       task_priority NOT NULL DEFAULT 'medium',
    custom_fields  JSONB NOT NULL DEFAULT '{}'::jsonb,
    due_at         TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at     TIMESTAMPTZ
);
COMMENT ON TABLE tasks IS 'The unit of work; belongs to a project, may have a parent task';
COMMENT ON COLUMN tasks.id IS 'Unique task identifier';
COMMENT ON COLUMN tasks.workspace_id IS 'Owning workspace (tenant column)';
COMMENT ON COLUMN tasks.project_id IS 'Parent project';
COMMENT ON COLUMN tasks.parent_task_id IS 'Parent task for subtasks; NULL for top-level tasks';
COMMENT ON COLUMN tasks.reporter_id IS 'User who created the task';
COMMENT ON COLUMN tasks.title IS 'Short task summary';
COMMENT ON COLUMN tasks.description IS 'Optional long-form description';
COMMENT ON COLUMN tasks.status IS 'Current lifecycle state';
COMMENT ON COLUMN tasks.priority IS 'Relative urgency';
COMMENT ON COLUMN tasks.custom_fields IS 'Arbitrary user-defined fields';
COMMENT ON COLUMN tasks.due_at IS 'Optional due date';
COMMENT ON COLUMN tasks.created_at IS 'Row creation timestamp';
COMMENT ON COLUMN tasks.updated_at IS 'Last modification timestamp';
COMMENT ON COLUMN tasks.deleted_at IS 'Soft-delete (archive) timestamp; NULL when active';

-- task_assignees: M2M task <-> user (multiple assignees). Composite PK over the
-- two FKs → M2M detection. Not directly tenant-scoped (reached via tenanted task).
CREATE TABLE task_assignees (
    task_id     UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL REFERENCES users(id),
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (task_id, user_id)
);
COMMENT ON TABLE task_assignees IS 'Assigns users to tasks (M2M)';
COMMENT ON COLUMN task_assignees.task_id IS 'Task side of the assignment';
COMMENT ON COLUMN task_assignees.user_id IS 'Assigned user';
COMMENT ON COLUMN task_assignees.assigned_at IS 'When the assignment was made';

-- task_labels: M2M task <-> label. Composite PK over the two FKs → M2M detection.
CREATE TABLE task_labels (
    task_id  UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    label_id UUID NOT NULL REFERENCES labels(id) ON DELETE CASCADE,
    PRIMARY KEY (task_id, label_id)
);
COMMENT ON TABLE task_labels IS 'Applies labels to tasks (M2M)';
COMMENT ON COLUMN task_labels.task_id IS 'Task side of the tagging';
COMMENT ON COLUMN task_labels.label_id IS 'Applied label';

-- ============================================================
-- Comments (O2M from tasks; author_id → users)
-- ============================================================

CREATE TABLE comments (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id),
    task_id      UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    author_id    UUID NOT NULL REFERENCES users(id),
    body         TEXT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
COMMENT ON TABLE comments IS 'A note left by a user on a task';
COMMENT ON COLUMN comments.id IS 'Unique comment identifier';
COMMENT ON COLUMN comments.workspace_id IS 'Owning workspace (tenant column)';
COMMENT ON COLUMN comments.task_id IS 'Task the comment belongs to';
COMMENT ON COLUMN comments.author_id IS 'User who wrote the comment';
COMMENT ON COLUMN comments.body IS 'Comment text';
COMMENT ON COLUMN comments.created_at IS 'Row creation timestamp';
COMMENT ON COLUMN comments.updated_at IS 'Last modification timestamp';

-- ============================================================
-- Activity: append-only feed written by the event subscriber.
-- Tenant-scoped. Events are DISABLED on this table in sqlgen.yml (no events
-- about the event log).
-- ============================================================

CREATE TABLE activity (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id),
    actor_id     UUID REFERENCES users(id),
    entity_table TEXT NOT NULL,
    entity_id    UUID NOT NULL,
    action       TEXT NOT NULL,
    payload      JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
COMMENT ON TABLE activity IS 'Append-only activity feed derived from mutation events';
COMMENT ON COLUMN activity.id IS 'Unique activity entry identifier';
COMMENT ON COLUMN activity.workspace_id IS 'Owning workspace (tenant column)';
COMMENT ON COLUMN activity.actor_id IS 'User who triggered the activity; NULL for system actions';
COMMENT ON COLUMN activity.entity_table IS 'Table name of the affected entity';
COMMENT ON COLUMN activity.entity_id IS 'Primary key of the affected entity';
COMMENT ON COLUMN activity.action IS 'Mutation action (create/update/delete/upsert)';
COMMENT ON COLUMN activity.payload IS 'Snapshot of the change';
COMMENT ON COLUMN activity.created_at IS 'When the activity occurred';

-- Helpful indexes for common access paths.
CREATE INDEX idx_projects_workspace     ON projects (workspace_id);
CREATE INDEX idx_tasks_project          ON tasks (project_id);
CREATE INDEX idx_tasks_workspace_status ON tasks (workspace_id, status);
CREATE INDEX idx_tasks_parent           ON tasks (parent_task_id);
CREATE INDEX idx_comments_task          ON comments (task_id);
CREATE INDEX idx_activity_workspace     ON activity (workspace_id, created_at DESC);

-- Uniqueness that respects soft-delete: a workspace slug and label name stay
-- unique; tasks/projects use soft-delete but have no natural unique key to guard.
