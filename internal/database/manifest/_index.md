# database manifest

- **Dialect:** postgres
- **Generator:** sqlgen 0.1.0
- **Schema version:** 0.1.0

Features: audit_columns, cache, events, graph_top_level, graphql, soft_delete, tenancy, views

## Entities

| Name | Table | Kind | File | Description |
| --- | --- | --- | --- | --- |
| APIToken | `api_tokens` | table | [api_token.md](api_token.md) | API access tokens, personal or service-account scoped |
| Activity | `activity` | table | [activity.md](activity.md) | Append-only activity feed derived from mutation events |
| Attachment | `attachments` | table | [attachment.md](attachment.md) | Files attached to a task or a comment (polymorphic) |
| Comment | `comments` | table | [comment.md](comment.md) | A note left by a user on a task |
| Cycle | `cycles` | table | [cycle.md](cycle.md) | A time-boxed planning iteration (sprint) |
| Invitation | `invitations` | table | [invitation.md](invitation.md) | Pending invitations for users to join a workspace |
| Label | `labels` | table | [label.md](label.md) | A workspace-scoped tag applied to tasks |
| Membership | `memberships` | table | [membership.md](membership.md) | Links a user to a workspace with a role (M2M) |
| Project | `projects` | table | [project.md](project.md) | A container for tasks, owned by one workspace |
| ProjectStat | `project_stats` | view | [project_stat.md](project_stat.md) |  |
| SsoConnection | `sso_connections` | table | [sso_connection.md](sso_connection.md) | Per-workspace single sign-on configuration |
| Task | `tasks` | table | [task.md](task.md) | The unit of work; belongs to a project, may have a parent task |
| TaskAssignee | `task_assignees` | table | [task_assignee.md](task_assignee.md) | Assigns users to tasks (M2M) |
| TaskDependency | `task_dependencies` | table | [task_dependency.md](task_dependency.md) | Directed dependencies between tasks (self-referential M2M) |
| TaskLabel | `task_labels` | table | [task_label.md](task_label.md) | Applies labels to tasks (M2M) |
| TaskWatcher | `task_watchers` | table | [task_watcher.md](task_watcher.md) | Users subscribed to updates on a task (M2M) |
| Team | `teams` | table | [team.md](team.md) | A group of users within a workspace that owns projects |
| TeamMember | `team_members` | table | [team_member.md](team_member.md) | Membership of users in teams (M2M) |
| TimeEntry | `time_entries` | table | [time_entry.md](time_entry.md) | Logged time against a task |
| User | `users` | table | [user.md](user.md) | Global user identity, spanning workspaces |
| Workspace | `workspaces` | table | [workspace.md](workspace.md) | Tenant root; the unit of isolation |
| WorkspaceSetting | `workspace_settings` | table | [workspace_setting.md](workspace_setting.md) | Per-workspace configuration (one row per workspace) |

See [conventions](_conventions.md) for package-wide error sentinels, pagination, comparators, soft-delete, CallOptions, and the omittable reference.
