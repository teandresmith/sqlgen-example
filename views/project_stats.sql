-- project_stats — sqlgen view annotation file (input.views).
--
-- This is the CODEGEN source for the view (sqlgen parses it to derive the Go
-- struct + read client). The identical SELECT is applied to Postgres by
-- migrations/0002_project_stats_view.up.sql — keep them in sync.
--
-- Directives:
--   @pk       marks the primary key so a Get(ctx, pk) method is generated.
--   Aggregate return types are inferred (COUNT → int64), so no @type needed.

-- @pk: project_id
-- @type project_id: uuid.UUID uuid
-- @type workspace_id: uuid.UUID uuid

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
