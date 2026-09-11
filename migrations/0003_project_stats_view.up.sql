-- 0002_project_stats_view — creates the project_stats view IN THE DATABASE.
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
