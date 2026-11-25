-- 0001_init — the whole schema of the multi-tenant task tracker, in one file:
-- core tables, the enterprise tier (teams, cycles, SSO, API tokens, invitations,
-- dependencies, watchers, time entries, attachments), the tenant-integrity keys
-- and the project_stats view.
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
    deleted_at     TIMESTAMPTZ,
    -- Target of the composite (workspace_id, task_id) FKs on the link tables.
    -- workspace_id-leading, so it also serves tenant-scoped lookups.
    CONSTRAINT tasks_ws_id_key UNIQUE (workspace_id, id)
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

-- Tenant link tables (task_assignees, task_watchers, team_members): each carries
-- workspace_id and a composite FK (workspace_id, parent_id) → parent
-- (workspace_id, id), so a link's workspace is its parent's, for every writer —
-- the client, SkipTenancy, raw SQL. The composite FK subsumes a plain
-- parent_id FK, so there is no second one for sqlgen to reconcile. sqlgen
-- detects such a table as a junction only when a PRIMARY KEY or UNIQUE holds
-- both FKs' columns plus workspace_id, hence the *_ws_key UNIQUEs (implied by
-- the primary key; they exist for detection).

-- task_assignees: M2M task <-> user (multiple assignees).
CREATE TABLE task_assignees (
    task_id      UUID NOT NULL,
    user_id      UUID NOT NULL REFERENCES users(id),
    assigned_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    workspace_id UUID NOT NULL,
    PRIMARY KEY (task_id, user_id),
    CONSTRAINT task_assignees_workspace_fkey FOREIGN KEY (workspace_id) REFERENCES workspaces(id),
    CONSTRAINT task_assignees_ws_key UNIQUE (workspace_id, task_id, user_id),
    CONSTRAINT task_assignees_task_same_ws
        FOREIGN KEY (workspace_id, task_id) REFERENCES tasks (workspace_id, id) ON DELETE CASCADE
);
COMMENT ON COLUMN task_assignees.workspace_id IS 'Owning workspace (tenant column); equals the task''s';
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

-- ============================================================
-- New enums & domain
-- ============================================================

CREATE TYPE sso_provider AS ENUM ('saml', 'oidc', 'google', 'okta');
CREATE TYPE invitation_status AS ENUM ('pending', 'accepted', 'revoked', 'expired');
CREATE TYPE cycle_status AS ENUM ('upcoming', 'active', 'completed');
CREATE TYPE dependency_type AS ENUM ('blocks', 'relates_to', 'duplicates');
CREATE TYPE attachment_entity_type AS ENUM ('task', 'comment');

COMMENT ON TYPE sso_provider IS 'Identity provider protocol/vendor for SSO';
COMMENT ON TYPE invitation_status IS 'Lifecycle state of a workspace invitation';
COMMENT ON TYPE cycle_status IS 'Lifecycle state of a planning cycle';
COMMENT ON TYPE dependency_type IS 'Nature of a task-to-task dependency';
COMMENT ON TYPE attachment_entity_type IS 'Which entity kind an attachment is bound to';

-- Domain type: a validated email address (generates a Go type alias in sqlgen).
CREATE DOMAIN email AS TEXT CHECK (VALUE ~ '^[^@[:space:]]+@[^@[:space:]]+\.[^@[:space:]]+$');
COMMENT ON DOMAIN email IS 'A syntactically-valid email address';

-- ============================================================
-- Pack 1 — Teams & access structure
-- ============================================================

CREATE TABLE teams (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id),
    name         TEXT NOT NULL,
    slug         TEXT NOT NULL,
    description  TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, slug),
    -- Target of the composite FKs from projects, team_members.
    CONSTRAINT teams_ws_id_key UNIQUE (workspace_id, id)
);
COMMENT ON TABLE teams IS 'A group of users within a workspace that owns projects';
COMMENT ON COLUMN teams.id IS 'Unique team identifier';
COMMENT ON COLUMN teams.workspace_id IS 'Owning workspace (tenant column)';
COMMENT ON COLUMN teams.name IS 'Team display name';
COMMENT ON COLUMN teams.slug IS 'URL-safe short name (unique per workspace)';
COMMENT ON COLUMN teams.description IS 'Optional description';
COMMENT ON COLUMN teams.created_at IS 'Row creation timestamp';
COMMENT ON COLUMN teams.updated_at IS 'Last modification timestamp';

