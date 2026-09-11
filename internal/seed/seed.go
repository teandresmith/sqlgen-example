// Package seed populates a realistic, multi-workspace dataset so an operator can
// explore the API immediately after standing the stack up (issue 16, PRD story
// 54). It writes through the same *database.Client the server serves from, so
// every row it creates is subject to the real tenant scoping, authz hook, cache,
// and event publishing — and is therefore queryable through the GraphQL API
// under the appropriate Active Workspace exactly as if a user had created it.
//
// The dataset spans two Workspaces (Acme, Globex) with overlapping Users — one
// User is a member of both — so tenant isolation is visible in the seeded data:
// each Workspace's Teams, Projects, Tasks, Labels, and Comments belong to that
// Workspace alone, while the User identities are global and cross the boundary.
//
// Seeding assumes an empty (freshly migrated) database. Run creates rows with
// server-generated primary keys and does not upsert, so it is not safe to run
// twice: a second Run would hit the unique constraint on a seeded email or slug
// partway through. That is why the caller (cmd/seed) guards the invocation,
// refusing to seed a database that already holds Workspaces — which is what
// makes re-invoking the command a safe no-op even though Run itself is not
// idempotent.
package seed

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/teandresmith/sqlgen/omittable"
	"uuid"

	"github.com/teandresmith/sqlgen-example/internal/authz"
	"github.com/teandresmith/sqlgen-example/internal/database"
	"github.com/teandresmith/sqlgen-example/internal/tenancy"
)

// DefaultPassword is the plaintext credential every seeded User is given, so an
// operator can log in as any of them to explore the API. It is a development
// convenience, never a production secret — the seeder is a dev/exploration tool.
const DefaultPassword = "password123"

// Summary reports what a Run created, for the caller to print. It gives the
// operator the Workspace ids to put in X-Workspace-Id and an example login.
type Summary struct {
	// Workspaces lists each seeded Workspace with the login of its owner.
	Workspaces []WorkspaceSummary
	// Password is the plaintext credential shared by every seeded User.
	Password string
	// Totals counts every row created, keyed by a human-readable entity name.
	Totals map[string]int
}

// WorkspaceSummary identifies one seeded Workspace and how to enter it.
type WorkspaceSummary struct {
	ID         uuid.UUID
	Name       string
	Slug       string
	OwnerEmail string
}

// hashFunc hashes a plaintext password into the stored credential form. It is
// the auth service's HashPassword, passed in rather than imported so the seed
// package does not depend on the whole auth service — cmd/seed supplies it, and
// tests supply the harness's.
type hashFunc func(string) (string, error)

// Run seeds the dataset through client and returns a Summary of what it created.
// hashPassword hashes the shared DefaultPassword into each User's stored
// credential (the same hashing the register mutation uses, so seeded Users can
// log in). It creates the global Users first, then each Workspace's tenant-scoped
// graph under an Active-Workspace, system-granted context so the tenant column
// is injected and the authz hook admits the administrative writes (Memberships,
// Teams, Labels) a bare seeding context could not perform.
func Run(ctx context.Context, client *database.Client, hashPassword hashFunc, logger *slog.Logger) (*Summary, error) {
	s := &seeder{client: client, hashPassword: hashPassword, logger: logger, totals: map[string]int{}}
	if err := s.run(ctx); err != nil {
		return nil, err
	}
	return &Summary{Workspaces: s.workspaces, Password: DefaultPassword, Totals: s.totals}, nil
}

// seeder carries the client and accumulators through the seeding steps so the
// helpers stay small and the error handling reads linearly.
type seeder struct {
	client       *database.Client
	hashPassword hashFunc
	logger       *slog.Logger

	workspaces []WorkspaceSummary
	totals     map[string]int
}

