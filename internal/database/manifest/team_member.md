# TeamMember

- **Table:** `team_members` (schema `public`)
- **Kind:** table

Membership of users in teams (M2M)

## Files

- `team_member_gen.go`

## Primary key

- Kind: composite
- Struct: `TeamMemberPK`
- `team_id` — TeamID `uuid.UUID`
- `user_id` — UserID `uuid.UUID`

## Columns

| Name | Go field | Go type | DB type | Null | PK | Unique | Default | Comparator | Comment |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `team_id` | TeamID | `uuid.UUID` | `uuid` |  | yes |  |  | `comparator.ID` | Team side of the membership |
| `user_id` | UserID | `uuid.UUID` | `uuid` |  | yes |  |  | `comparator.ID` | User side of the membership |
| `added_at` | AddedAt | `time.Time` | `timestamptz` |  |  |  | `now()` | `comparator.Time` | When the user joined the team |

## Indexes

| Name | Columns | Unique | Method | Where |
| --- | --- | --- | --- | --- |
| `idx_team_members_user` | user_id |  | btree |  |
| `team_members_pkey` | team_id, user_id | yes | btree |  |

## Query methods

### Get

- Params: `pk TeamMemberPK`
- Returns: `*TeamMember`, `error`
- Errors: `ErrNotFound`
- Notes: Delegates to GetMany with a primary-key filter; returns ErrNotFound when no row matches.

Generated SQL:

postgres:

```sql
SELECT "added_at", "team_id", "user_id" FROM "public"."team_members" WHERE "team_id" = $1 AND "user_id" = $2 LIMIT 1
```

### GetMany

- Params: `input *GetTeamMembersInput`
- Returns: `[]*TeamMember`, `error`

Generated SQL:

postgres:

```sql
SELECT "added_at", "team_id", "user_id" FROM "public"."team_members" WHERE <filter> ORDER BY <sort> LIMIT <limit>
```

### Count

- Params: `filter *TeamMemberFilter`
- Returns: `int64`, `error`

Generated SQL:

postgres:

```sql
SELECT COUNT(*) FROM "public"."team_members" WHERE <filter>
```

### Exists

- Params: `pk TeamMemberPK`
- Returns: `bool`, `error`

Generated SQL:

postgres:

```sql
SELECT EXISTS(SELECT 1 FROM "public"."team_members" WHERE "team_id" = $1 AND "user_id" = $2)
```

### ExistsWhere

- Params: `filter *TeamMemberFilter`
- Returns: `bool`, `error`

Generated SQL:

postgres:

```sql
SELECT EXISTS(SELECT 1 FROM "public"."team_members" WHERE <filter>)
```

### Paginate

- Params: `input PaginateInput[TeamMemberFilter]`
- Returns: `*PaginateResult[TeamMember]`, `error`
- Notes: Offset pagination. Issues no statement of its own — composes Count and GetMany, so no sql_bodies entry.

### Connection

- Params: `input ConnectionInput[TeamMemberFilter]`
- Returns: `*Connection[TeamMember]`, `error`
- Errors: `ErrInvalidCursor`
- Notes: Relay cursor pagination. Issues no statement of its own — composes Count and GetMany, so no sql_bodies entry.

### Stream

- Params: `input *StreamTeamMembersInput`
- Returns: `iter.Seq2[*TeamMember, error]`, `error`
- Notes: Scalars only — no relationship loading, cache always bypassed. Errors surface through the iterator's second value.

Generated SQL:

postgres:

```sql
SELECT "added_at", "team_id", "user_id" FROM "public"."team_members" WHERE <filter> ORDER BY <sort>
```

## Mutation methods

### Create

