# TaskWatcher

- **Table:** `task_watchers` (schema `public`)
- **Kind:** table

Users subscribed to updates on a task (M2M)

## Files

- `task_watcher_gen.go`

## Primary key

- Kind: composite
- Struct: `TaskWatcherPK`
- `task_id` — TaskID `uuid.UUID`
- `user_id` — UserID `uuid.UUID`

## Columns

| Name | Go field | Go type | DB type | Null | PK | Unique | Default | Comparator | Comment |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `task_id` | TaskID | `uuid.UUID` | `uuid` |  | yes |  |  | `comparator.ID` | Watched task |
| `user_id` | UserID | `uuid.UUID` | `uuid` |  | yes |  |  | `comparator.ID` | Watching user |
| `created_at` | CreatedAt | `time.Time` | `timestamptz` |  |  |  | `now()` | `comparator.Time` | When the user started watching |
| `workspace_id` | WorkspaceID | `uuid.UUID` | `uuid` |  |  |  |  | `comparator.ID` | Owning workspace (tenant column); equals the task's |

## Indexes

| Name | Columns | Unique | Method | Where |
| --- | --- | --- | --- | --- |
| `idx_task_watchers_user` | user_id |  | btree |  |
| `task_watchers_pkey` | task_id, user_id | yes | btree |  |
| `task_watchers_ws_key` | workspace_id, task_id, user_id | yes | btree |  |

## Query methods

### Get

- Params: `pk TaskWatcherPK`
- Returns: `*TaskWatcher`, `error`
- Errors: `ErrNotFound`, `tenancy.ErrMissing`
- Notes: Delegates to GetMany with a primary-key filter; returns ErrNotFound when no row matches.

Generated SQL:

postgres:

```sql
SELECT "created_at", "task_id", "user_id", "workspace_id" FROM "public"."task_watchers" WHERE "task_id" = $1 AND "user_id" = $2 LIMIT 1
```

### GetMany

- Params: `input *GetTaskWatchersInput`
- Returns: `[]*TaskWatcher`, `error`
- Errors: `tenancy.ErrMissing`

Generated SQL:

postgres:

```sql
SELECT "created_at", "task_id", "user_id", "workspace_id" FROM "public"."task_watchers" WHERE <filter> ORDER BY <sort> LIMIT <limit>
```

### Exists

- Params: `pk TaskWatcherPK`
- Returns: `bool`, `error`
- Errors: `tenancy.ErrMissing`

Generated SQL:

postgres:

```sql
SELECT EXISTS(SELECT 1 FROM "public"."task_watchers" WHERE "task_id" = $1 AND "user_id" = $2)
```

### ExistsWhere

- Params: `filter *TaskWatcherFilter`
- Returns: `bool`, `error`
- Errors: `tenancy.ErrMissing`

Generated SQL:

postgres:

```sql
SELECT EXISTS(SELECT 1 FROM "public"."task_watchers" WHERE <filter>)
```

### Count

- Params: `filter *TaskWatcherFilter`
- Returns: `int64`, `error`
- Errors: `tenancy.ErrMissing`

Generated SQL:

postgres:

```sql
SELECT COUNT(*) FROM "public"."task_watchers" WHERE <filter>
```

### Paginate

- Params: `input PaginateInput[TaskWatcherFilter]`
- Returns: `*PaginateResult[TaskWatcher]`, `error`
- Errors: `tenancy.ErrMissing`
- Notes: Offset pagination. Issues no statement of its own — composes Count and GetMany, so no sql_bodies entry.

### Connection

- Params: `input ConnectionInput[TaskWatcherFilter]`
- Returns: `*Connection[TaskWatcher]`, `error`
- Errors: `ErrInvalidCursor`, `tenancy.ErrMissing`
- Notes: Relay cursor pagination. Issues no statement of its own — composes Count and GetMany, so no sql_bodies entry.

### Stream

- Params: `input *StreamTaskWatchersInput`
- Returns: `iter.Seq2[*TaskWatcher, error]`, `error`
- Errors: `tenancy.ErrMissing`
- Notes: Scalars only — no relationship loading, cache always bypassed. Errors surface through the iterator's second value.

Generated SQL:

postgres:

```sql
SELECT "created_at", "task_id", "user_id", "workspace_id" FROM "public"."task_watchers" WHERE <filter> ORDER BY <sort>
```

## Mutation methods

### Create

- Params: `input *CreateTaskWatcherInput`
- Returns: `*TaskWatcher`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`

Generated SQL:

postgres:

```sql
INSERT INTO "public"."task_watchers" (<columns>) VALUES (<values>)
```

### CreateMany

- Params: `inputs []*CreateTaskWatcherInput`
- Returns: `[]*TaskWatcher`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Batched in generation.batch_size chunks. Earlier chunks are not rolled back on a later failure — wrap in a transaction for atomicity.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."task_watchers" (<columns>) VALUES <values>
```

