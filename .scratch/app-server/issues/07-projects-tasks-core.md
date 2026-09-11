# 07 — Projects & Tasks core

Status: ready-for-human
Blocked by: 03-workspaces-membership-tenant-scoping

Parent: `.scratch/app-server/PRD.md`

## What to build

The core work graph, verified through the tenant-scoped GraphQL seam. A Workspace member creates a Project and archives it (soft-delete) so it leaves active views without losing history. A member creates a Task in a Project with a status and priority, and the creating User is set as the Task's Reporter. Tasks can nest as Subtasks via the self-referential parent, and a Task can declare that it depends on another Task, with both dependencies and dependents visible. Basic CRUD is generated; this ticket wires and proves the behavior end to end.

## Acceptance criteria

- [x] A Workspace member can create a Project (story 19)
- [x] A Workspace member can archive a Project (soft-delete); it disappears from active views but history is retained (story 20)
- [x] A Workspace member can create a Task in a Project with a status and priority (story 21)
- [x] The Task's Reporter is set to the creating User (story 30)
- [x] A Subtask can be created under a Task via the self-referential parent (story 22)
- [x] A Task can declare a dependency on another Task, and its dependencies and dependents can be read (stories 23, 24)
- [x] Integration tests exercise the above through the HTTP seam under an Active Workspace

## Blocked by

- 03-workspaces-membership-tenant-scoping