-- team_members: M2M team <-> user (tenant link table, see 0001's note above).
CREATE TABLE team_members (
    team_id      UUID NOT NULL,
    user_id      UUID NOT NULL REFERENCES users(id),
    added_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    workspace_id UUID NOT NULL,
    PRIMARY KEY (team_id, user_id),
    CONSTRAINT team_members_workspace_fkey FOREIGN KEY (workspace_id) REFERENCES workspaces(id),
    CONSTRAINT team_members_ws_key UNIQUE (workspace_id, team_id, user_id),
    CONSTRAINT team_members_team_same_ws
        FOREIGN KEY (workspace_id, team_id) REFERENCES teams (workspace_id, id) ON DELETE CASCADE
);
COMMENT ON COLUMN team_members.workspace_id IS 'Owning workspace (tenant column); equals the team''s';
COMMENT ON TABLE team_members IS 'Membership of users in teams (M2M)';
COMMENT ON COLUMN team_members.team_id IS 'Team side of the membership';
COMMENT ON COLUMN team_members.user_id IS 'User side of the membership';
COMMENT ON COLUMN team_members.added_at IS 'When the user joined the team';

-- ============================================================
-- Pack 2 — Enterprise auth & security
-- ============================================================

CREATE TABLE sso_connections (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id),
    provider     sso_provider NOT NULL,
    config       JSONB NOT NULL DEFAULT '{}'::jsonb,
    enabled      BOOLEAN NOT NULL DEFAULT false,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, provider)
);
COMMENT ON TABLE sso_connections IS 'Per-workspace single sign-on configuration';
COMMENT ON COLUMN sso_connections.id IS 'Unique identifier';
COMMENT ON COLUMN sso_connections.workspace_id IS 'Owning workspace (tenant column)';
COMMENT ON COLUMN sso_connections.provider IS 'Identity provider protocol/vendor';
COMMENT ON COLUMN sso_connections.config IS 'Provider-specific configuration (metadata URL, client IDs, etc.)';
COMMENT ON COLUMN sso_connections.enabled IS 'Whether this connection is active';
COMMENT ON COLUMN sso_connections.created_at IS 'Row creation timestamp';
COMMENT ON COLUMN sso_connections.updated_at IS 'Last modification timestamp';

-- api_tokens: personal (user_id set) or service (user_id NULL) tokens.
-- scopes is a TEXT[] → exercises the array/Slice comparator.
CREATE TABLE api_tokens (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id),
    user_id      UUID REFERENCES users(id),
    name         TEXT NOT NULL,
    token_hash   TEXT NOT NULL UNIQUE,
    scopes       TEXT[] NOT NULL DEFAULT '{}',
    last_used_at TIMESTAMPTZ,
    expires_at   TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
COMMENT ON TABLE api_tokens IS 'API access tokens, personal or service-account scoped';
COMMENT ON COLUMN api_tokens.id IS 'Unique identifier';
COMMENT ON COLUMN api_tokens.workspace_id IS 'Owning workspace (tenant column)';
COMMENT ON COLUMN api_tokens.user_id IS 'Owning user; NULL for service-account tokens';
COMMENT ON COLUMN api_tokens.name IS 'Human-readable token label';
COMMENT ON COLUMN api_tokens.token_hash IS 'Hashed token secret (never stored in plaintext)';
COMMENT ON COLUMN api_tokens.scopes IS 'Granted permission scopes';
COMMENT ON COLUMN api_tokens.last_used_at IS 'Timestamp of the most recent use';
COMMENT ON COLUMN api_tokens.expires_at IS 'Optional expiry; NULL never expires';
COMMENT ON COLUMN api_tokens.created_at IS 'Row creation timestamp';

CREATE TABLE invitations (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id),
    email        email NOT NULL,
    role         membership_role NOT NULL DEFAULT 'member',
    token        TEXT NOT NULL UNIQUE,
    status       invitation_status NOT NULL DEFAULT 'pending',
    invited_by   UUID NOT NULL REFERENCES users(id),
    expires_at   TIMESTAMPTZ NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
COMMENT ON TABLE invitations IS 'Pending invitations for users to join a workspace';
COMMENT ON COLUMN invitations.id IS 'Unique identifier';
COMMENT ON COLUMN invitations.workspace_id IS 'Workspace the invitee is invited to (tenant column)';
COMMENT ON COLUMN invitations.email IS 'Invitee email address';
COMMENT ON COLUMN invitations.role IS 'Role the invitee will receive on acceptance';
COMMENT ON COLUMN invitations.token IS 'Opaque acceptance token';
COMMENT ON COLUMN invitations.status IS 'Current invitation state';
COMMENT ON COLUMN invitations.invited_by IS 'User who issued the invitation';
COMMENT ON COLUMN invitations.expires_at IS 'When the invitation expires';
COMMENT ON COLUMN invitations.created_at IS 'Row creation timestamp';
COMMENT ON COLUMN invitations.updated_at IS 'Last modification timestamp';

-- ============================================================
-- Pack 3 — Advanced planning
-- ============================================================

