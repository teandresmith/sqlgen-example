# Project

- **Table:** `projects` (schema `public`)
- **Kind:** table

A container for tasks, owned by one workspace

## Files

- `project_gen.go`

## Primary key

- Kind: single
- `id` — ID `uuid.UUID`

## Columns

| Name | Go field | Go type | DB type | Null | PK | Unique | Default | Comparator | Comment |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `id` | ID | `uuid.UUID` | `uuid` |  | yes |  | `gen_random_uuid()` | `comparator.ID` | Unique project identifier |
| `workspace_id` | WorkspaceID | `uuid.UUID` | `uuid` |  |  |  |  | `comparator.ID` | Owning workspace (tenant column) |
| `name` | Name | `string` | `text` |  |  |  |  | `comparator.String` | Project display name |
| `description` | Description | `*string` | `text` | yes |  |  |  | `comparator.NullableString` | Optional long-form description |
| `created_at` | CreatedAt | `time.Time` | `timestamptz` |  |  |  | `now()` | `comparator.Time` | Row creation timestamp |
| `updated_at` | UpdatedAt | `time.Time` | `timestamptz` |  |  |  | `now()` | `comparator.Time` | Last modification timestamp |
| `deleted_at` | DeletedAt | `*time.Time` | `timestamptz` | yes |  |  |  | `comparator.NullableTime` | Soft-delete (archive) timestamp; NULL when active |
| `team_id` | TeamID | `*uuid.UUID` | `uuid` | yes |  |  |  | `comparator.NullableID` | Optional owning team |

## Indexes

| Name | Columns | Unique | Method | Where |
| --- | --- | --- | --- | --- |
| `idx_projects_team` | team_id |  | btree |  |
| `idx_projects_workspace` | workspace_id |  | btree |  |
| `projects_pkey` | id | yes | btree |  |

## Relationships

| Name | Kind | Target | FK | Filter |
| --- | --- | --- | --- | --- |
| tasks | o2m | Task | public.tasks.project_id |  |

## Query methods

### Get

- Params: `id uuid.UUID`
- Returns: `*Project`, `error`
- Errors: `ErrNotFound`, `tenancy.ErrMissing`
- Notes: Delegates to GetMany with a primary-key filter; returns ErrNotFound when no row matches.

Generated SQL:

postgres:

```sql
SELECT "created_at", "deleted_at", "description", "id", "name", "team_id", "updated_at", "workspace_id" FROM "public"."projects" WHERE "id" = $1 AND "deleted_at" IS NULL LIMIT 1
```

### GetMany

- Params: `input *GetProjectsInput`
- Returns: `[]*Project`, `error`
- Errors: `tenancy.ErrMissing`

Generated SQL:

postgres:

```sql
SELECT "created_at", "deleted_at", "description", "id", "name", "team_id", "updated_at", "workspace_id" FROM "public"."projects" WHERE <filter> AND "deleted_at" IS NULL ORDER BY <sort> LIMIT <limit>
```

### Count

- Params: `filter *ProjectFilter`
- Returns: `int64`, `error`
- Errors: `tenancy.ErrMissing`

Generated SQL:

postgres:

```sql
SELECT COUNT(*) FROM "public"."projects" WHERE <filter> AND "deleted_at" IS NULL
```

### Exists

- Params: `id uuid.UUID`
- Returns: `bool`, `error`
- Errors: `tenancy.ErrMissing`

Generated SQL:

postgres:

```sql
SELECT EXISTS(SELECT 1 FROM "public"."projects" WHERE "id" = $1 AND "deleted_at" IS NULL)
```

### ExistsWhere

- Params: `filter *ProjectFilter`
- Returns: `bool`, `error`
- Errors: `tenancy.ErrMissing`

Generated SQL:

postgres:

```sql
SELECT EXISTS(SELECT 1 FROM "public"."projects" WHERE <filter> AND "deleted_at" IS NULL)
```

### Paginate

- Params: `input PaginateInput[ProjectFilter]`
- Returns: `*PaginateResult[Project]`, `error`
- Errors: `tenancy.ErrMissing`
- Notes: Offset pagination. Issues no statement of its own — composes Count and GetMany, so no sql_bodies entry.

