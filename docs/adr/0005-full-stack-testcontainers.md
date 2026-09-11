# Every integration test runs the full stack (Postgres + Redis + NATS) in testcontainers

Integration tests boot the entire backing stack — Postgres, Redis, and NATS — via
testcontainers, rather than substituting the in-process memory cache / memorybus
for the networked backends in tests. This is a deliberate choice for maximum
end-to-end confidence in the real wiring, accepted despite the cost: sqlgen only
ever calls the `cache.Backend` / `event.Publisher` interfaces, so in-process
backends would exercise the same sqlgen codepaths more cheaply. We record it so a
future contributor doesn't "optimize" the containers away — the point is to prove
the production adapters connect, not just that sqlgen's logic runs. Container
startup is amortized with a shared `TestMain`.
