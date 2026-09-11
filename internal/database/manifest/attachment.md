# Attachment

- **Table:** `attachments` (schema `public`)
- **Kind:** table

Files attached to a task or a comment (polymorphic)

## Files

- `attachment_gen.go`

## Primary key

- Kind: single
- `id` — ID `uuid.UUID`

## Columns

| Name | Go field | Go type | DB type | Null | PK | Unique | Default | Comparator | Comment |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `id` | ID | `uuid.UUID` | `uuid` |  | yes |  | `gen_random_uuid()` | `comparator.ID` | Unique identifier |
| `workspace_id` | WorkspaceID | `uuid.UUID` | `uuid` |  |  |  |  | `comparator.ID` | Owning workspace (tenant column) |
| `entity_type` | EntityType | `AttachmentEntityType` | `attachment_entity_type` |  |  |  |  | `comparator.Enum[AttachmentEntityType]` | Which entity kind this attachment is bound to |
| `entity_id` | EntityID | `uuid.UUID` | `uuid` |  |  |  |  | `comparator.ID` | Primary key of the bound entity (task or comment) |
| `uploaded_by` | UploadedBy | `uuid.UUID` | `uuid` |  |  |  |  | `comparator.ID` | User who uploaded the file |
| `file_name` | FileName | `string` | `text` |  |  |  |  | `comparator.String` | Original file name |
| `content_type` | ContentType | `string` | `text` |  |  |  |  | `comparator.String` | MIME type |
| `size_bytes` | SizeBytes | `int64` | `bigint` |  |  |  |  | `comparator.Number[int64]` | File size in bytes |
| `storage_url` | StorageURL | `string` | `text` |  |  |  |  | `comparator.String` | Location of the stored blob |
| `created_at` | CreatedAt | `time.Time` | `timestamptz` |  |  |  | `now()` | `comparator.Time` | Row creation timestamp |

## Indexes

| Name | Columns | Unique | Method | Where |
| --- | --- | --- | --- | --- |
| `attachments_pkey` | id | yes | btree |  |
| `idx_attachments_entity` | entity_type, entity_id |  | btree |  |

## Query methods

### Get

- Params: `id uuid.UUID`
- Returns: `*Attachment`, `error`
- Errors: `ErrNotFound`, `tenancy.ErrMissing`
- Notes: Delegates to GetMany with a primary-key filter; returns ErrNotFound when no row matches.

Generated SQL:

postgres:

```sql
SELECT "content_type", "created_at", "entity_id", "entity_type", "file_name", "id", "size_bytes", "storage_url", "uploaded_by", "workspace_id" FROM "public"."attachments" WHERE "id" = $1 LIMIT 1
```

### GetMany

- Params: `input *GetAttachmentsInput`
- Returns: `[]*Attachment`, `error`
- Errors: `tenancy.ErrMissing`

Generated SQL:

postgres:

```sql
SELECT "content_type", "created_at", "entity_id", "entity_type", "file_name", "id", "size_bytes", "storage_url", "uploaded_by", "workspace_id" FROM "public"."attachments" WHERE <filter> ORDER BY <sort> LIMIT <limit>
```

### Count

- Params: `filter *AttachmentFilter`
- Returns: `int64`, `error`
- Errors: `tenancy.ErrMissing`

Generated SQL:

postgres:

```sql
SELECT COUNT(*) FROM "public"."attachments" WHERE <filter>
```

### Exists

- Params: `id uuid.UUID`
- Returns: `bool`, `error`
- Errors: `tenancy.ErrMissing`

Generated SQL:

postgres:

```sql
SELECT EXISTS(SELECT 1 FROM "public"."attachments" WHERE "id" = $1)
```

### ExistsWhere

- Params: `filter *AttachmentFilter`
- Returns: `bool`, `error`
- Errors: `tenancy.ErrMissing`

Generated SQL:

postgres:

```sql
SELECT EXISTS(SELECT 1 FROM "public"."attachments" WHERE <filter>)
```

### Paginate

- Params: `input PaginateInput[AttachmentFilter]`
- Returns: `*PaginateResult[Attachment]`, `error`
- Errors: `tenancy.ErrMissing`
- Notes: Offset pagination. Issues no statement of its own — composes Count and GetMany, so no sql_bodies entry.

### Connection

- Params: `input ConnectionInput[AttachmentFilter]`
- Returns: `*Connection[Attachment]`, `error`
- Errors: `ErrInvalidCursor`, `tenancy.ErrMissing`
- Notes: Relay cursor pagination. Issues no statement of its own — composes Count and GetMany, so no sql_bodies entry.

### Stream

- Params: `input *StreamAttachmentsInput`
- Returns: `iter.Seq2[*Attachment, error]`, `error`
- Errors: `tenancy.ErrMissing`
- Notes: Scalars only — no relationship loading, cache always bypassed. Errors surface through the iterator's second value.

Generated SQL:

postgres:

```sql
SELECT "content_type", "created_at", "entity_id", "entity_type", "file_name", "id", "size_bytes", "storage_url", "uploaded_by", "workspace_id" FROM "public"."attachments" WHERE <filter> ORDER BY <sort>
```