### Connection

- Params: `input ConnectionInput[ProjectFilter]`
- Returns: `*Connection[Project]`, `error`
- Errors: `ErrInvalidCursor`, `tenancy.ErrMissing`
- Notes: Relay cursor pagination. Issues no statement of its own — composes Count and GetMany, so no sql_bodies entry.

### Stream

- Params: `input *StreamProjectsInput`
- Returns: `iter.Seq2[*Project, error]`, `error`
- Errors: `tenancy.ErrMissing`
- Notes: Scalars only — no relationship loading, cache always bypassed. Errors surface through the iterator's second value.

Generated SQL:

postgres:

```sql
SELECT "created_at", "deleted_at", "description", "id", "name", "team_id", "updated_at", "workspace_id" FROM "public"."projects" WHERE <filter> AND "deleted_at" IS NULL ORDER BY <sort>
```

## Mutation methods

### Create

- Params: `input *CreateProjectInput`
- Returns: `*Project`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`

Generated SQL:

postgres:

```sql
INSERT INTO "public"."projects" (<columns>) VALUES (<values>) RETURNING "id"
```

### CreateMany

- Params: `inputs []*CreateProjectInput`
- Returns: `[]*Project`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Batched in generation.batch_size chunks. Earlier chunks are not rolled back on a later failure — wrap in a transaction for atomicity.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."projects" (<columns>) VALUES <values> RETURNING "id"
```

### Upsert

- Params: `input *CreateProjectInput`, `target ProjectConflictTarget`
- Returns: `*Project`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: One method over the generated ProjectConflictTarget enum — the target argument selects the conflict columns at call time.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."projects" (<columns>) VALUES (<values>) ON CONFLICT (<conflict_target>) DO UPDATE SET <excluded> RETURNING "id"
```

### UpsertMany

- Params: `inputs []*CreateProjectInput`, `target ProjectConflictTarget`
- Returns: `[]*Project`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Batched in generation.batch_size chunks over the same ProjectConflictTarget enum Upsert takes. Inputs resolving to one target are deduped before the statement is built, last occurrence wins. Earlier chunks are not rolled back on a later failure — wrap in a transaction for atomicity.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."projects" (<columns>) VALUES <values> ON CONFLICT (<conflict_target>) DO UPDATE SET <excluded>
```

### Update

