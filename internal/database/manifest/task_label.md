# TaskLabel

- **Table:** `task_labels` (schema `public`)
- **Kind:** table

Applies labels to tasks (M2M)

## Files

- `task_label_gen.go`

## Primary key

- Kind: composite
- Struct: `TaskLabelPK`
- `task_id` — TaskID `uuid.UUID`
- `label_id` — LabelID `uuid.UUID`

## Columns

| Name | Go field | Go type | DB type | Null | PK | Unique | Default | Comparator | Comment |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `task_id` | TaskID | `uuid.UUID` | `uuid` |  | yes |  |  | `comparator.ID` | Task side of the tagging |
| `label_id` | LabelID | `uuid.UUID` | `uuid` |  | yes |  |  | `comparator.ID` | Applied label |

## Indexes

| Name | Columns | Unique | Method | Where |
| --- | --- | --- | --- | --- |
| `task_labels_pkey` | task_id, label_id | yes | btree |  |

## Query methods

### Get

- Params: `pk TaskLabelPK`
- Returns: `*TaskLabel`, `error`
- Errors: `ErrNotFound`
- Notes: Delegates to GetMany with a primary-key filter; returns ErrNotFound when no row matches.

Generated SQL:

postgres:

```sql
SELECT "label_id", "task_id" FROM "public"."task_labels" WHERE "task_id" = $1 AND "label_id" = $2 LIMIT 1
```

### GetMany

- Params: `input *GetTaskLabelsInput`
- Returns: `[]*TaskLabel`, `error`

Generated SQL:

postgres:

```sql
SELECT "label_id", "task_id" FROM "public"."task_labels" WHERE <filter> ORDER BY <sort> LIMIT <limit>
```

### Count

- Params: `filter *TaskLabelFilter`
- Returns: `int64`, `error`

Generated SQL:

postgres:

```sql
SELECT COUNT(*) FROM "public"."task_labels" WHERE <filter>
```

### Exists

- Params: `pk TaskLabelPK`
- Returns: `bool`, `error`

Generated SQL:

postgres:

```sql
SELECT EXISTS(SELECT 1 FROM "public"."task_labels" WHERE "task_id" = $1 AND "label_id" = $2)
```

### ExistsWhere

- Params: `filter *TaskLabelFilter`
- Returns: `bool`, `error`

Generated SQL:

postgres:

```sql
SELECT EXISTS(SELECT 1 FROM "public"."task_labels" WHERE <filter>)
```

### Paginate

- Params: `input PaginateInput[TaskLabelFilter]`
- Returns: `*PaginateResult[TaskLabel]`, `error`
- Notes: Offset pagination. Issues no statement of its own — composes Count and GetMany, so no sql_bodies entry.

### Connection

- Params: `input ConnectionInput[TaskLabelFilter]`
- Returns: `*Connection[TaskLabel]`, `error`
- Errors: `ErrInvalidCursor`
- Notes: Relay cursor pagination. Issues no statement of its own — composes Count and GetMany, so no sql_bodies entry.

### Stream

- Params: `input *StreamTaskLabelsInput`
- Returns: `iter.Seq2[*TaskLabel, error]`, `error`
- Notes: Scalars only — no relationship loading, cache always bypassed. Errors surface through the iterator's second value.

Generated SQL:

postgres:

```sql
SELECT "label_id", "task_id" FROM "public"."task_labels" WHERE <filter> ORDER BY <sort>
```

## Mutation methods

### Create

- Params: `input *CreateTaskLabelInput`
- Returns: `*TaskLabel`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`

Generated SQL:

postgres:

```sql
INSERT INTO "public"."task_labels" (<columns>) VALUES (<values>)
```

### CreateMany

- Params: `inputs []*CreateTaskLabelInput`
- Returns: `[]*TaskLabel`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`
- Notes: Batched in generation.batch_size chunks. Earlier chunks are not rolled back on a later failure — wrap in a transaction for atomicity.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."task_labels" (<columns>) VALUES <values>
```

### Upsert

- Params: `input *CreateTaskLabelInput`, `target TaskLabelConflictTarget`
- Returns: `*TaskLabel`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`
- Notes: One method over the generated TaskLabelConflictTarget enum — the target argument selects the conflict columns at call time.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."task_labels" (<columns>) VALUES (<values>) ON CONFLICT (<conflict_target>) DO UPDATE SET <excluded>
```

### UpsertMany

- Params: `inputs []*CreateTaskLabelInput`, `target TaskLabelConflictTarget`
- Returns: `[]*TaskLabel`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`
- Notes: Batched in generation.batch_size chunks over the same TaskLabelConflictTarget enum Upsert takes. Inputs resolving to one target are deduped before the statement is built, last occurrence wins. Earlier chunks are not rolled back on a later failure — wrap in a transaction for atomicity.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."task_labels" (<columns>) VALUES <values> ON CONFLICT (<conflict_target>) DO UPDATE SET <excluded>
```

### Update

- Params: `pk TaskLabelPK`, `input *UpdateTaskLabelInput`
- Returns: `*TaskLabel`, `error`
- Errors: `ErrNotFound`, `ErrNilInput`, `ErrConstraintViolation`
- Notes: Only fields where IsSet() reports true reach the SET clause; an input with none set issues no UPDATE and returns the row unchanged.

Generated SQL:

postgres:

```sql
UPDATE "public"."task_labels" SET <set> WHERE "task_id" = $1 AND "label_id" = $2
```

### UpdateMany

- Params: `items []UpdateTaskLabelItem`
- Returns: `[]*TaskLabel`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`
- Notes: Issues one statement per item. Batched like CreateMany — earlier items are not rolled back on a later failure. A primary key that does not exist is silently skipped, even under strict_updates.

Generated SQL:

postgres:

```sql
UPDATE "public"."task_labels" SET <set> WHERE "task_id" = $1 AND "label_id" = $2
```

### UpdateWhere

- Params: `filter *TaskLabelFilter`, `input *UpdateTaskLabelInput`
- Returns: `[]*TaskLabel`, `error`
- Errors: `ErrNilInput`, `ErrEmptyFilter`, `ErrConstraintViolation`
- Notes: Idempotent — returns an empty slice when no row matches. ErrEmptyFilter when the filter produces no conditions.

Generated SQL:

postgres:

```sql
UPDATE "public"."task_labels" SET <set> WHERE <filter> RETURNING "task_id", "label_id"
```

### HardDelete

- Params: `pk TaskLabelPK`
- Returns: `error`
- Notes: Idempotent — a primary key that does not exist is not an error.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."task_labels" WHERE "task_id" = $1 AND "label_id" = $2
```

### HardDeleteMany

- Params: `pks []TaskLabelPK`
- Returns: `error`
- Notes: Idempotent — primary keys that do not exist are not an error.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."task_labels" WHERE ("task_id", "label_id") IN (<pks>)
```

### HardDeleteWhere

- Params: `filter *TaskLabelFilter`
- Returns: `error`
- Errors: `ErrEmptyFilter`
- Notes: Idempotent — no match is not an error. ErrEmptyFilter when the filter produces no conditions.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."task_labels" WHERE <filter> RETURNING "task_id", "label_id"
```

## Filter

Type `TaskLabelFilter`.

| Field | Type |
| --- | --- |
| LabelID | `*comparator.ID` |
| TaskID | `*comparator.ID` |
| And | `[]*TaskLabelFilter` |
| Or | `[]*TaskLabelFilter` |

## Sort

Type `TaskLabelSort`.

Fields: LabelID, TaskID
