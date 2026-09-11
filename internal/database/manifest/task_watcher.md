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

## Indexes

| Name | Columns | Unique | Method | Where |
| --- | --- | --- | --- | --- |
| `idx_task_watchers_user` | user_id |  | btree |  |
| `task_watchers_pkey` | task_id, user_id | yes | btree |  |

## Query methods

### Get

- Params: `pk TaskWatcherPK`
- Returns: `*TaskWatcher`, `error`
- Errors: `ErrNotFound`
- Notes: Delegates to GetMany with a primary-key filter; returns ErrNotFound when no row matches.

Generated SQL:

postgres:

```sql
SELECT "created_at", "task_id", "user_id" FROM "public"."task_watchers" WHERE "task_id" = $1 AND "user_id" = $2 LIMIT 1
```

### GetMany

- Params: `input *GetTaskWatchersInput`
- Returns: `[]*TaskWatcher`, `error`

Generated SQL:

postgres:

```sql
SELECT "created_at", "task_id", "user_id" FROM "public"."task_watchers" WHERE <filter> ORDER BY <sort> LIMIT <limit>
```

### Count

- Params: `filter *TaskWatcherFilter`
- Returns: `int64`, `error`

Generated SQL:

postgres:

```sql
SELECT COUNT(*) FROM "public"."task_watchers" WHERE <filter>
```

### Exists

- Params: `pk TaskWatcherPK`
- Returns: `bool`, `error`

Generated SQL:

postgres:

```sql
SELECT EXISTS(SELECT 1 FROM "public"."task_watchers" WHERE "task_id" = $1 AND "user_id" = $2)
```

### ExistsWhere

- Params: `filter *TaskWatcherFilter`
- Returns: `bool`, `error`

Generated SQL:

postgres:

```sql
SELECT EXISTS(SELECT 1 FROM "public"."task_watchers" WHERE <filter>)
```

### Paginate

- Params: `input PaginateInput[TaskWatcherFilter]`
- Returns: `*PaginateResult[TaskWatcher]`, `error`
- Notes: Offset pagination. Issues no statement of its own — composes Count and GetMany, so no sql_bodies entry.

### Connection

- Params: `input ConnectionInput[TaskWatcherFilter]`
- Returns: `*Connection[TaskWatcher]`, `error`
- Errors: `ErrInvalidCursor`
- Notes: Relay cursor pagination. Issues no statement of its own — composes Count and GetMany, so no sql_bodies entry.

### Stream

- Params: `input *StreamTaskWatchersInput`
- Returns: `iter.Seq2[*TaskWatcher, error]`, `error`
- Notes: Scalars only — no relationship loading, cache always bypassed. Errors surface through the iterator's second value.

Generated SQL:

postgres:

```sql
SELECT "created_at", "task_id", "user_id" FROM "public"."task_watchers" WHERE <filter> ORDER BY <sort>
```

## Mutation methods

### Create

- Params: `input *CreateTaskWatcherInput`
- Returns: `*TaskWatcher`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`

Generated SQL:

postgres:

```sql
INSERT INTO "public"."task_watchers" (<columns>) VALUES (<values>)
```

### CreateMany

- Params: `inputs []*CreateTaskWatcherInput`
- Returns: `[]*TaskWatcher`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`
- Notes: Batched in generation.batch_size chunks. Earlier chunks are not rolled back on a later failure — wrap in a transaction for atomicity.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."task_watchers" (<columns>) VALUES <values>
```

### Upsert

- Params: `input *CreateTaskWatcherInput`, `target TaskWatcherConflictTarget`
- Returns: `*TaskWatcher`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`
- Notes: One method over the generated TaskWatcherConflictTarget enum — the target argument selects the conflict columns at call time.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."task_watchers" (<columns>) VALUES (<values>) ON CONFLICT (<conflict_target>) DO UPDATE SET <excluded>
```

### UpsertMany

- Params: `inputs []*CreateTaskWatcherInput`, `target TaskWatcherConflictTarget`
- Returns: `[]*TaskWatcher`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`
- Notes: Batched in generation.batch_size chunks over the same TaskWatcherConflictTarget enum Upsert takes. Inputs resolving to one target are deduped before the statement is built, last occurrence wins. Earlier chunks are not rolled back on a later failure — wrap in a transaction for atomicity.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."task_watchers" (<columns>) VALUES <values> ON CONFLICT (<conflict_target>) DO UPDATE SET <excluded>
```

### Update

- Params: `pk TaskWatcherPK`, `input *UpdateTaskWatcherInput`
- Returns: `*TaskWatcher`, `error`
- Errors: `ErrNotFound`, `ErrNilInput`, `ErrConstraintViolation`
- Notes: Only fields where IsSet() reports true reach the SET clause; an input with none set issues no UPDATE and returns the row unchanged.

Generated SQL:

postgres:

```sql
UPDATE "public"."task_watchers" SET <set> WHERE "task_id" = $1 AND "user_id" = $2
```

### UpdateMany

- Params: `items []UpdateTaskWatcherItem`
- Returns: `[]*TaskWatcher`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`
- Notes: Issues one statement per item. Batched like CreateMany — earlier items are not rolled back on a later failure. A primary key that does not exist is silently skipped, even under strict_updates.

Generated SQL:

postgres:

```sql
UPDATE "public"."task_watchers" SET <set> WHERE "task_id" = $1 AND "user_id" = $2
```

### UpdateWhere

- Params: `filter *TaskWatcherFilter`, `input *UpdateTaskWatcherInput`
- Returns: `[]*TaskWatcher`, `error`
- Errors: `ErrNilInput`, `ErrEmptyFilter`, `ErrConstraintViolation`
- Notes: Idempotent — returns an empty slice when no row matches. ErrEmptyFilter when the filter produces no conditions.

Generated SQL:

postgres:

```sql
UPDATE "public"."task_watchers" SET <set> WHERE <filter> RETURNING "task_id", "user_id"
```

### HardDelete

- Params: `pk TaskWatcherPK`
- Returns: `error`
- Notes: Idempotent — a primary key that does not exist is not an error.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."task_watchers" WHERE "task_id" = $1 AND "user_id" = $2
```

### HardDeleteMany

- Params: `pks []TaskWatcherPK`
- Returns: `error`
- Notes: Idempotent — primary keys that do not exist are not an error.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."task_watchers" WHERE ("task_id", "user_id") IN (<pks>)
```

### HardDeleteWhere

- Params: `filter *TaskWatcherFilter`
- Returns: `error`
- Errors: `ErrEmptyFilter`
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
| And | `[]*TaskWatcherFilter` |
| Or | `[]*TaskWatcherFilter` |

## Sort

Type `TaskWatcherSort`.

Fields: CreatedAt, TaskID, UserID
