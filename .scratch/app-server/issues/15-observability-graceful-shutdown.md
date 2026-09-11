# 15 — Observability & graceful shutdown

Status: ready-for-agent
Blocked by: 01-walking-skeleton

Parent: `.scratch/app-server/PRD.md`

## What to build

Operational readiness. An OpenTelemetry `MetricsRecorder` (from `metrics/otel`) is attached to the client and the HTTP server, exporting metrics to the collector defined in `docker-compose`. The server shuts down gracefully so in-flight requests complete and connections close cleanly, and exposes a readiness check so orchestration can tell when it is live (building on the health endpoint from the skeleton).

## Acceptance criteria

- [x] An OTel `MetricsRecorder` is attached to the client and the HTTP server (story 56)
- [x] Request and query metrics are exported to the collector defined in `docker-compose`
- [x] The server shuts down gracefully: in-flight requests complete and connections close cleanly (story 58)
- [x] A health/readiness check reports when the server is live (story 57)
- [x] A test or documented manual check confirms metrics reach the collector and shutdown drains in-flight work

## Blocked by

- 01-walking-skeleton
