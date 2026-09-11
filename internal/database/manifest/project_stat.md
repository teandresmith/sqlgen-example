# ProjectStat

- **Table:** `project_stats` (schema `public`)
- **Kind:** view
- **Source:** `-- project_stats — sqlgen view annotation file (input.views).
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
`

## Files

- `project_stat_gen.go`

## Primary key

- Kind: single
- `project_id` — ProjectID `uuid.UUID`

## Columns

| Name | Go field | Go type | DB type | Null | PK | Unique | Default | Comparator | Comment |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `project_id` | ProjectID | `uuid.UUID` | `uuid.UUID` |  | yes |  |  | `comparator.ID` |  |
| `workspace_id` | WorkspaceID | `uuid.UUID` | `uuid.UUID` |  |  |  |  | `comparator.String` |  |
| `total_tasks` | TotalTasks | `int64` | `int64` |  |  |  |  | `comparator.Number[int64]` |  |
| `done_tasks` | DoneTasks | `int64` | `int64` |  |  |  |  | `comparator.Number[int64]` |  |
| `open_tasks` | OpenTasks | `int64` | `int64` |  |  |  |  | `comparator.Number[int64]` |  |

## Query methods

### Get

- Params: `id uuid.UUID`
- Returns: `*ProjectStat`, `error`
- Errors: `ErrNotFound`, `tenancy.ErrMissing`
- Notes: Generated from the view's @pk annotation.

Generated SQL:

postgres:

```sql
SELECT "done_tasks", "open_tasks", "project_id", "total_tasks", "workspace_id" FROM "public"."project_stats" WHERE "project_id" = $1 LIMIT 1
```

### GetMany

- Params: `input *GetProjectStatsInput`
- Returns: `[]*ProjectStat`, `error`
- Errors: `tenancy.ErrMissing`

Generated SQL:

postgres:

```sql
SELECT "done_tasks", "open_tasks", "project_id", "total_tasks", "workspace_id" FROM "public"."project_stats" WHERE <filter> ORDER BY <sort> LIMIT <limit>
```

### Count

- Params: `filter *ProjectStatFilter`
- Returns: `int64`, `error`
- Errors: `tenancy.ErrMissing`

Generated SQL:

postgres:

```sql
SELECT COUNT(*) FROM "public"."project_stats" WHERE <filter>
```

### Paginate

- Params: `input PaginateInput[ProjectStatFilter]`
- Returns: `*PaginateResult[ProjectStat]`, `error`
- Errors: `tenancy.ErrMissing`
- Notes: Offset pagination. Issues no statement of its own — composes Count and GetMany, so no sql_bodies entry.

### Connection

- Params: `input ConnectionInput[ProjectStatFilter]`
- Returns: `*Connection[ProjectStat]`, `error`
- Errors: `ErrInvalidCursor`, `tenancy.ErrMissing`
- Notes: Relay cursor pagination. Issues no statement of its own — composes Count and GetMany, so no sql_bodies entry.

## Filter

Type `ProjectStatFilter`.

| Field | Type |
| --- | --- |
| DoneTasks | `*comparator.Number[int64]` |
| OpenTasks | `*comparator.Number[int64]` |
| ProjectID | `*comparator.ID` |
| TotalTasks | `*comparator.Number[int64]` |
| WorkspaceID | `*comparator.String` |
| And | `[]*ProjectStatFilter` |
| Or | `[]*ProjectStatFilter` |

## Sort

Type `ProjectStatSort`.

Fields: DoneTasks, OpenTasks, ProjectID, TotalTasks, WorkspaceID