func (s *seeder) run(ctx context.Context) error {
	// Global Users (untenanted): identities that span Workspaces. Alice belongs
	// to both Workspaces below, which is what makes global-user + tenant
	// isolation observable in one dataset.
	alice, err := s.user(ctx, "alice@taskr.dev", "Alice Nguyen")
	if err != nil {
		return err
	}
	bob, err := s.user(ctx, "bob@taskr.dev", "Bob Martinez")
	if err != nil {
		return err
	}
	carol, err := s.user(ctx, "carol@taskr.dev", "Carol Okafor")
	if err != nil {
		return err
	}
	dave, err := s.user(ctx, "dave@globex.dev", "Dave Kim")
	if err != nil {
		return err
	}
	erin, err := s.user(ctx, "erin@globex.dev", "Erin Zhao")
	if err != nil {
		return err
	}

	if err := s.seedAcme(ctx, alice, bob, carol); err != nil {
		return err
	}
	if err := s.seedGlobex(ctx, dave, erin, alice); err != nil {
		return err
	}
	return nil
}

// seedAcme builds the Acme Workspace: three members, two teams, a Label
// catalog, and two Projects of Tasks with assignees, labels, a subtask, and
// comments. alice owns it, bob is an admin, carol a member.
func (s *seeder) seedAcme(ctx context.Context, alice, bob, carol *database.User) error {
	ws, err := s.workspace(ctx, "Acme Corp", "acme")
	if err != nil {
		return err
	}
	// Every tenant-scoped write below runs under the Workspace as Active
	// Workspace (so sqlgen injects workspace_id) and with a system grant (so the
	// authz hook admits the administrative writes — Memberships, Teams,
	// team_members, Labels — that a member-role context could not perform).
	wctx := adminContext(ctx, ws.ID)
	s.recordWorkspace(ws, alice.Email)

	if err := s.membership(wctx, ws.ID, alice.ID, database.MembershipRoleOwner); err != nil {
		return err
	}
	if err := s.membership(wctx, ws.ID, bob.ID, database.MembershipRoleAdmin); err != nil {
		return err
	}
	if err := s.membership(wctx, ws.ID, carol.ID, database.MembershipRoleMember); err != nil {
		return err
	}

	eng, err := s.team(wctx, "Engineering", "eng", "Builds and ships the product")
	if err != nil {
		return err
	}
	design, err := s.team(wctx, "Design", "design", "Owns product design and brand")
	if err != nil {
		return err
	}
	if err := s.teamMembers(wctx, eng.ID, alice.ID, bob.ID); err != nil {
		return err
	}
	if err := s.teamMembers(wctx, design.ID, carol.ID); err != nil {
		return err
	}

	bug, err := s.label(wctx, "bug", "#d73a4a")
	if err != nil {
		return err
	}
	feature, err := s.label(wctx, "feature", "#0e8a16")
	if err != nil {
		return err
	}
	urgent, err := s.label(wctx, "urgent", "#b60205")
	if err != nil {
		return err
	}

	// Project 1 — Website Redesign, owned by Engineering.
	web, err := s.project(wctx, "Website Redesign", "Rebuild the marketing site on the new design system", eng.ID)
	if err != nil {
		return err
	}
	t1, err := s.task(wctx, taskSpec{Project: web.ID, Reporter: alice.ID, Title: "Set up design system tokens", Description: "Define color, spacing, and type tokens", Status: database.TaskStatusTodo, Priority: database.TaskPriorityHigh})
	if err != nil {
		return err
	}
	if err := s.assign(wctx, t1.ID, bob.ID); err != nil {
		return err
	}
	if err := s.tagTask(wctx, t1.ID, feature.ID); err != nil {
		return err
	}
	if err := s.comment(wctx, t1.ID, alice.ID, "Let's start from the Figma tokens export."); err != nil {
		return err
	}

	t2, err := s.task(wctx, taskSpec{Project: web.ID, Reporter: bob.ID, Title: "Fix nav dropdown on mobile", Description: "Dropdown clips behind the hero on small screens", Status: database.TaskStatusInProgress, Priority: database.TaskPriorityUrgent})
	if err != nil {
		return err
	}
	if err := s.assign(wctx, t2.ID, alice.ID); err != nil {
		return err
	}
	if err := s.tagTask(wctx, t2.ID, bug.ID); err != nil {
		return err
	}
	if err := s.tagTask(wctx, t2.ID, urgent.ID); err != nil {
		return err
	}
	// A subtask of t2, to exercise the self-referential parent_task_id.
	if _, err := s.task(wctx, taskSpec{Project: web.ID, Reporter: bob.ID, Title: "Audit z-index stacking", Description: "Find the offending stacking context", Status: database.TaskStatusTodo, Priority: database.TaskPriorityMedium, Parent: &t2.ID}); err != nil {
		return err
	}

	due := time.Date(2026, 10, 1, 17, 0, 0, 0, time.UTC)
	t3, err := s.task(wctx, taskSpec{Project: web.ID, Reporter: carol.ID, Title: "Launch marketing page", Description: "Ship the new landing page", Status: database.TaskStatusBacklog, Priority: database.TaskPriorityMedium, Due: &due})
	if err != nil {
		return err
	}
	if err := s.tagTask(wctx, t3.ID, feature.ID); err != nil {
		return err
	}

	// Project 2 — Mobile App, owned by Design.
	mobile, err := s.project(wctx, "Mobile App", "Native onboarding and core flows", design.ID)
	if err != nil {
		return err
	}
	t4, err := s.task(wctx, taskSpec{Project: mobile.ID, Reporter: carol.ID, Title: "Design onboarding flow", Description: "Three-screen first-run experience", Status: database.TaskStatusInReview, Priority: database.TaskPriorityHigh})
	if err != nil {
		return err
	}
	if err := s.assign(wctx, t4.ID, carol.ID); err != nil {
		return err
	}
	if err := s.tagTask(wctx, t4.ID, feature.ID); err != nil {
		return err
	}
	if err := s.comment(wctx, t4.ID, bob.ID, "Can we reuse the web tokens here too?"); err != nil {
		return err
	}
	return nil
}

