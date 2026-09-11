# 16 — Seed command

Status: ready-for-agent
Blocked by: 07-projects-tasks-core, 08-task-assignment-labels, 09-task-content-time, 12-teams

Parent: `.scratch/app-server/PRD.md`

## What to build

A `cmd/seed` command that populates a realistic multi-workspace dataset so an operator can explore the API immediately. It builds on the same `*database.Client` the server uses and seeds across the core entities — Users, Workspaces, Memberships, Teams, Projects, Tasks, and their relationships (assignees, labels, comments) — spanning multiple Workspaces so tenant isolation is visible in the seeded data.

## Acceptance criteria

- [x] `cmd/seed` populates a realistic dataset across multiple Workspaces (story 54)
- [x] Seeded data spans Users, Workspaces, Memberships, Teams, Projects, and Tasks with relationships
- [x] The seeder uses the same `*database.Client` construction as the server
- [x] After seeding, the seeded entities are queryable through the GraphQL API under the appropriate Active Workspace

## Blocked by

- 07-projects-tasks-core
- 08-task-assignment-labels
- 09-task-content-time
- 12-teams
