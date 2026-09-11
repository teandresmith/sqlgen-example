# Adopter-first scope, verified by real integration (not edge-case maximalism)

This project has two goals — be a functional test bed for `sqlgen` and be an
example for adopters — which conflict when they pull toward exhaustive
edge-case coverage vs. a coherent, idiomatic app. We resolve it **adopter-first**:
build a realistic multi-tenant SaaS a newcomer would copy, and verify it by
running it end-to-end against live Postgres + a live GraphQL server. We
deliberately do **not** chase every feature/edge case, because sqlgen's own
`cmd/sqlgen/testdata/examples/` golden + testcontainer suite already covers
exhaustive functional verification; what that suite cannot prove is that the
pieces compose into a real, hand-written application — which is this project's job.