// seedGlobex builds the Globex Workspace: dave owns it, erin is a member, and
// alice — already Acme's owner — joins as an admin, so the same global User
// spans both tenants. It has one Team and one Project of Tasks.
func (s *seeder) seedGlobex(ctx context.Context, dave, erin, alice *database.User) error {
	ws, err := s.workspace(ctx, "Globex", "globex")
	if err != nil {
		return err
	}
	wctx := adminContext(ctx, ws.ID)
	s.recordWorkspace(ws, dave.Email)

	if err := s.membership(wctx, ws.ID, dave.ID, database.MembershipRoleOwner); err != nil {
		return err
	}
	if err := s.membership(wctx, ws.ID, erin.ID, database.MembershipRoleMember); err != nil {
		return err
	}
	if err := s.membership(wctx, ws.ID, alice.ID, database.MembershipRoleAdmin); err != nil {
		return err
	}

	platform, err := s.team(wctx, "Platform", "platform", "Owns the billing and infra stack")
	if err != nil {
		return err
	}
	if err := s.teamMembers(wctx, platform.ID, dave.ID, erin.ID); err != nil {
		return err
	}

	backend, err := s.label(wctx, "backend", "#1d76db")
	if err != nil {
		return err
	}
	infra, err := s.label(wctx, "infra", "#5319e7")
	if err != nil {
		return err
	}

	billing, err := s.project(wctx, "Billing Service", "Usage metering and invoice generation", platform.ID)
	if err != nil {
		return err
	}
	t1, err := s.task(wctx, taskSpec{Project: billing.ID, Reporter: dave.ID, Title: "Invoice generation", Description: "Generate monthly invoices from usage records", Status: database.TaskStatusInProgress, Priority: database.TaskPriorityHigh})
	if err != nil {
		return err
	}
	if err := s.assign(wctx, t1.ID, erin.ID); err != nil {
		return err
	}
	if err := s.tagTask(wctx, t1.ID, backend.ID); err != nil {
		return err
	}
	if err := s.comment(wctx, t1.ID, dave.ID, "Round half-up to the cent to match the ledger."); err != nil {
		return err
	}

	t2, err := s.task(wctx, taskSpec{Project: billing.ID, Reporter: erin.ID, Title: "Rate limit the metering API", Description: "Protect the ingest endpoint from bursts", Status: database.TaskStatusTodo, Priority: database.TaskPriorityMedium})
	if err != nil {
		return err
	}
	if err := s.assign(wctx, t2.ID, dave.ID); err != nil {
		return err
	}
	if err := s.tagTask(wctx, t2.ID, backend.ID); err != nil {
		return err
	}
	if err := s.tagTask(wctx, t2.ID, infra.ID); err != nil {
		return err
	}
	return nil
}