- Params: `id uuid.UUID`, `input *UpdateProjectInput`
- Returns: `*Project`, `error`
- Errors: `ErrNotFound`, `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Only fields where IsSet() reports true reach the SET clause; an input with none set issues no UPDATE and returns the row unchanged.

Generated SQL:

postgres:

```sql
UPDATE "public"."projects" SET <set> WHERE "id" = $1
```

### UpdateMany

- Params: `items []UpdateProjectItem`
- Returns: `[]*Project`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Issues one statement per item. Batched like CreateMany — earlier items are not rolled back on a later failure. A primary key that does not exist is silently skipped, even under strict_updates.

Generated SQL:

postgres:

```sql
UPDATE "public"."projects" SET <set> WHERE "id" = $1
```

### UpdateWhere

- Params: `filter *ProjectFilter`, `input *UpdateProjectInput`
- Returns: `[]*Project`, `error`
- Errors: `ErrNilInput`, `ErrEmptyFilter`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Idempotent — returns an empty slice when no row matches. ErrEmptyFilter when the filter produces no conditions.

Generated SQL:

postgres:

```sql
UPDATE "public"."projects" SET <set> WHERE <filter> RETURNING "id"
```

### SoftDelete

- Params: `id uuid.UUID`
- Returns: `*Project`, `error`
- Errors: `tenancy.ErrMissing`
- Notes: Idempotent — a primary key that does not exist is not an error.

Generated SQL:

postgres:

```sql
UPDATE "public"."projects" SET "deleted_at" = CURRENT_TIMESTAMP WHERE "id" = $1
```

### SoftDeleteMany

- Params: `ids []uuid.UUID`
- Returns: `[]*Project`, `error`
- Errors: `tenancy.ErrMissing`
- Notes: Idempotent — primary keys that do not exist are not an error.

Generated SQL:

postgres:

```sql
UPDATE "public"."projects" SET "deleted_at" = CURRENT_TIMESTAMP WHERE "id" IN (<ids>)
```

### SoftDeleteWhere

- Params: `filter *ProjectFilter`
- Returns: `[]*Project`, `error`
- Errors: `ErrEmptyFilter`, `tenancy.ErrMissing`
- Notes: Idempotent — no match is not an error. ErrEmptyFilter when the filter produces no conditions.

Generated SQL:

postgres:

```sql
UPDATE "public"."projects" SET "deleted_at" = CURRENT_TIMESTAMP WHERE <filter> AND "deleted_at" IS NULL RETURNING "id"
```

### Restore

- Params: `id uuid.UUID`
- Returns: `*Project`, `error`
- Errors: `tenancy.ErrMissing`
- Notes: Idempotent — a primary key that does not exist is not an error.

Generated SQL:

postgres:

```sql
UPDATE "public"."projects" SET "deleted_at" = $1 WHERE "id" = $2
```

### RestoreMany

- Params: `ids []uuid.UUID`
- Returns: `[]*Project`, `error`
- Errors: `tenancy.ErrMissing`
- Notes: Idempotent — primary keys that do not exist are not an error.

Generated SQL:

postgres:

```sql
UPDATE "public"."projects" SET "deleted_at" = $1 WHERE "id" IN (<ids>)
```

### RestoreWhere

- Params: `filter *ProjectFilter`
- Returns: `[]*Project`, `error`
- Errors: `ErrEmptyFilter`, `tenancy.ErrMissing`
- Notes: Idempotent — no match is not an error. ErrEmptyFilter when the filter produces no conditions.

Generated SQL:

postgres:

```sql
UPDATE "public"."projects" SET "deleted_at" = $1 WHERE <filter> AND "deleted_at" IS NOT NULL RETURNING "id"
```

### HardDelete

- Params: `id uuid.UUID`
- Returns: `error`
- Errors: `tenancy.ErrMissing`
- Notes: Idempotent — a primary key that does not exist is not an error.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."projects" WHERE "id" = $1
```

### HardDeleteMany

- Params: `ids []uuid.UUID`
- Returns: `error`
- Errors: `tenancy.ErrMissing`
- Notes: Idempotent — primary keys that do not exist are not an error.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."projects" WHERE "id" IN (<ids>)
```

### HardDeleteWhere

- Params: `filter *ProjectFilter`
- Returns: `error`
- Errors: `ErrEmptyFilter`, `tenancy.ErrMissing`
- Notes: Idempotent — no match is not an error. ErrEmptyFilter when the filter produces no conditions.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."projects" WHERE <filter> RETURNING "id"
```

### UpdateWithRelated

- Params: `id uuid.UUID`, `input *UpdateProjectWithRelatedInput`
- Returns: `*Project`, `error`
- Errors: `ErrNilInput`, `ErrNotFound`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Writes the parent and its nested rows in one transaction — a SAVEPOINT when ctx already holds one. Issues no statement of its own: it composes Update and, per eligible relationship (Tasks), the target's and junction's own client methods, so no sql_bodies entry. Each inner call fires its own hooks, events and cache invalidation under its own op; the nested method has none.

## Filter

Type `ProjectFilter`.

| Field | Type |
| --- | --- |
| CreatedAt | `*comparator.Time` |
| DeletedAt | `*comparator.NullableTime` |
| Description | `*comparator.NullableString` |
| ID | `*comparator.ID` |
| Name | `*comparator.String` |
| TeamID | `*comparator.NullableID` |
| UpdatedAt | `*comparator.Time` |
| WorkspaceID | `*comparator.ID` |
| And | `[]*ProjectFilter` |
| Or | `[]*ProjectFilter` |

## Sort

Type `ProjectSort`.

Fields: CreatedAt, DeletedAt, Description, ID, Name, TeamID, UpdatedAt, WorkspaceID
