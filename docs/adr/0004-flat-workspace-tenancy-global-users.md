# Flat Workspace tenancy with global Users

The tenant boundary is a single flat level: the Workspace, identified by a
`workspace_id` column on every tenant-scoped table. Users are global identities
that join many Workspaces through the Membership junction; the active Workspace is
chosen per request. We chose flat tenancy (not an Organization → Workspace
hierarchy) because sqlgen's tenancy is v1 flat/single-level — a second level would
be hand-rolled against the grain of the tool — and because global-user +
per-request-workspace is the realistic multi-tenant SaaS shape that best exercises
sqlgen's structural tenant scoping composing with the Membership M2M.
