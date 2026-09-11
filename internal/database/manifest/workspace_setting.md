# WorkspaceSetting

- **Table:** `workspace_settings` (schema `public`)
- **Kind:** table

Per-workspace configuration (one row per workspace)

## Files

- `workspace_setting_gen.go`

## Primary key

- Kind: single
- `id` — ID `uuid.UUID`

## Columns

| Name | Go field | Go type | DB type | Null | PK | Unique | Default | Comparator | Comment |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `id` | ID | `uuid.UUID` | `uuid` |  | yes |  | `gen_random_uuid()` | `comparator.ID` | Unique identifier |
| `workspace_id` | WorkspaceID | `uuid.UUID` | `uuid` |  |  | yes |  | `comparator.ID` | Owning workspace (unique → one-to-one) |
| `settings` | Settings | `types.JSON` | `jsonb` |  |  |  | `'{}'::jsonb` | `comparator.JSONB` | Arbitrary configuration payload |
| `created_at` | CreatedAt | `time.Time` | `timestamptz` |  |  |  | `now()` | `comparator.Time` | Row creation timestamp |
| `updated_at` | UpdatedAt | `time.Time` | `timestamptz` |  |  |  | `now()` | `comparator.Time` | Last modification timestamp |

## Indexes

| Name | Columns | Unique | Method | Where |
| --- | --- | --- | --- | --- |
| `workspace_settings_pkey` | id | yes | btree |  |
| `workspace_settings_workspace_id_key` | workspace_id | yes | btree |  |

## Relationships

| Name | Kind | Target | FK | Filter |
| --- | --- | --- | --- | --- |
| workspaces | o2o | Workspace | public.workspace_settings.workspace_id |  |

## Query methods

### Get

- Params: `id uuid.UUID`
- Returns: `*WorkspaceSetting`, `error`
- Errors: `ErrNotFound`, `tenancy.ErrMissing`
- Notes: Delegates to GetMany with a primary-key filter; returns ErrNotFound when no row matches.

Generated SQL:

postgres:

```sql
SELECT "created_at", "id", "settings", "updated_at", "workspace_id" FROM "public"."workspace_settings" WHERE "id" = $1 LIMIT 1
```

### GetMany

- Params: `input *GetWorkspaceSettingsInput`
- Returns: `[]*WorkspaceSetting`, `error`
- Errors: `tenancy.ErrMissing`

Generated SQL:

postgres:

```sql
SELECT "created_at", "id", "settings", "updated_at", "workspace_id" FROM "public"."workspace_settings" WHERE <filter> ORDER BY <sort> LIMIT <limit>
```

### Count

- Params: `filter *WorkspaceSettingFilter`
- Returns: `int64`, `error`
- Errors: `tenancy.ErrMissing`

Generated SQL:

postgres:

```sql
SELECT COUNT(*) FROM "public"."workspace_settings" WHERE <filter>
```

### Exists

- Params: `id uuid.UUID`
- Returns: `bool`, `error`
- Errors: `tenancy.ErrMissing`

Generated SQL:

postgres:

```sql
SELECT EXISTS(SELECT 1 FROM "public"."workspace_settings" WHERE "id" = $1)
```

### ExistsWhere

- Params: `filter *WorkspaceSettingFilter`
- Returns: `bool`, `error`
- Errors: `tenancy.ErrMissing`

Generated SQL:

postgres:

```sql
SELECT EXISTS(SELECT 1 FROM "public"."workspace_settings" WHERE <filter>)
```

### Paginate

- Params: `input PaginateInput[WorkspaceSettingFilter]`
- Returns: `*PaginateResult[WorkspaceSetting]`, `error`
- Errors: `tenancy.ErrMissing`
- Notes: Offset pagination. Issues no statement of its own — composes Count and GetMany, so no sql_bodies entry.

### Connection

- Params: `input ConnectionInput[WorkspaceSettingFilter]`
- Returns: `*Connection[WorkspaceSetting]`, `error`
- Errors: `ErrInvalidCursor`, `tenancy.ErrMissing`
- Notes: Relay cursor pagination. Issues no statement of its own — composes Count and GetMany, so no sql_bodies entry.

### Stream

- Params: `input *StreamWorkspaceSettingsInput`
- Returns: `iter.Seq2[*WorkspaceSetting, error]`, `error`
- Errors: `tenancy.ErrMissing`
- Notes: Scalars only — no relationship loading, cache always bypassed. Errors surface through the iterator's second value.

Generated SQL:

postgres:

```sql
SELECT "created_at", "id", "settings", "updated_at", "workspace_id" FROM "public"."workspace_settings" WHERE <filter> ORDER BY <sort>
```

## Mutation methods

### Create

- Params: `input *CreateWorkspaceSettingInput`
- Returns: `*WorkspaceSetting`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`

Generated SQL:

postgres:

```sql
INSERT INTO "public"."workspace_settings" (<columns>) VALUES (<values>) RETURNING "id"
```

### CreateMany

- Params: `inputs []*CreateWorkspaceSettingInput`
- Returns: `[]*WorkspaceSetting`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Batched in generation.batch_size chunks. Earlier chunks are not rolled back on a later failure — wrap in a transaction for atomicity.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."workspace_settings" (<columns>) VALUES <values> RETURNING "id"
```

### Upsert

- Params: `input *CreateWorkspaceSettingInput`, `target WorkspaceSettingConflictTarget`
- Returns: `*WorkspaceSetting`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: One method over the generated WorkspaceSettingConflictTarget enum — the target argument selects the conflict columns at call time.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."workspace_settings" (<columns>) VALUES (<values>) ON CONFLICT (<conflict_target>) DO UPDATE SET <excluded> RETURNING "id"
```

### UpsertMany

- Params: `inputs []*CreateWorkspaceSettingInput`, `target WorkspaceSettingConflictTarget`
- Returns: `[]*WorkspaceSetting`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Batched in generation.batch_size chunks over the same WorkspaceSettingConflictTarget enum Upsert takes. Inputs resolving to one target are deduped before the statement is built, last occurrence wins. Earlier chunks are not rolled back on a later failure — wrap in a transaction for atomicity.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."workspace_settings" (<columns>) VALUES <values> ON CONFLICT (<conflict_target>) DO UPDATE SET <excluded>
```

### Update

- Params: `id uuid.UUID`, `input *UpdateWorkspaceSettingInput`
- Returns: `*WorkspaceSetting`, `error`
- Errors: `ErrNotFound`, `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Only fields where IsSet() reports true reach the SET clause; an input with none set issues no UPDATE and returns the row unchanged.

Generated SQL:

postgres:

```sql
UPDATE "public"."workspace_settings" SET <set> WHERE "id" = $1
```

### UpdateMany

- Params: `items []UpdateWorkspaceSettingItem`
- Returns: `[]*WorkspaceSetting`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Issues one statement per item. Batched like CreateMany — earlier items are not rolled back on a later failure. A primary key that does not exist is silently skipped, even under strict_updates.

Generated SQL:

postgres:

```sql
UPDATE "public"."workspace_settings" SET <set> WHERE "id" = $1
```

### UpdateWhere

- Params: `filter *WorkspaceSettingFilter`, `input *UpdateWorkspaceSettingInput`
- Returns: `[]*WorkspaceSetting`, `error`
- Errors: `ErrNilInput`, `ErrEmptyFilter`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Idempotent — returns an empty slice when no row matches. ErrEmptyFilter when the filter produces no conditions.

Generated SQL:

postgres:

```sql
UPDATE "public"."workspace_settings" SET <set> WHERE <filter> RETURNING "id"
```

### HardDelete

- Params: `id uuid.UUID`
- Returns: `error`
- Errors: `tenancy.ErrMissing`
- Notes: Idempotent — a primary key that does not exist is not an error.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."workspace_settings" WHERE "id" = $1
```

### HardDeleteMany

- Params: `ids []uuid.UUID`
- Returns: `error`
- Errors: `tenancy.ErrMissing`
- Notes: Idempotent — primary keys that do not exist are not an error.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."workspace_settings" WHERE "id" IN (<ids>)
```

### HardDeleteWhere

- Params: `filter *WorkspaceSettingFilter`
- Returns: `error`
- Errors: `ErrEmptyFilter`, `tenancy.ErrMissing`
- Notes: Idempotent — no match is not an error. ErrEmptyFilter when the filter produces no conditions.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."workspace_settings" WHERE <filter> RETURNING "id"
```

## Filter

Type `WorkspaceSettingFilter`.

| Field | Type |
| --- | --- |
| CreatedAt | `*comparator.Time` |
| ID | `*comparator.ID` |
| Settings | `*comparator.JSONB` |
| UpdatedAt | `*comparator.Time` |
| WorkspaceID | `*comparator.ID` |
| And | `[]*WorkspaceSettingFilter` |
| Or | `[]*WorkspaceSettingFilter` |

## Sort

Type `WorkspaceSettingSort`.

Fields: CreatedAt, ID, Settings, UpdatedAt, WorkspaceID
