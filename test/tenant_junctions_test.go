package test

// Tenant junctions (migration 0006): task_assignees, task_watchers and
// team_members carry workspace_id with a composite FK to their parent, so
// sqlgen scopes the link and the database refuses a cross-workspace one. Each
// junction is run as Workspace A, as Workspace B, and with SkipTenancy.

import (
	"context"
	"testing"
	"uuid"

	"github.com/teandresmith/sqlgen/comparator"
	"github.com/teandresmith/sqlgen/omittable"

	"github.com/teandresmith/sqlgen-example/internal/database"
	"github.com/teandresmith/sqlgen-example/internal/tenancy"
)

type junctionCase struct {
	name      string
	table     string
	parentCol string
	newParent func(t *testing.T, token, wsID string) string
	// create links parent+user as the context's tenant (or explicit ws under skip).
	create func(ctx context.Context, parent, user uuid.UUID, ws *uuid.UUID, skip bool) error
	count  func(ctx context.Context, user uuid.UUID, skip bool) (int, error)
	remove func(ctx context.Context, parent, user uuid.UUID) error
}

func ctxAs(ws string) context.Context {
	id := uuid.MustParse(ws)
	return tenancy.WithRole(tenancy.WithActiveWorkspace(context.Background(), id), database.MembershipRoleOwner)
}

func optSkip[T any](skip bool) func(*database.CallOptions[T]) {
	return func(o *database.CallOptions[T]) {
		if skip {
			o.SkipTenancy = true
		}
		o.SkipCache = true
	}
}

func idEq(u uuid.UUID) *comparator.ID { s := u.String(); return &comparator.ID{Eq: &s} }

var junctionCases = []junctionCase{
	{
		name: "task_assignees", table: "task_assignees", parentCol: "task_id",
		newParent: func(t *testing.T, token, ws string) string {
			p := createProject(t, token, ws, "Junction work")
			return createTaskInProject(t, token, ws, p, "Junction task", "TODO", "MEDIUM", "").ID
		},
		create: func(ctx context.Context, p, u uuid.UUID, ws *uuid.UUID, skip bool) error {
			in := &database.CreateTaskAssigneeInput{TaskID: p, UserID: u}
			if ws != nil {
				in.WorkspaceID = omittable.Set(*ws)
			}
			_, err := testClient.TaskAssignees().Create(ctx, in, optSkip[database.TaskAssigneeFieldOptions](skip))
			return err
		},
		count: func(ctx context.Context, u uuid.UUID, skip bool) (int, error) {
			return int64To(testClient.TaskAssignees().Count(ctx, &database.TaskAssigneeFilter{UserID: idEq(u)}, optSkip[database.TaskAssigneeFieldOptions](skip)))
		},
		remove: func(ctx context.Context, p, u uuid.UUID) error {
			return testClient.TaskAssignees().HardDelete(ctx, database.TaskAssigneePK{TaskID: p, UserID: u}, optSkip[database.TaskAssigneeFieldOptions](false))
		},
	},
	{
		name: "task_watchers", table: "task_watchers", parentCol: "task_id",
		newParent: func(t *testing.T, token, ws string) string {
			p := createProject(t, token, ws, "Junction work")
			return createTaskInProject(t, token, ws, p, "Junction task", "TODO", "MEDIUM", "").ID
		},
		create: func(ctx context.Context, p, u uuid.UUID, ws *uuid.UUID, skip bool) error {
			in := &database.CreateTaskWatcherInput{TaskID: p, UserID: u}
			if ws != nil {
				in.WorkspaceID = omittable.Set(*ws)
			}
			_, err := testClient.TaskWatchers().Create(ctx, in, optSkip[database.TaskWatcherFieldOptions](skip))
			return err
		},
		count: func(ctx context.Context, u uuid.UUID, skip bool) (int, error) {
			return int64To(testClient.TaskWatchers().Count(ctx, &database.TaskWatcherFilter{UserID: idEq(u)}, optSkip[database.TaskWatcherFieldOptions](skip)))
		},
		remove: func(ctx context.Context, p, u uuid.UUID) error {
			return testClient.TaskWatchers().HardDelete(ctx, database.TaskWatcherPK{TaskID: p, UserID: u}, optSkip[database.TaskWatcherFieldOptions](false))
		},
	},
	{
		name: "team_members", table: "team_members", parentCol: "team_id",
		newParent: func(t *testing.T, token, ws string) string {
			return createTeam(t, token, ws, "Junction team", "junction-team").ID
		},
		create: func(ctx context.Context, p, u uuid.UUID, ws *uuid.UUID, skip bool) error {
			in := &database.CreateTeamMemberInput{TeamID: p, UserID: u}
			if ws != nil {
				in.WorkspaceID = omittable.Set(*ws)
			}
			_, err := testClient.TeamMembers().Create(ctx, in, optSkip[database.TeamMemberFieldOptions](skip))
			return err
		},
		count: func(ctx context.Context, u uuid.UUID, skip bool) (int, error) {
			return int64To(testClient.TeamMembers().Count(ctx, &database.TeamMemberFilter{UserID: idEq(u)}, optSkip[database.TeamMemberFieldOptions](skip)))
		},
		remove: func(ctx context.Context, p, u uuid.UUID) error {
			return testClient.TeamMembers().HardDelete(ctx, database.TeamMemberPK{TeamID: p, UserID: u}, optSkip[database.TeamMemberFieldOptions](false))
		},
	},
}