CREATE TABLE cycles (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id),
    team_id      UUID REFERENCES teams(id),
    name         TEXT NOT NULL,
    status       cycle_status NOT NULL DEFAULT 'upcoming',
    starts_on    DATE NOT NULL,
    ends_on      DATE NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Target of the composite FK from tasks.cycle_id.
    CONSTRAINT cycles_ws_id_key UNIQUE (workspace_id, id)
);
COMMENT ON TABLE cycles IS 'A time-boxed planning iteration (sprint)';
COMMENT ON COLUMN cycles.id IS 'Unique identifier';
COMMENT ON COLUMN cycles.workspace_id IS 'Owning workspace (tenant column)';
COMMENT ON COLUMN cycles.team_id IS 'Optional owning team';
COMMENT ON COLUMN cycles.name IS 'Cycle display name';
COMMENT ON COLUMN cycles.status IS 'Current cycle state';
COMMENT ON COLUMN cycles.starts_on IS 'First day of the cycle';
COMMENT ON COLUMN cycles.ends_on IS 'Last day of the cycle';
COMMENT ON COLUMN cycles.created_at IS 'Row creation timestamp';
COMMENT ON COLUMN cycles.updated_at IS 'Last modification timestamp';

-- task_dependencies: self-referential M2M (both FKs → tasks). Requires manual
-- relationship config in sqlgen.yml (auto-detection does not name self-M2M).
CREATE TABLE task_dependencies (
    task_id            UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    depends_on_task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    type               dependency_type NOT NULL DEFAULT 'blocks',
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (task_id, depends_on_task_id),
    CHECK (task_id <> depends_on_task_id)
);
COMMENT ON TABLE task_dependencies IS 'Directed dependencies between tasks (self-referential M2M)';
COMMENT ON COLUMN task_dependencies.task_id IS 'The dependent task';
COMMENT ON COLUMN task_dependencies.depends_on_task_id IS 'The task depended upon';
COMMENT ON COLUMN task_dependencies.type IS 'Nature of the dependency';
COMMENT ON COLUMN task_dependencies.created_at IS 'Row creation timestamp';

-- task_watchers: M2M task <-> user (a second task<->user junction alongside
-- task_assignees; both are configured with explicit names in sqlgen.yml).
CREATE TABLE task_watchers (
    task_id      UUID NOT NULL,
    user_id      UUID NOT NULL REFERENCES users(id),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    workspace_id UUID NOT NULL,
    PRIMARY KEY (task_id, user_id),
    CONSTRAINT task_watchers_workspace_fkey FOREIGN KEY (workspace_id) REFERENCES workspaces(id),
    CONSTRAINT task_watchers_ws_key UNIQUE (workspace_id, task_id, user_id),
    CONSTRAINT task_watchers_task_same_ws
        FOREIGN KEY (workspace_id, task_id) REFERENCES tasks (workspace_id, id) ON DELETE CASCADE
);
COMMENT ON COLUMN task_watchers.workspace_id IS 'Owning workspace (tenant column); equals the task''s';
COMMENT ON TABLE task_watchers IS 'Users subscribed to updates on a task (M2M)';
COMMENT ON COLUMN task_watchers.task_id IS 'Watched task';
COMMENT ON COLUMN task_watchers.user_id IS 'Watching user';
COMMENT ON COLUMN task_watchers.created_at IS 'When the user started watching';

-- ============================================================
-- Pack 4 — Time tracking & attachments
-- ============================================================

-- time_entries: hours is NUMERIC → exercises the decimal override.
CREATE TABLE time_entries (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id),
    task_id      UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    user_id      UUID NOT NULL REFERENCES users(id),
    hours        NUMERIC(6, 2) NOT NULL,
    billable     BOOLEAN NOT NULL DEFAULT true,
    notes        TEXT,
    spent_on     DATE NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
COMMENT ON TABLE time_entries IS 'Logged time against a task';
COMMENT ON COLUMN time_entries.id IS 'Unique identifier';
COMMENT ON COLUMN time_entries.workspace_id IS 'Owning workspace (tenant column)';
COMMENT ON COLUMN time_entries.task_id IS 'Task the time was logged against';
COMMENT ON COLUMN time_entries.user_id IS 'User who logged the time';
COMMENT ON COLUMN time_entries.hours IS 'Hours worked (decimal)';
COMMENT ON COLUMN time_entries.billable IS 'Whether the time is billable';
COMMENT ON COLUMN time_entries.notes IS 'Optional note';
COMMENT ON COLUMN time_entries.spent_on IS 'The day the work was performed';
COMMENT ON COLUMN time_entries.created_at IS 'Row creation timestamp';

-- attachments: polymorphic — bound to a task OR a comment via
-- (entity_type, entity_id). No FK on entity_id; the parent relationships are
-- configured in sqlgen.yml with a discriminator filter (sub-categorized
-- polymorphism, PRD §13.7).
CREATE TABLE attachments (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id),
    entity_type  attachment_entity_type NOT NULL,
    entity_id    UUID NOT NULL,
    uploaded_by  UUID NOT NULL REFERENCES users(id),
    file_name    TEXT NOT NULL,
    content_type TEXT NOT NULL,
    size_bytes   BIGINT NOT NULL,
    storage_url  TEXT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