## Mutation methods

### Create

- Params: `input *CreateAttachmentInput`
- Returns: `*Attachment`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`

Generated SQL:

postgres:

```sql
INSERT INTO "public"."attachments" (<columns>) VALUES (<values>) RETURNING "id"
```

### CreateMany

- Params: `inputs []*CreateAttachmentInput`
- Returns: `[]*Attachment`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Batched in generation.batch_size chunks. Earlier chunks are not rolled back on a later failure — wrap in a transaction for atomicity.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."attachments" (<columns>) VALUES <values> RETURNING "id"
```

### Upsert

- Params: `input *CreateAttachmentInput`, `target AttachmentConflictTarget`
- Returns: `*Attachment`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: One method over the generated AttachmentConflictTarget enum — the target argument selects the conflict columns at call time.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."attachments" (<columns>) VALUES (<values>) ON CONFLICT (<conflict_target>) DO UPDATE SET <excluded> RETURNING "id"
```

### UpsertMany

- Params: `inputs []*CreateAttachmentInput`, `target AttachmentConflictTarget`
- Returns: `[]*Attachment`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Batched in generation.batch_size chunks over the same AttachmentConflictTarget enum Upsert takes. Inputs resolving to one target are deduped before the statement is built, last occurrence wins. Earlier chunks are not rolled back on a later failure — wrap in a transaction for atomicity.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."attachments" (<columns>) VALUES <values> ON CONFLICT (<conflict_target>) DO UPDATE SET <excluded>
```

### Update

- Params: `id uuid.UUID`, `input *UpdateAttachmentInput`
- Returns: `*Attachment`, `error`
- Errors: `ErrNotFound`, `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Only fields where IsSet() reports true reach the SET clause; an input with none set issues no UPDATE and returns the row unchanged.

Generated SQL:

postgres:

```sql
UPDATE "public"."attachments" SET <set> WHERE "id" = $1
```

### UpdateMany

- Params: `items []UpdateAttachmentItem`
- Returns: `[]*Attachment`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Issues one statement per item. Batched like CreateMany — earlier items are not rolled back on a later failure. A primary key that does not exist is silently skipped, even under strict_updates.

Generated SQL:

postgres:

```sql
UPDATE "public"."attachments" SET <set> WHERE "id" = $1
```

### UpdateWhere

- Params: `filter *AttachmentFilter`, `input *UpdateAttachmentInput`
- Returns: `[]*Attachment`, `error`
- Errors: `ErrNilInput`, `ErrEmptyFilter`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Idempotent — returns an empty slice when no row matches. ErrEmptyFilter when the filter produces no conditions.

Generated SQL:

postgres:

```sql
UPDATE "public"."attachments" SET <set> WHERE <filter> RETURNING "id"
```

### Increment

- Params: `id uuid.UUID`, `input IncrementInput[AttachmentIncrementColumn]`
- Returns: `error`
- Errors: `ErrNotFound`, `tenancy.ErrMissing`
- Notes: Atomic single-column arithmetic. The column is restricted to the generated AttachmentIncrementColumn enum — numeric, and identifying nothing: neither PK, FK, nor the tenant column. A negative amount decrements.

Generated SQL:

postgres:

```sql
UPDATE "public"."attachments" SET <column> = <column> + $1 WHERE "id" = $2
```

### HardDelete

- Params: `id uuid.UUID`
- Returns: `error`
- Errors: `tenancy.ErrMissing`
- Notes: Idempotent — a primary key that does not exist is not an error.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."attachments" WHERE "id" = $1
```

### HardDeleteMany

- Params: `ids []uuid.UUID`
- Returns: `error`
- Errors: `tenancy.ErrMissing`
- Notes: Idempotent — primary keys that do not exist are not an error.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."attachments" WHERE "id" IN (<ids>)
```

### HardDeleteWhere

- Params: `filter *AttachmentFilter`
- Returns: `error`
- Errors: `ErrEmptyFilter`, `tenancy.ErrMissing`
- Notes: Idempotent — no match is not an error. ErrEmptyFilter when the filter produces no conditions.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."attachments" WHERE <filter> RETURNING "id"
```

## Filter

Type `AttachmentFilter`.

| Field | Type |
| --- | --- |
| ContentType | `*comparator.String` |
| CreatedAt | `*comparator.Time` |
| EntityID | `*comparator.ID` |
| EntityType | `*comparator.Enum[AttachmentEntityType]` |
| FileName | `*comparator.String` |
| ID | `*comparator.ID` |
| SizeBytes | `*comparator.Number[int64]` |
| StorageURL | `*comparator.String` |
| UploadedBy | `*comparator.ID` |
| WorkspaceID | `*comparator.ID` |
| And | `[]*AttachmentFilter` |
| Or | `[]*AttachmentFilter` |

## Sort

Type `AttachmentSort`.

Fields: ContentType, CreatedAt, EntityID, EntityType, FileName, ID, SizeBytes, StorageURL, UploadedBy, WorkspaceID