// adminContext returns a context that makes wsID the Active Workspace (so
// sqlgen injects workspace_id on tenant-scoped writes) and carries a system
// grant (so the authz hook admits administrative writes on guarded tables
// without an ambient owner/admin Role). Both signals are settable only by
// server code, so this is the same trusted-bootstrap mechanism the server's
// own bootstrap paths use — see internal/authz.
func adminContext(ctx context.Context, wsID uuid.UUID) context.Context {
	return authz.WithSystemGrant(tenancy.WithActiveWorkspace(ctx, wsID))
}

// recordWorkspace appends a Workspace to the summary and logs its creation.
func (s *seeder) recordWorkspace(ws *database.Workspace, ownerEmail string) {
	s.workspaces = append(s.workspaces, WorkspaceSummary{ID: ws.ID, Name: ws.Name, Slug: ws.Slug, OwnerEmail: ownerEmail})
	if s.logger != nil {
		s.logger.Info("seeding workspace", "slug", ws.Slug, "id", ws.ID)
	}
}

func (s *seeder) user(ctx context.Context, email, displayName string) (*database.User, error) {
	hash, err := s.hashPassword(DefaultPassword)
	if err != nil {
		return nil, fmt.Errorf("seed: hashing password for %s: %w", email, err)
	}
	u, err := s.client.Users().Create(ctx, &database.CreateUserInput{
		Email:        email,
		DisplayName:  displayName,
		PasswordHash: hash,
	})
	if err != nil {
		return nil, fmt.Errorf("seed: creating user %s: %w", email, err)
	}
	s.totals["users"]++
	return u, nil
}

func (s *seeder) workspace(ctx context.Context, name, slug string) (*database.Workspace, error) {
	ws, err := s.client.Workspaces().Create(ctx, &database.CreateWorkspaceInput{Name: name, Slug: slug})
	if err != nil {
		return nil, fmt.Errorf("seed: creating workspace %s: %w", slug, err)
	}
	s.totals["workspaces"]++
	return ws, nil
}

func (s *seeder) membership(ctx context.Context, wsID, userID uuid.UUID, role database.MembershipRole) error {
	if _, err := s.client.Memberships().Create(ctx, &database.CreateMembershipInput{
		WorkspaceID: wsID,
		UserID:      userID,
		Role:        omittable.Set(role),
	}); err != nil {
		return fmt.Errorf("seed: creating membership (ws=%s user=%s): %w", wsID, userID, err)
	}
	s.totals["memberships"]++
	return nil
}

func (s *seeder) team(ctx context.Context, name, slug, description string) (*database.Team, error) {
	desc := description
	t, err := s.client.Teams().Create(ctx, &database.CreateTeamInput{
		Name:        name,
		Slug:        slug,
		Description: omittable.Set(&desc),
	})
	if err != nil {
		return nil, fmt.Errorf("seed: creating team %s: %w", slug, err)
	}
	s.totals["teams"]++
	return t, nil
}

