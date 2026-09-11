# Task Tracker (sqlgen example)

A multi-tenant SaaS task tracker built as the reference/example consumer of the
`sqlgen` code generator ([github.com/teandresmith/sqlgen](https://github.com/teandresmith/sqlgen)). It is both an adopter-facing example and a
live integration test bed. The API surface is GraphQL. Postgres is the only
database; the schema is DDL-driven.

## Language

### Tenancy & identity

**Workspace**:
The tenant root — the single unit of isolation. Every tenant-scoped row carries a
`workspace_id`. A Workspace is itself a _shared_ (non-tenant-scoped) entity.
_Avoid_: Organization, Org, Team, Tenant, Account.

**User**:
A global identity that can belong to many Workspaces. Users are _shared_ (not
scoped to a Workspace).
_Avoid_: Account, Member (a Member is the link, not the person), Person.

**Membership**:
The link between a User and a Workspace, carrying the User's `role` in that
Workspace. Modeled as a M2M junction with composite PK `(workspace_id, user_id)`.
_Avoid_: Member, Seat, Role assignment.

**Role**:
The permission level a Membership grants (`owner`, `admin`, `member`, `guest`).
An enum on Membership, not a standalone entity.
_Avoid_: Permission, Access level.

### Work items

**Project**:
A container for Tasks, owned by exactly one Workspace. Can be archived
(soft-deleted).
_Avoid_: Board, List, Space.

**Task**:
The unit of work, belonging to one Project. Carries a `status` and `priority`
enum, optional `custom_fields` (JSONB), and may have a parent Task (subtasks, via
a self-referential FK). Can be archived (soft-deleted).
_Avoid_: Ticket, Issue, Card, Item, Story.

**Subtask**:
A Task whose `parent_task_id` points at another Task. Not a separate entity — the
same Task table, self-referenced.
_Avoid_: Child ticket, Sub-item.

**Reporter**:
The User who created a Task (single FK, `reporter_id`). Distinct from Assignees.
_Avoid_: Author, Creator, Owner.

**Assignee**:
A User responsible for a Task. A Task may have many Assignees (M2M junction
`task_assignees`).
_Avoid_: Owner, Handler, Worker.

**Label**:
A Workspace-scoped tag applied to Tasks (M2M junction `task_labels`).
_Avoid_: Tag, Category, Marker.

**Comment**:
A note left by a User on a Task (O2M from Task).
_Avoid_: Note, Reply, Message.

### Org structure

**Team**:
A named group of Users within a Workspace that can own Projects and Cycles.
Distinct from a Workspace (the tenant) and a Membership (the user↔workspace link).
_Avoid_: Group, Squad, Department.

### Planning

**Cycle**:
A time-boxed planning iteration (a sprint), with a start and end date, that Tasks
can be scheduled into.
_Avoid_: Sprint, Iteration, Milestone.

**Task Dependency**:
A directed link from one Task to another it depends on (`blocks`, `relates_to`,
`duplicates`). Modeled as a self-referential M2M.
_Avoid_: Blocker, Link, Relation.

**Watcher**:
A User subscribed to updates on a Task, without being responsible for it (unlike
an Assignee). M2M Task↔User.
_Avoid_: Subscriber, Follower.

### Time & files

**Time Entry**:
A record of hours a User logged against a Task on a given day. Hours are a
decimal quantity.
_Avoid_: Timesheet, Worklog, Booking.

**Attachment**:
A file bound to either a Task or a Comment, distinguished by an `entity_type`
discriminator (polymorphic). Blob lives in external storage; the row is metadata.
_Avoid_: File, Upload, Document.

### Access & security

**API Token**:
A hashed credential granting scoped API access, owned by a User (personal) or by
no User (service account). Distinct from the JWT used for interactive login.
_Avoid_: API key, Secret, Credential.

**Invitation**:
A pending, tokenized offer for an email address to join a Workspace with a given
Role. Becomes a Membership on acceptance.
_Avoid_: Invite, Request.

**SSO Connection**:
A Workspace's single sign-on configuration for an identity provider (SAML, OIDC,
Google, Okta).
_Avoid_: IdP, Auth provider, Identity connection.

### Platform

**Activity**:
An append-only record of a mutation, written by the event subscriber that
consumes sqlgen mutation events. The Activity table has events disabled on itself
(no events about the event log).
_Avoid_: Audit log, Event (an Event is the in-flight sqlgen message; an Activity
is the persisted feed row), History.

**Active Workspace**:
The Workspace a given request operates in, selected per request by the
`X-Workspace-Id` header and validated against the caller's Memberships. Supplies
the tenant to sqlgen's `TenantResolver`.
_Avoid_: Current org, Selected tenant.
