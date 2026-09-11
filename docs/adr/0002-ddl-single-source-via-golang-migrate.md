# DDL migration files are the single source of truth, applied by golang-migrate

The schema lives as numbered `NNNN_name.up.sql` / `.down.sql` files under
`./migrations`. `golang-migrate` applies them to Postgres, and the *same* files
are `sqlgen`'s parse input — so the schema the code is generated from can never
drift from the schema running in the database. We chose `golang-migrate` over
`goose` specifically because sqlgen's parser auto-skips `*.down.sql` files while
keeping `*.up.sql`, which matches golang-migrate's separate up/down file naming
exactly; goose puts both directions in one annotated file, so sqlgen would parse
the down statements too.
