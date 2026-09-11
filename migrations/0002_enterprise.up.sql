-- 0002_enterprise — enterprise-tier tables layered on the core schema.
--   Pack 1: Teams & access structure
--   Pack 2: Enterprise auth & security (SSO, API tokens, invitations)
--   Pack 3: Advanced planning (cycles, task dependencies, watchers)
--   Pack 4: Time tracking & attachments
--
-- Demonstrates incremental migration evolution: golang-migrate applies this
-- after 0001, and sqlgen parses 0001+0002 cumulatively (ALTERs included).

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
    UNIQUE (workspace_id, slug)
);
COMMENT ON TABLE teams IS 'A group of users within a workspace that owns projects';
COMMENT ON COLUMN teams.id IS 'Unique team identifier';
COMMENT ON COLUMN teams.workspace_id IS 'Owning workspace (tenant column)';
COMMENT ON COLUMN teams.name IS 'Team display name';
COMMENT ON COLUMN teams.slug IS 'URL-safe short name (unique per workspace)';
COMMENT ON COLUMN teams.description IS 'Optional description';
COMMENT ON COLUMN teams.created_at IS 'Row creation timestamp';
COMMENT ON COLUMN teams.updated_at IS 'Last modification timestamp';

-- team_members: M2M team <-> user (2 FKs in composite PK → auto-detected).
CREATE TABLE team_members (
    team_id  UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id  UUID NOT NULL REFERENCES users(id),
    added_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (team_id, user_id)
);
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
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
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
    task_id    UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (task_id, user_id)
);
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
-- Wire new FKs onto existing tables (incremental ALTERs)
-- ============================================================

ALTER TABLE projects ADD COLUMN team_id UUID REFERENCES teams(id);
COMMENT ON COLUMN projects.team_id IS 'Optional owning team';

ALTER TABLE tasks ADD COLUMN cycle_id UUID REFERENCES cycles(id);
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
