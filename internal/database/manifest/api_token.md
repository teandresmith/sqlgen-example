# APIToken

- **Table:** `api_tokens` (schema `public`)
- **Kind:** table

API access tokens, personal or service-account scoped

## Files

- `api_token_gen.go`

## Primary key

- Kind: single
- `id` — ID `uuid.UUID`

## Columns

| Name | Go field | Go type | DB type | Null | PK | Unique | Default | Comparator | Comment |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `id` | ID | `uuid.UUID` | `uuid` |  | yes |  | `gen_random_uuid()` | `comparator.ID` | Unique identifier |
| `workspace_id` | WorkspaceID | `uuid.UUID` | `uuid` |  |  |  |  | `comparator.ID` | Owning workspace (tenant column) |
| `user_id` | UserID | `*uuid.UUID` | `uuid` | yes |  |  |  | `comparator.NullableID` | Owning user; NULL for service-account tokens |
| `name` | Name | `string` | `text` |  |  |  |  | `comparator.String` | Human-readable token label |
| `token_hash` | TokenHash | `string` | `text` |  |  | yes |  | `comparator.String` | Hashed token secret (never stored in plaintext) |
| `scopes` | Scopes | `[]string` | `text[]` |  |  |  | `'{}'` | `comparator.Slice[string]` | Granted permission scopes |
| `last_used_at` | LastUsedAt | `*time.Time` | `timestamptz` | yes |  |  |  | `comparator.NullableTime` | Timestamp of the most recent use |
| `expires_at` | ExpiresAt | `*time.Time` | `timestamptz` | yes |  |  |  | `comparator.NullableTime` | Optional expiry; NULL never expires |
| `created_at` | CreatedAt | `time.Time` | `timestamptz` |  |  |  | `now()` | `comparator.Time` | Row creation timestamp |

## Indexes

| Name | Columns | Unique | Method | Where |
| --- | --- | --- | --- | --- |
| `api_tokens_pkey` | id | yes | btree |  |
| `api_tokens_token_hash_key` | token_hash | yes | btree |  |
| `idx_api_tokens_workspace` | workspace_id |  | btree |  |

## Query methods

### Get

- Params: `id uuid.UUID`
- Returns: `*APIToken`, `error`
- Errors: `ErrNotFound`, `tenancy.ErrMissing`
- Notes: Delegates to GetMany with a primary-key filter; returns ErrNotFound when no row matches.

Generated SQL:

postgres:

```sql
SELECT "created_at", "expires_at", "id", "last_used_at", "name", "scopes", "token_hash", "user_id", "workspace_id" FROM "public"."api_tokens" WHERE "id" = $1 LIMIT 1
```

### GetMany

- Params: `input *GetAPITokensInput`
- Returns: `[]*APIToken`, `error`
- Errors: `tenancy.ErrMissing`

Generated SQL:

postgres:

```sql
SELECT "created_at", "expires_at", "id", "last_used_at", "name", "scopes", "token_hash", "user_id", "workspace_id" FROM "public"."api_tokens" WHERE <filter> ORDER BY <sort> LIMIT <limit>
```

### Count

- Params: `filter *APITokenFilter`
- Returns: `int64`, `error`
- Errors: `tenancy.ErrMissing`

Generated SQL:

postgres:

```sql
SELECT COUNT(*) FROM "public"."api_tokens" WHERE <filter>
```

### Exists

- Params: `id uuid.UUID`
- Returns: `bool`, `error`
- Errors: `tenancy.ErrMissing`

Generated SQL:

postgres:

```sql
SELECT EXISTS(SELECT 1 FROM "public"."api_tokens" WHERE "id" = $1)
```

### ExistsWhere

- Params: `filter *APITokenFilter`
- Returns: `bool`, `error`
- Errors: `tenancy.ErrMissing`

Generated SQL:

postgres:

```sql
SELECT EXISTS(SELECT 1 FROM "public"."api_tokens" WHERE <filter>)
```

### Paginate

- Params: `input PaginateInput[APITokenFilter]`
- Returns: `*PaginateResult[APIToken]`, `error`
- Errors: `tenancy.ErrMissing`
- Notes: Offset pagination. Issues no statement of its own — composes Count and GetMany, so no sql_bodies entry.

### Connection

- Params: `input ConnectionInput[APITokenFilter]`
- Returns: `*Connection[APIToken]`, `error`
- Errors: `ErrInvalidCursor`, `tenancy.ErrMissing`
- Notes: Relay cursor pagination. Issues no statement of its own — composes Count and GetMany, so no sql_bodies entry.

### Stream

- Params: `input *StreamAPITokensInput`
- Returns: `iter.Seq2[*APIToken, error]`, `error`
- Errors: `tenancy.ErrMissing`
- Notes: Scalars only — no relationship loading, cache always bypassed. Errors surface through the iterator's second value.