COMMENT ON TABLE attachments IS 'Files attached to a task or a comment (polymorphic)';
COMMENT ON COLUMN attachments.id IS 'Unique identifier';
COMMENT ON COLUMN attachments.workspace_id IS 'Owning workspace (tenant column)';
COMMENT ON COLUMN attachments.entity_type IS 'Which entity kind this attachment is bound to';
COMMENT ON COLUMN attachments.entity_id IS 'Primary key of the bound entity (task or comment)';
COMMENT ON COLUMN attachments.uploaded_by IS 'User who uploaded the file';
COMMENT ON COLUMN attachments.file_name IS 'Original file name';
COMMENT ON COLUMN attachments.content_type IS 'MIME type';
COMMENT ON COLUMN attachments.size_bytes IS 'File size in bytes';
COMMENT ON COLUMN attachments.storage_url IS 'Location of the stored blob';
COMMENT ON COLUMN attachments.created_at IS 'Row creation timestamp';

-- ============================================================
-- Wire the optional planning links onto the core tables
-- ============================================================
--
-- A Task can be scheduled only into a Cycle of its OWN Workspace, and a Project
-- assigned only to a Team of its own, at the database, for every writer. The
-- plain cycle_id / team_id FK would let any path that sets the column (the
-- generated update, a future mutation, raw SQL) point across Workspaces, so each
-- is the composite FK (workspace_id, x_id) → x (workspace_id, id) instead, which
-- also subsumes the plain FK. MATCH SIMPLE (the default) skips the check while
-- x_id IS NULL, so an unscheduled/unassigned row needs no extra predicate.

ALTER TABLE projects ADD COLUMN team_id UUID;
ALTER TABLE projects ADD CONSTRAINT projects_team_same_ws
    FOREIGN KEY (workspace_id, team_id) REFERENCES teams (workspace_id, id);
COMMENT ON COLUMN projects.team_id IS 'Optional owning team';

ALTER TABLE tasks ADD COLUMN cycle_id UUID;
ALTER TABLE tasks ADD CONSTRAINT tasks_cycle_same_ws
    FOREIGN KEY (workspace_id, cycle_id) REFERENCES cycles (workspace_id, id);
COMMENT ON COLUMN tasks.cycle_id IS 'Optional planning cycle the task is scheduled in';

-- ============================================================
-- Indexes
-- ============================================================

CREATE INDEX idx_teams_workspace        ON teams (workspace_id);
CREATE INDEX idx_team_members_user      ON team_members (user_id);
CREATE INDEX idx_api_tokens_workspace   ON api_tokens (workspace_id);
CREATE INDEX idx_invitations_workspace  ON invitations (workspace_id);
CREATE INDEX idx_cycles_workspace       ON cycles (workspace_id);
CREATE INDEX idx_task_deps_dependson    ON task_dependencies (depends_on_task_id);
CREATE INDEX idx_task_watchers_user     ON task_watchers (user_id);
CREATE INDEX idx_time_entries_task      ON time_entries (task_id);
CREATE INDEX idx_time_entries_workspace ON time_entries (workspace_id, spent_on);
CREATE INDEX idx_attachments_entity     ON attachments (entity_type, entity_id);
CREATE INDEX idx_projects_team          ON projects (team_id);
CREATE INDEX idx_tasks_cycle            ON tasks (cycle_id);

-- ============================================================
-- project_stats view
-- ============================================================

-- project_stats view — creates the project_stats view IN THE DATABASE.
--
-- NOTE ON DUAL SOURCING: sqlgen deliberately splits its two inputs — CREATE VIEW
-- statements in an input.paths (DDL) file are SKIPPED with a warning; views are
-- generated only from annotation files under input.views. So this view is
-- authored in two places that must stay in sync:
--   * here (migration) — so the view exists in Postgres for the app to query;
--   * views/project_stats.sql (annotation) — so sqlgen generates the Go client.
-- Keep the SELECT identical in both.

CREATE VIEW project_stats AS
SELECT
    p.id           AS project_id,
    p.workspace_id AS workspace_id,
    COUNT(t.id)                                              AS total_tasks,
    COUNT(t.id) FILTER (WHERE t.status = 'done')            AS done_tasks,
    COUNT(t.id) FILTER (WHERE t.status <> 'done')           AS open_tasks
FROM projects p
LEFT JOIN tasks t
    ON t.project_id = p.id
   AND t.deleted_at IS NULL
WHERE p.deleted_at IS NULL
GROUP BY p.id, p.workspace_id;

COMMENT ON VIEW project_stats IS 'Per-project task counts by status';