- Params: `input *CreateTeamMemberInput`
- Returns: `*TeamMember`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`

Generated SQL:

postgres:

```sql
INSERT INTO "public"."team_members" (<columns>) VALUES (<values>)
```

### CreateMany

- Params: `inputs []*CreateTeamMemberInput`
- Returns: `[]*TeamMember`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`
- Notes: Batched in generation.batch_size chunks. Earlier chunks are not rolled back on a later failure — wrap in a transaction for atomicity.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."team_members" (<columns>) VALUES <values>
```

### Upsert

- Params: `input *CreateTeamMemberInput`, `target TeamMemberConflictTarget`
- Returns: `*TeamMember`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`
- Notes: One method over the generated TeamMemberConflictTarget enum — the target argument selects the conflict columns at call time.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."team_members" (<columns>) VALUES (<values>) ON CONFLICT (<conflict_target>) DO UPDATE SET <excluded>
```

### UpsertMany

- Params: `inputs []*CreateTeamMemberInput`, `target TeamMemberConflictTarget`
- Returns: `[]*TeamMember`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`
- Notes: Batched in generation.batch_size chunks over the same TeamMemberConflictTarget enum Upsert takes. Inputs resolving to one target are deduped before the statement is built, last occurrence wins. Earlier chunks are not rolled back on a later failure — wrap in a transaction for atomicity.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."team_members" (<columns>) VALUES <values> ON CONFLICT (<conflict_target>) DO UPDATE SET <excluded>
```

### Update

- Params: `pk TeamMemberPK`, `input *UpdateTeamMemberInput`
- Returns: `*TeamMember`, `error`
- Errors: `ErrNotFound`, `ErrNilInput`, `ErrConstraintViolation`
- Notes: Only fields where IsSet() reports true reach the SET clause; an input with none set issues no UPDATE and returns the row unchanged.

Generated SQL:

postgres:

```sql
UPDATE "public"."team_members" SET <set> WHERE "team_id" = $1 AND "user_id" = $2
```

### UpdateMany

- Params: `items []UpdateTeamMemberItem`
- Returns: `[]*TeamMember`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`
- Notes: Issues one statement per item. Batched like CreateMany — earlier items are not rolled back on a later failure. A primary key that does not exist is silently skipped, even under strict_updates.

Generated SQL:

postgres:

```sql
UPDATE "public"."team_members" SET <set> WHERE "team_id" = $1 AND "user_id" = $2
```

### UpdateWhere

- Params: `filter *TeamMemberFilter`, `input *UpdateTeamMemberInput`
- Returns: `[]*TeamMember`, `error`
- Errors: `ErrNilInput`, `ErrEmptyFilter`, `ErrConstraintViolation`
- Notes: Idempotent — returns an empty slice when no row matches. ErrEmptyFilter when the filter produces no conditions.

Generated SQL:

postgres:

```sql
UPDATE "public"."team_members" SET <set> WHERE <filter> RETURNING "team_id", "user_id"
```

### HardDelete

- Params: `pk TeamMemberPK`
- Returns: `error`
- Notes: Idempotent — a primary key that does not exist is not an error.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."team_members" WHERE "team_id" = $1 AND "user_id" = $2
```

### HardDeleteMany

- Params: `pks []TeamMemberPK`
- Returns: `error`
- Notes: Idempotent — primary keys that do not exist are not an error.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."team_members" WHERE ("team_id", "user_id") IN (<pks>)
```

### HardDeleteWhere

- Params: `filter *TeamMemberFilter`
- Returns: `error`
- Errors: `ErrEmptyFilter`
- Notes: Idempotent — no match is not an error. ErrEmptyFilter when the filter produces no conditions.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."team_members" WHERE <filter> RETURNING "team_id", "user_id"
```

## Filter

Type `TeamMemberFilter`.

| Field | Type |
| --- | --- |
| AddedAt | `*comparator.Time` |
| TeamID | `*comparator.ID` |
| UserID | `*comparator.ID` |
| And | `[]*TeamMemberFilter` |
| Or | `[]*TeamMemberFilter` |

## Sort

Type `TeamMemberSort`.

Fields: AddedAt, TeamID, UserID