Generated SQL:

postgres:

```sql
SELECT "created_at", "expires_at", "id", "last_used_at", "name", "scopes", "token_hash", "user_id", "workspace_id" FROM "public"."api_tokens" WHERE <filter> ORDER BY <sort>
```

## Mutation methods

### Create

- Params: `input *CreateAPITokenInput`
- Returns: `*APIToken`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`

Generated SQL:

postgres:

```sql
INSERT INTO "public"."api_tokens" (<columns>) VALUES (<values>) RETURNING "id"
```

### CreateMany

- Params: `inputs []*CreateAPITokenInput`
- Returns: `[]*APIToken`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Batched in generation.batch_size chunks. Earlier chunks are not rolled back on a later failure — wrap in a transaction for atomicity.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."api_tokens" (<columns>) VALUES <values> RETURNING "id"
```

### Upsert

- Params: `input *CreateAPITokenInput`, `target APITokenConflictTarget`
- Returns: `*APIToken`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: One method over the generated APITokenConflictTarget enum — the target argument selects the conflict columns at call time.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."api_tokens" (<columns>) VALUES (<values>) ON CONFLICT (<conflict_target>) DO UPDATE SET <excluded> RETURNING "id"
```

### UpsertMany

- Params: `inputs []*CreateAPITokenInput`, `target APITokenConflictTarget`
- Returns: `[]*APIToken`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Batched in generation.batch_size chunks over the same APITokenConflictTarget enum Upsert takes. Inputs resolving to one target are deduped before the statement is built, last occurrence wins. Earlier chunks are not rolled back on a later failure — wrap in a transaction for atomicity.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."api_tokens" (<columns>) VALUES <values> ON CONFLICT (<conflict_target>) DO UPDATE SET <excluded>
```

### Update

- Params: `id uuid.UUID`, `input *UpdateAPITokenInput`
- Returns: `*APIToken`, `error`
- Errors: `ErrNotFound`, `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Only fields where IsSet() reports true reach the SET clause; an input with none set issues no UPDATE and returns the row unchanged.

Generated SQL:

postgres:

```sql
UPDATE "public"."api_tokens" SET <set> WHERE "id" = $1
```

### UpdateMany

- Params: `items []UpdateAPITokenItem`
- Returns: `[]*APIToken`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Issues one statement per item. Batched like CreateMany — earlier items are not rolled back on a later failure. A primary key that does not exist is silently skipped, even under strict_updates.

Generated SQL:

postgres:

```sql
UPDATE "public"."api_tokens" SET <set> WHERE "id" = $1
```

### UpdateWhere

- Params: `filter *APITokenFilter`, `input *UpdateAPITokenInput`
- Returns: `[]*APIToken`, `error`
- Errors: `ErrNilInput`, `ErrEmptyFilter`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Idempotent — returns an empty slice when no row matches. ErrEmptyFilter when the filter produces no conditions.

Generated SQL:

postgres:

```sql
UPDATE "public"."api_tokens" SET <set> WHERE <filter> RETURNING "id"
```

### HardDelete

- Params: `id uuid.UUID`
- Returns: `error`
- Errors: `tenancy.ErrMissing`
- Notes: Idempotent — a primary key that does not exist is not an error.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."api_tokens" WHERE "id" = $1
```

### HardDeleteMany

- Params: `ids []uuid.UUID`
- Returns: `error`
- Errors: `tenancy.ErrMissing`
- Notes: Idempotent — primary keys that do not exist are not an error.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."api_tokens" WHERE "id" IN (<ids>)
```

### HardDeleteWhere

- Params: `filter *APITokenFilter`
- Returns: `error`
- Errors: `ErrEmptyFilter`, `tenancy.ErrMissing`
- Notes: Idempotent — no match is not an error. ErrEmptyFilter when the filter produces no conditions.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."api_tokens" WHERE <filter> RETURNING "id"
```

## Filter

Type `APITokenFilter`.

| Field | Type |
| --- | --- |
| CreatedAt | `*comparator.Time` |
| ExpiresAt | `*comparator.NullableTime` |
| ID | `*comparator.ID` |
| LastUsedAt | `*comparator.NullableTime` |
| Name | `*comparator.String` |
| Scopes | `*comparator.Slice[string]` |
| TokenHash | `*comparator.String` |
| UserID | `*comparator.NullableID` |
| WorkspaceID | `*comparator.ID` |
| And | `[]*APITokenFilter` |
| Or | `[]*APITokenFilter` |

## Sort

Type `APITokenSort`.

Fields: CreatedAt, ExpiresAt, ID, LastUsedAt, Name, Scopes, TokenHash, UserID, WorkspaceID
