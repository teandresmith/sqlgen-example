# SsoConnection

- **Table:** `sso_connections` (schema `public`)
- **Kind:** table

Per-workspace single sign-on configuration

## Files

- `sso_connection_gen.go`

## Primary key

- Kind: single
- `id` — ID `uuid.UUID`

## Columns

| Name | Go field | Go type | DB type | Null | PK | Unique | Default | Comparator | Comment |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `id` | ID | `uuid.UUID` | `uuid` |  | yes |  | `gen_random_uuid()` | `comparator.ID` | Unique identifier |
| `workspace_id` | WorkspaceID | `uuid.UUID` | `uuid` |  |  |  |  | `comparator.ID` | Owning workspace (tenant column) |
| `provider` | Provider | `SsoProvider` | `sso_provider` |  |  |  |  | `comparator.Enum[SsoProvider]` | Identity provider protocol/vendor |
| `config` | Config | `types.JSON` | `jsonb` |  |  |  | `'{}'::jsonb` | `comparator.JSONB` | Provider-specific configuration (metadata URL, client IDs, etc.) |
| `enabled` | Enabled | `bool` | `boolean` |  |  |  | `false` | `comparator.Bool` | Whether this connection is active |
| `created_at` | CreatedAt | `time.Time` | `timestamptz` |  |  |  | `now()` | `comparator.Time` | Row creation timestamp |
| `updated_at` | UpdatedAt | `time.Time` | `timestamptz` |  |  |  | `now()` | `comparator.Time` | Last modification timestamp |

## Indexes

| Name | Columns | Unique | Method | Where |
| --- | --- | --- | --- | --- |
| `sso_connections_pkey` | id | yes | btree |  |
| `sso_connections_workspace_id_provider_idx` | workspace_id, provider | yes | btree |  |

## Query methods

### Get

- Params: `id uuid.UUID`
- Returns: `*SsoConnection`, `error`
- Errors: `ErrNotFound`, `tenancy.ErrMissing`
- Notes: Delegates to GetMany with a primary-key filter; returns ErrNotFound when no row matches.

Generated SQL:

postgres:

```sql
SELECT "config", "created_at", "enabled", "id", "provider", "updated_at", "workspace_id" FROM "public"."sso_connections" WHERE "id" = $1 LIMIT 1
```

### GetMany

- Params: `input *GetSsoConnectionsInput`
- Returns: `[]*SsoConnection`, `error`
- Errors: `tenancy.ErrMissing`

Generated SQL:

postgres:

```sql
SELECT "config", "created_at", "enabled", "id", "provider", "updated_at", "workspace_id" FROM "public"."sso_connections" WHERE <filter> ORDER BY <sort> LIMIT <limit>
```

### Count

- Params: `filter *SsoConnectionFilter`
- Returns: `int64`, `error`
- Errors: `tenancy.ErrMissing`

Generated SQL:

postgres:

```sql
SELECT COUNT(*) FROM "public"."sso_connections" WHERE <filter>
```

### Exists

- Params: `id uuid.UUID`
- Returns: `bool`, `error`
- Errors: `tenancy.ErrMissing`

Generated SQL:

postgres:

```sql
SELECT EXISTS(SELECT 1 FROM "public"."sso_connections" WHERE "id" = $1)
```

### ExistsWhere

- Params: `filter *SsoConnectionFilter`
- Returns: `bool`, `error`
- Errors: `tenancy.ErrMissing`

Generated SQL:

postgres:

```sql
SELECT EXISTS(SELECT 1 FROM "public"."sso_connections" WHERE <filter>)
```

### Paginate

- Params: `input PaginateInput[SsoConnectionFilter]`
- Returns: `*PaginateResult[SsoConnection]`, `error`
- Errors: `tenancy.ErrMissing`
- Notes: Offset pagination. Issues no statement of its own — composes Count and GetMany, so no sql_bodies entry.

### Connection

- Params: `input ConnectionInput[SsoConnectionFilter]`
- Returns: `*Connection[SsoConnection]`, `error`
- Errors: `ErrInvalidCursor`, `tenancy.ErrMissing`
- Notes: Relay cursor pagination. Issues no statement of its own — composes Count and GetMany, so no sql_bodies entry.

### Stream

- Params: `input *StreamSsoConnectionsInput`
- Returns: `iter.Seq2[*SsoConnection, error]`, `error`
- Errors: `tenancy.ErrMissing`
- Notes: Scalars only — no relationship loading, cache always bypassed. Errors surface through the iterator's second value.

Generated SQL:

postgres:

```sql
SELECT "config", "created_at", "enabled", "id", "provider", "updated_at", "workspace_id" FROM "public"."sso_connections" WHERE <filter> ORDER BY <sort>
```

## Mutation methods

### Create

- Params: `input *CreateSsoConnectionInput`
- Returns: `*SsoConnection`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`

Generated SQL:

postgres:

```sql
INSERT INTO "public"."sso_connections" (<columns>) VALUES (<values>) RETURNING "id"
```

### CreateMany

- Params: `inputs []*CreateSsoConnectionInput`
- Returns: `[]*SsoConnection`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Batched in generation.batch_size chunks. Earlier chunks are not rolled back on a later failure — wrap in a transaction for atomicity.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."sso_connections" (<columns>) VALUES <values> RETURNING "id"
```

### Upsert

- Params: `input *CreateSsoConnectionInput`, `target SsoConnectionConflictTarget`
- Returns: `*SsoConnection`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: One method over the generated SsoConnectionConflictTarget enum — the target argument selects the conflict columns at call time.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."sso_connections" (<columns>) VALUES (<values>) ON CONFLICT (<conflict_target>) DO UPDATE SET <excluded> RETURNING "id"
```

### UpsertMany

- Params: `inputs []*CreateSsoConnectionInput`, `target SsoConnectionConflictTarget`
- Returns: `[]*SsoConnection`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Batched in generation.batch_size chunks over the same SsoConnectionConflictTarget enum Upsert takes. Inputs resolving to one target are deduped before the statement is built, last occurrence wins. Earlier chunks are not rolled back on a later failure — wrap in a transaction for atomicity.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."sso_connections" (<columns>) VALUES <values> ON CONFLICT (<conflict_target>) DO UPDATE SET <excluded>
```

### Update

- Params: `id uuid.UUID`, `input *UpdateSsoConnectionInput`
- Returns: `*SsoConnection`, `error`
- Errors: `ErrNotFound`, `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Only fields where IsSet() reports true reach the SET clause; an input with none set issues no UPDATE and returns the row unchanged.

Generated SQL:

postgres:

```sql
UPDATE "public"."sso_connections" SET <set> WHERE "id" = $1
```

### UpdateMany

- Params: `items []UpdateSsoConnectionItem`
- Returns: `[]*SsoConnection`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Issues one statement per item. Batched like CreateMany — earlier items are not rolled back on a later failure. A primary key that does not exist is silently skipped, even under strict_updates.

Generated SQL:

postgres:

```sql
UPDATE "public"."sso_connections" SET <set> WHERE "id" = $1
```

### UpdateWhere

- Params: `filter *SsoConnectionFilter`, `input *UpdateSsoConnectionInput`
- Returns: `[]*SsoConnection`, `error`
- Errors: `ErrNilInput`, `ErrEmptyFilter`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Idempotent — returns an empty slice when no row matches. ErrEmptyFilter when the filter produces no conditions.

Generated SQL:

postgres:

```sql
UPDATE "public"."sso_connections" SET <set> WHERE <filter> RETURNING "id"
```

### HardDelete

- Params: `id uuid.UUID`
- Returns: `error`
- Errors: `tenancy.ErrMissing`
- Notes: Idempotent — a primary key that does not exist is not an error.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."sso_connections" WHERE "id" = $1
```

### HardDeleteMany

- Params: `ids []uuid.UUID`
- Returns: `error`
- Errors: `tenancy.ErrMissing`
- Notes: Idempotent — primary keys that do not exist are not an error.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."sso_connections" WHERE "id" IN (<ids>)
```

### HardDeleteWhere

- Params: `filter *SsoConnectionFilter`
- Returns: `error`
- Errors: `ErrEmptyFilter`, `tenancy.ErrMissing`
- Notes: Idempotent — no match is not an error. ErrEmptyFilter when the filter produces no conditions.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."sso_connections" WHERE <filter> RETURNING "id"
```

## Filter

Type `SsoConnectionFilter`.

| Field | Type |
| --- | --- |
| Config | `*comparator.JSONB` |
| CreatedAt | `*comparator.Time` |
| Enabled | `*comparator.Bool` |
| ID | `*comparator.ID` |
| Provider | `*comparator.Enum[SsoProvider]` |
| UpdatedAt | `*comparator.Time` |
| WorkspaceID | `*comparator.ID` |
| And | `[]*SsoConnectionFilter` |
| Or | `[]*SsoConnectionFilter` |

## Sort

Type `SsoConnectionSort`.

Fields: Config, CreatedAt, Enabled, ID, Provider, UpdatedAt, WorkspaceID