### Update

- Params: `pk TaskWatcherPK`, `input *UpdateTaskWatcherInput`
- Returns: `*TaskWatcher`, `error`
- Errors: `ErrNotFound`, `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Only fields where IsSet() reports true reach the SET clause; an input with none set issues no UPDATE and returns the row unchanged.

Generated SQL:

postgres:

```sql
UPDATE "public"."task_watchers" SET <set> WHERE "task_id" = $1 AND "user_id" = $2
```

### UpdateMany

- Params: `items []UpdateTaskWatcherItem`
- Returns: `[]*TaskWatcher`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Issues one statement per item. Batched like CreateMany — earlier items are not rolled back on a later failure. A primary key that does not exist is silently skipped, even under strict_updates.

Generated SQL:

postgres:

```sql
UPDATE "public"."task_watchers" SET <set> WHERE "task_id" = $1 AND "user_id" = $2
```

### UpdateWhere

- Params: `filter *TaskWatcherFilter`, `input *UpdateTaskWatcherInput`
- Returns: `[]*TaskWatcher`, `error`
- Errors: `ErrNilInput`, `ErrEmptyFilter`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Idempotent — returns an empty slice when no row matches. ErrEmptyFilter when the filter produces no conditions.

Generated SQL:

postgres:

```sql
UPDATE "public"."task_watchers" SET <set> WHERE <filter> RETURNING "task_id", "user_id"
```

### Upsert

- Params: `input *CreateTaskWatcherInput`, `target TaskWatcherConflictTarget`
- Returns: `*TaskWatcher`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: One method over the generated TaskWatcherConflictTarget enum — the target argument selects the conflict columns at call time.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."task_watchers" (<columns>) VALUES (<values>) ON CONFLICT (<conflict_target>) DO UPDATE SET <excluded>
```

### UpsertMany

- Params: `inputs []*CreateTaskWatcherInput`, `target TaskWatcherConflictTarget`
- Returns: `[]*TaskWatcher`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Batched in generation.batch_size chunks over the same TaskWatcherConflictTarget enum Upsert takes. Inputs resolving to one target are deduped before the statement is built, last occurrence wins. Earlier chunks are not rolled back on a later failure — wrap in a transaction for atomicity.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."task_watchers" (<columns>) VALUES <values> ON CONFLICT (<conflict_target>) DO UPDATE SET <excluded>
```

### HardDelete

- Params: `pk TaskWatcherPK`
- Returns: `error`
- Errors: `tenancy.ErrMissing`
- Notes: Idempotent — a primary key that does not exist is not an error.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."task_watchers" WHERE "task_id" = $1 AND "user_id" = $2
```

### HardDeleteMany

- Params: `pks []TaskWatcherPK`
- Returns: `error`
- Errors: `tenancy.ErrMissing`
- Notes: Idempotent — primary keys that do not exist are not an error.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."task_watchers" WHERE ("task_id", "user_id") IN (<pks>)
```

### HardDeleteWhere

- Params: `filter *TaskWatcherFilter`
- Returns: `error`
- Errors: `ErrEmptyFilter`, `tenancy.ErrMissing`
- Notes: Idempotent — no match is not an error. ErrEmptyFilter when the filter produces no conditions.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."task_watchers" WHERE <filter> RETURNING "task_id", "user_id"
```

## Filter

Type `TaskWatcherFilter`.

| Field | Type |
| --- | --- |
| CreatedAt | `*comparator.Time` |
| TaskID | `*comparator.ID` |
| UserID | `*comparator.ID` |
| WorkspaceID | `*comparator.ID` |
| And | `[]*TaskWatcherFilter` |
| Or | `[]*TaskWatcherFilter` |

## Sort

Type `TaskWatcherSort`.

Fields: CreatedAt, TaskID, UserID, WorkspaceID

## Read examples

```go
row, err := client.TaskWatchers().Get(ctx, database.TaskWatcherPK{TaskID: uuid.NewV4(), UserID: uuid.NewV4()})
rows, err := client.TaskWatchers().GetMany(ctx, &database.GetTaskWatchersInput{Filter: &database.TaskWatcherFilter{CreatedAt: &comparator.Time{Eq: new(time.Now())}}})
```

## Write examples

```go
created, err := client.TaskWatchers().Create(ctx, &database.CreateTaskWatcherInput{TaskID: uuid.NewV4(), UserID: uuid.NewV4()})
updated, err := client.TaskWatchers().Update(ctx, database.TaskWatcherPK{TaskID: uuid.NewV4(), UserID: uuid.NewV4()}, &database.UpdateTaskWatcherInput{CreatedAt: omittable.Set(time.Now())})
```