func TestTenantJunctions(t *testing.T) {
	for _, jc := range junctionCases {
		t.Run(jc.name, func(t *testing.T) {
			token, user := register(t, "junction-"+jc.name+"@example.com", "junction-pw-"+jc.name, "Junction "+jc.name)
			wsA := bootstrapWorkspace(t, token, "Junction A "+jc.name, "junction-a-"+jc.name)
			wsB := bootstrapWorkspace(t, token, "Junction B "+jc.name, "junction-b-"+jc.name)
			parentA := uuid.MustParse(jc.newParent(t, token, wsA))
			parentB := uuid.MustParse(jc.newParent(t, token, wsB))
			u := uuid.MustParse(user.ID)
			ctxA, ctxB := ctxAs(wsA), ctxAs(wsB)
			uuidA, uuidB := uuid.MustParse(wsA), uuid.MustParse(wsB)

			// Each workspace links the same user to its own parent.
			if err := jc.create(ctxA, parentA, u, nil, false); err != nil {
				t.Fatalf("as A, own parent: %v", err)
			}
			if err := jc.create(ctxB, parentB, u, nil, false); err != nil {
				t.Fatalf("as B, own parent: %v", err)
			}

			t.Run("load", func(t *testing.T) {
				for _, c := range []struct {
					who  string
					ctx  context.Context
					skip bool
					want int
				}{{"A", ctxA, false, 1}, {"B", ctxB, false, 1}, {"skip", context.Background(), true, 2}} {
					got, err := jc.count(c.ctx, u, c.skip)
					if err != nil || got != c.want {
						t.Errorf("as %s: count=%d err=%v, want %d", c.who, got, err, c.want)
					}
				}
			})

			t.Run("cross-workspace create is refused", func(t *testing.T) {
				// As A, naming B's parent: the stamped tenant is A, the parent is B's.
				if err := jc.create(ctxA, parentB, u, nil, false); err == nil {
					t.Error("as A with B's parent: want an error")
				}
				// As B, naming A's parent.
				if err := jc.create(ctxB, parentA, u, nil, false); err == nil {
					t.Error("as B with A's parent: want an error")
				}
				// SkipTenancy with an explicit workspace that is not the parent's.
				if err := jc.create(context.Background(), parentB, u, &uuidA, true); err == nil {
					t.Error("skip, ws A with B's parent: want an error")
				}
				if err := jc.create(context.Background(), parentA, u, &uuidB, true); err == nil {
					t.Error("skip, ws B with A's parent: want an error")
				}
				if got, _ := jc.count(context.Background(), u, true); got != 2 {
					t.Errorf("rows after refused writes = %d, want 2", got)
				}
			})

			t.Run("raw SQL cannot cross workspaces", func(t *testing.T) {
				q := "INSERT INTO " + jc.table + " (workspace_id, " + jc.parentCol + ", user_id) VALUES ($1, $2, $3)"
				other := uuid.MustParse(createUser(t, "junction-raw-"+jc.name))
				if _, err := testPool.Exec(context.Background(), q, uuidA, parentB, other); err == nil {
					t.Error("raw insert (ws A, B's parent) succeeded")
				}
			})

			t.Run("delete is tenant-scoped", func(t *testing.T) {
				// As B, deleting A's link must not remove it.
				_ = jc.remove(ctxB, parentA, u)
				if got, _ := jc.count(ctxA, u, false); got != 1 {
					t.Errorf("A's link after B's delete = %d, want 1", got)
				}
				if err := jc.remove(ctxA, parentA, u); err != nil {
					t.Fatalf("as A, own link: %v", err)
				}
				if got, _ := jc.count(ctxA, u, false); got != 0 {
					t.Errorf("A's link after own delete = %d, want 0", got)
				}
				if got, _ := jc.count(ctxB, u, false); got != 1 {
					t.Errorf("B's link after A's delete = %d, want 1", got)
				}
			})
		})
	}
}

// createUser inserts a bare user row for a raw-SQL probe and returns its id.
func createUser(t *testing.T, tag string) string {
	t.Helper()
	var id string
	err := testPool.QueryRow(context.Background(),
		"INSERT INTO users (email, display_name, password_hash) VALUES ($1, $2, 'x') RETURNING id::text",
		tag+"@example.com", tag).Scan(&id)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	return id
}

func int64To(n int64, err error) (int, error) { return int(n), err }