func (s *seeder) teamMembers(ctx context.Context, teamID uuid.UUID, userIDs ...uuid.UUID) error {
	for _, userID := range userIDs {
		if _, err := s.client.TeamMembers().Create(ctx, &database.CreateTeamMemberInput{
			TeamID: teamID,
			UserID: userID,
		}); err != nil {
			return fmt.Errorf("seed: adding user %s to team %s: %w", userID, teamID, err)
		}
		s.totals["team_members"]++
	}
	return nil
}

func (s *seeder) label(ctx context.Context, name, color string) (*database.Label, error) {
	l, err := s.client.Labels().Create(ctx, &database.CreateLabelInput{
		Name:  name,
		Color: omittable.Set(color),
	})
	if err != nil {
		return nil, fmt.Errorf("seed: creating label %s: %w", name, err)
	}
	s.totals["labels"]++
	return l, nil
}

func (s *seeder) project(ctx context.Context, name, description string, teamID uuid.UUID) (*database.Project, error) {
	desc := description
	p, err := s.client.Projects().Create(ctx, &database.CreateProjectInput{
		Name:        name,
		Description: omittable.Set(&desc),
		TeamID:      omittable.Set(&teamID),
	})
	if err != nil {
		return nil, fmt.Errorf("seed: creating project %s: %w", name, err)
	}
	s.totals["projects"]++
	return p, nil
}

// taskSpec describes one Task to seed. Bundling the fields keeps the several
// same-typed values (the two user ids, the title/description) from being
// transposable positional arguments, and lets the two optional fields read
// alike: an invalid Parent means a top-level task, a nil Due leaves the due
// date unset.
type taskSpec struct {
	Project     uuid.UUID
	Reporter    uuid.UUID
	Title       string
	Description string
	Status      database.TaskStatus
	Priority    database.TaskPriority
	Parent      *uuid.UUID
	Due         *time.Time
}

// task creates the Task described by spec.
func (s *seeder) task(ctx context.Context, spec taskSpec) (*database.Task, error) {
	desc := spec.Description
	input := &database.CreateTaskInput{
		ProjectID:   spec.Project,
		ReporterID:  spec.Reporter,
		Title:       spec.Title,
		Description: omittable.Set(&desc),
		Status:      omittable.Set(spec.Status),
		Priority:    omittable.Set(spec.Priority),
	}
	if spec.Parent != nil {
		input.ParentTaskID = omittable.Set(spec.Parent)
	}
	if spec.Due != nil {
		input.DueAt = omittable.Set(spec.Due)
	}
	t, err := s.client.Tasks().Create(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("seed: creating task %q: %w", spec.Title, err)
	}
	s.totals["tasks"]++
	return t, nil
}

func (s *seeder) assign(ctx context.Context, taskID, userID uuid.UUID) error {
	if _, err := s.client.TaskAssignees().Create(ctx, &database.CreateTaskAssigneeInput{
		TaskID: taskID,
		UserID: userID,
	}); err != nil {
		return fmt.Errorf("seed: assigning task %s to user %s: %w", taskID, userID, err)
	}
	s.totals["assignees"]++
	return nil
}

func (s *seeder) tagTask(ctx context.Context, taskID, labelID uuid.UUID) error {
	if _, err := s.client.TaskLabels().Create(ctx, &database.CreateTaskLabelInput{
		TaskID:  taskID,
		LabelID: labelID,
	}); err != nil {
		return fmt.Errorf("seed: tagging task %s with label %s: %w", taskID, labelID, err)
	}
	s.totals["task_labels"]++
	return nil
}

func (s *seeder) comment(ctx context.Context, taskID, authorID uuid.UUID, body string) error {
	if _, err := s.client.Comments().Create(ctx, &database.CreateCommentInput{
		TaskID:   taskID,
		AuthorID: authorID,
		Body:     body,
	}); err != nil {
		return fmt.Errorf("seed: commenting on task %s: %w", taskID, err)
	}
	s.totals["comments"]++
	return nil
}
