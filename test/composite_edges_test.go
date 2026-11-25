package test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"uuid"

	"github.com/teandresmith/sqlgen/comparator"

	"github.com/teandresmith/sqlgen-example/internal/database"
)

// This file exercises the two composite-foreign-key edges sqlgen v0.1.2 detects
// as real relationships — Cycle.tasks through tasks_cycle_same_ws
// (workspace_id, cycle_id) → cycles(workspace_id, id) and Team.projects through
// projects_team_same_ws (workspace_id, team_id) → teams(workspace_id, id).
//
// Two Workspaces are planted with the same data (same names, slugs and titles,
// same owner), so nothing but the Workspace tells their rows apart. One row in
// Workspace B is then pointed at Workspace A's parent with the FK triggers off
// (session_replication_role = replica): a "foreign" child whose key is
// (B, parentA). A single-column join (cycle_id = id) would attach it to
// parentA; only the composite key keeps it off. Under tenant scoping the child's
// own tenant filter would hide it anyway, so the SkipTenancy Go-client cases are
// the ones where the composite key is the sole separator. The app itself never
// calls SkipTenancy on cycles/teams/tasks/projects, so those cases probe the
// generated edge, not an app path.
//
// Each edge runs the same matrix: relationship load, relationship filter, and
// nested writes (connect, allow_reparent, cross-Workspace connect, disconnect,
// clear) as Workspace A, as Workspace B, and with SkipTenancy.

// compositeEdge describes one composite edge in GraphQL and Go-client terms so
// the matrix below runs unchanged over both.
type compositeEdge struct {
	parentField    string // cycle / team
	listField      string // cycles / teams
	childField     string // tasks / projects
	nestedMutation string // updateCycleWithRelated / updateTeamWithRelated
	childNameField string // title / name
	childTable     string // tasks / projects
	childFKCol     string // cycle_id / team_id

	newParent func(t *testing.T, token, wsID, name, slug string) string
	newChild  func(t *testing.T, token, wsID, name string) string

	// Go client, always SkipTenancy.
	loadChildren func(ctx context.Context, parentIDs []uuid.UUID) (map[uuid.UUID][]uuid.UUID, error)
	parentsWith  func(ctx context.Context, parentIDs []uuid.UUID, childID uuid.UUID) ([]uuid.UUID, error)
	nested       func(ctx context.Context, parentID uuid.UUID, connect, disconnect []uuid.UUID, clear bool) error
}

func skipTenancy[T any](o *database.CallOptions[T]) {
	o.SkipTenancy = true
	o.SkipCache = true
}

var cycleTasksEdge = compositeEdge{
	parentField: "cycle", listField: "cycles", childField: "tasks",
	nestedMutation: "updateCycleWithRelated", childNameField: "title",
	childTable: "tasks", childFKCol: "cycle_id",
	newParent: func(t *testing.T, token, wsID, name, _ string) string {
		return createCycle(t, token, wsID, name, "ACTIVE").ID
	},
	newChild: func(t *testing.T, token, wsID, name string) string {
		projID := createProject(t, token, wsID, "Shared work")
		return createTaskInProject(t, token, wsID, projID, name, "TODO", "MEDIUM", "").ID
	},
	loadChildren: func(ctx context.Context, parentIDs []uuid.UUID) (map[uuid.UUID][]uuid.UUID, error) {
		cycles, err := testClient.Cycles().GetMany(ctx, &database.GetCyclesInput{
			Filter: &database.CycleFilter{ID: &comparator.ID{In: uuidStrings(parentIDs)}},
		}, func(o *database.CallOptions[database.CycleFieldOptions]) {
			skipTenancy(o)
			o.FieldOptions = &database.CycleFieldOptions{ID: true,
				Tasks: &database.TaskRelationshipOptions{FieldOptions: &database.TaskFieldOptions{ID: true}}}
		})
		if err != nil {
			return nil, err
		}
		out := map[uuid.UUID][]uuid.UUID{}
		for _, c := range cycles {
			out[c.ID] = []uuid.UUID{}
			for _, tk := range c.Tasks {
				out[c.ID] = append(out[c.ID], tk.ID)
			}
		}
		return out, nil
	},
	parentsWith: func(ctx context.Context, parentIDs []uuid.UUID, childID uuid.UUID) ([]uuid.UUID, error) {
		cycles, err := testClient.Cycles().GetMany(ctx, &database.GetCyclesInput{
			Filter: &database.CycleFilter{
				ID:    &comparator.ID{In: uuidStrings(parentIDs)},
				Tasks: &database.TaskFilter{ID: &comparator.ID{Eq: new(childID.String())}},
			},
		}, func(o *database.CallOptions[database.CycleFieldOptions]) {
			skipTenancy(o)
			o.FieldOptions = &database.CycleFieldOptions{ID: true}
		})
		ids := make([]uuid.UUID, 0, len(cycles))
		for _, c := range cycles {
			ids = append(ids, c.ID)
		}
		return ids, err
	},
	nested: func(ctx context.Context, parentID uuid.UUID, connect, disconnect []uuid.UUID, clear bool) error {
		_, err := testClient.Cycles().UpdateWithRelated(ctx, parentID, &database.UpdateCycleWithRelatedInput{
			Tasks: &database.CycleTasksUpdateNested{Connect: connect, Disconnect: disconnect, Clear: clear},
		}, skipTenancy[database.CycleFieldOptions])
		return err
	},
}

var teamProjectsEdge = compositeEdge{
	parentField: "team", listField: "teams", childField: "projects",
	nestedMutation: "updateTeamWithRelated", childNameField: "name",
	childTable: "projects", childFKCol: "team_id",
	newParent: func(t *testing.T, token, wsID, name, slug string) string {
		return createTeam(t, token, wsID, name, slug).ID
	},
	newChild: func(t *testing.T, token, wsID, name string) string {
		return createProject(t, token, wsID, name)
	},
	loadChildren: func(ctx context.Context, parentIDs []uuid.UUID) (map[uuid.UUID][]uuid.UUID, error) {
		teams, err := testClient.Teams().GetMany(ctx, &database.GetTeamsInput{
			Filter: &database.TeamFilter{ID: &comparator.ID{In: uuidStrings(parentIDs)}},
		}, func(o *database.CallOptions[database.TeamFieldOptions]) {
			skipTenancy(o)
			o.FieldOptions = &database.TeamFieldOptions{ID: true,
				Projects: &database.ProjectRelationshipOptions{FieldOptions: &database.ProjectFieldOptions{ID: true}}}
		})
		if err != nil {
			return nil, err
		}
		out := map[uuid.UUID][]uuid.UUID{}
		for _, tm := range teams {
			out[tm.ID] = []uuid.UUID{}
			for _, p := range tm.Projects {
				out[tm.ID] = append(out[tm.ID], p.ID)
			}
		}
		return out, nil
	},
	parentsWith: func(ctx context.Context, parentIDs []uuid.UUID, childID uuid.UUID) ([]uuid.UUID, error) {
		teams, err := testClient.Teams().GetMany(ctx, &database.GetTeamsInput{
			Filter: &database.TeamFilter{
				ID:       &comparator.ID{In: uuidStrings(parentIDs)},
				Projects: &database.ProjectFilter{ID: &comparator.ID{Eq: new(childID.String())}},
			},
		}, func(o *database.CallOptions[database.TeamFieldOptions]) {
			skipTenancy(o)
			o.FieldOptions = &database.TeamFieldOptions{ID: true}
		})
		ids := make([]uuid.UUID, 0, len(teams))
		for _, tm := range teams {
			ids = append(ids, tm.ID)
		}
		return ids, err
	},
	nested: func(ctx context.Context, parentID uuid.UUID, connect, disconnect []uuid.UUID, clear bool) error {
		_, err := testClient.Teams().UpdateWithRelated(ctx, parentID, &database.UpdateTeamWithRelatedInput{
			Projects: &database.TeamProjectsUpdateNested{Connect: connect, Disconnect: disconnect, Clear: clear},
		}, skipTenancy[database.TeamFieldOptions])
		return err
	},
}

func uuidStrings(ids []uuid.UUID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return out
}

// compositeFixture is the planted data for one edge:
//
//	Workspace A: parentA  ← childA1      childA2 (unlinked)   parentA2 (empty)
//	Workspace B: parentB  ← childB1      childB2 → (B, parentA)  [FK-violating]
type compositeFixture struct {
	token                      string
	wsA, wsB                   string
	parentA, parentA2, parentB string
	childA1, childA2           string
	childB1, childB2           string
}

func plantComposite(t *testing.T, e compositeEdge, tag string) compositeFixture {
	t.Helper()
	f := compositeFixture{}
	f.token, _ = register(t, "composite-"+tag+"@example.com", "composite-pw-"+tag, "Composite "+tag)
	f.wsA = bootstrapWorkspace(t, f.token, "Composite A "+tag, "composite-a-"+tag)
	f.wsB = bootstrapWorkspace(t, f.token, "Composite B "+tag, "composite-b-"+tag)

	// Same names in both Workspaces: only the Workspace distinguishes them.
	f.parentA = e.newParent(t, f.token, f.wsA, "Platform", "platform")
	f.parentB = e.newParent(t, f.token, f.wsB, "Platform", "platform")
	f.parentA2 = e.newParent(t, f.token, f.wsA, "Platform next", "platform-next")
	f.childA1 = e.newChild(t, f.token, f.wsA, "Shared one")
	f.childA2 = e.newChild(t, f.token, f.wsA, "Shared two")
	f.childB1 = e.newChild(t, f.token, f.wsB, "Shared one")
	f.childB2 = e.newChild(t, f.token, f.wsB, "Shared two")

	gqlExecData(t, connectMutation(e), map[string]any{"parentID": f.parentA, "childIDs": []string{f.childA1}},
		workspaceHeaders(f.token, f.wsA), &map[string]any{})
	gqlExecData(t, connectMutation(e), map[string]any{"parentID": f.parentB, "childIDs": []string{f.childB1}},
		workspaceHeaders(f.token, f.wsB), &map[string]any{})

	// The foreign child: B's row keyed (B, parentA). The composite FK forbids it,
	// so it is written with FK triggers disabled for this one session.
	setForeignKeyUnchecked(t, e, f.childB2, &f.parentA)
	t.Cleanup(func() { setForeignKeyUnchecked(t, e, f.childB2, nil) })
	return f
}

// setForeignKeyUnchecked writes childTable.childFKCol on one row with the
// foreign-key triggers off (session_replication_role = replica, superuser-only;
// the testcontainers user is a superuser).
func setForeignKeyUnchecked(t *testing.T, e compositeEdge, childID string, parentID *string) {
	t.Helper()
	ctx := context.Background()
	conn, err := testPool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "SET session_replication_role = replica"); err != nil {
		t.Fatalf("disable FK triggers: %v", err)
	}
	defer func() { _, _ = conn.Exec(ctx, "SET session_replication_role = DEFAULT") }()
	q := fmt.Sprintf("UPDATE %s SET %s = $1 WHERE id = $2", e.childTable, e.childFKCol)
	if _, err := conn.Exec(ctx, q, parentID, childID); err != nil {
		t.Fatalf("plant foreign child: %v", err)
	}
}

// childParent reads childFKCol straight from Postgres (no cache, no tenancy).
func childParent(t *testing.T, e compositeEdge, childID string) *string {
	t.Helper()
	var out *string
	q := fmt.Sprintf("SELECT %s::text FROM %s WHERE id = $1", e.childFKCol, e.childTable)
	if err := testPool.QueryRow(context.Background(), q, childID).Scan(&out); err != nil {
		t.Fatalf("read %s.%s: %v", e.childTable, e.childFKCol, err)
	}
	return out
}

func assertChildParent(t *testing.T, e compositeEdge, label, childID string, want *string) {
	t.Helper()
	got := childParent(t, e, childID)
	if (got == nil) != (want == nil) || (got != nil && *got != *want) {
		t.Errorf("%s: %s.%s = %v, want %v", label, e.childTable, e.childFKCol, deref(got), deref(want))
	}
}

func deref(s *string) string {
	if s == nil {
		return "<null>"
	}
	return *s
}

func connectMutation(e compositeEdge) string {
	return fmt.Sprintf(`mutation ($parentID: UUID!, $childIDs: [UUID!]!) {
		%s(id: $parentID, input: { %s: { connect: $childIDs } }) { id %s { id } }
	}`, e.nestedMutation, e.childField, e.childField)
}

// gqlChildren loads parent { children { id } } over HTTP. ok is false when the
// parent itself is not visible (an error or a null parent).
func gqlChildren(t *testing.T, e compositeEdge, token, wsID, parentID string) (ids []string, ok bool) {
	t.Helper()
	q := fmt.Sprintf(`query ($id: UUID!) { %s(id: $id) { %s { id } } }`, e.parentField, e.childField)
	resp := gqlExec(t, q, map[string]any{"id": parentID}, workspaceHeaders(token, wsID))
	var out map[string]map[string][]struct {
		ID string `json:"id"`
	}
	if len(resp.Errors) > 0 || resp.Data == nil {
		return nil, false
	}
	decode(t, resp, &out)
	parent, present := out[e.parentField]
	if !present || parent == nil {
		return nil, false
	}
	for _, c := range parent[e.childField] {
		ids = append(ids, c.ID)
	}
	slices.Sort(ids)
	return ids, true
}

// gqlParentsWithChild lists parents whose relationship filter matches
// children { <field>: { eq: value } }.
func gqlParentsWithChild(t *testing.T, e compositeEdge, token, wsID, field, value string) []string {
	t.Helper()
	// The GraphQL <T>Filter inputs carry no `id` member, so the probe goes by a
	// String column; the planted data gives both Workspaces the same names.
	q := fmt.Sprintf(`query ($v: String!) { %s(filter: { %s: { %s: { eq: $v } } }) { edges { node { id } } } }`,
		e.listField, e.childField, field)
	var out map[string]struct {
		Edges []struct {
			Node struct {
				ID string `json:"id"`
			} `json:"node"`
		} `json:"edges"`
	}
	gqlExecData(t, q, map[string]any{"v": value}, workspaceHeaders(token, wsID), &out)
	var ids []string
	for _, ed := range out[e.listField].Edges {
		ids = append(ids, ed.Node.ID)
	}
	slices.Sort(ids)
	return ids
}

func sorted(ids ...string) []string {
	out := slices.Clone(ids)
	slices.Sort(out)
	return out
}

func TestCompositeEdgeCycleTasks(t *testing.T)   { runCompositeEdge(t, cycleTasksEdge, "cyc") }
func TestCompositeEdgeTeamProjects(t *testing.T) { runCompositeEdge(t, teamProjectsEdge, "team") }

func runCompositeEdge(t *testing.T, e compositeEdge, tag string) {
	t.Run("load", func(t *testing.T) {
		f := plantComposite(t, e, tag+"-load")

		t.Run("as workspace A", func(t *testing.T) {
			got, ok := gqlChildren(t, e, f.token, f.wsA, f.parentA)
			if !ok || !slices.Equal(got, sorted(f.childA1)) {
				t.Errorf("A: %s(A).%s = %v (visible=%v), want [childA1 %s] — not B's foreign child %s",
					e.parentField, e.childField, got, ok, f.childA1, f.childB2)
			}
		})
		t.Run("as workspace B", func(t *testing.T) {
			got, ok := gqlChildren(t, e, f.token, f.wsB, f.parentB)
			if !ok || !slices.Equal(got, sorted(f.childB1)) {
				t.Errorf("B: %s(B).%s = %v (visible=%v), want [childB1 %s]", e.parentField, e.childField, got, ok, f.childB1)
			}
			if got, ok := gqlChildren(t, e, f.token, f.wsB, f.parentA); ok {
				t.Errorf("B: %s(A) is visible from B with %s %v; want not found", e.parentField, e.childField, got)
			}
		})
		t.Run("SkipTenancy", func(t *testing.T) {
			pA, pB := uuid.MustParse(f.parentA), uuid.MustParse(f.parentB)
			got, err := e.loadChildren(context.Background(), []uuid.UUID{pA, pB})
			if err != nil {
				t.Fatalf("SkipTenancy load: %v", err)
			}
			if want := []uuid.UUID{uuid.MustParse(f.childA1)}; !slices.Equal(got[pA], want) {
				t.Errorf("SkipTenancy: parentA.%s = %v, want %v (B's foreign child %s keyed (B, parentA) must not join)",
					e.childField, got[pA], want, f.childB2)
			}
			if want := []uuid.UUID{uuid.MustParse(f.childB1)}; !slices.Equal(got[pB], want) {
				t.Errorf("SkipTenancy: parentB.%s = %v, want %v", e.childField, got[pB], want)
			}
		})
	})

	t.Run("filter", func(t *testing.T) {
		f := plantComposite(t, e, tag+"-filter")

		t.Run("as workspace A", func(t *testing.T) {
			if got := gqlParentsWithChild(t, e, f.token, f.wsA, e.childNameField, "Shared one"); !slices.Equal(got, sorted(f.parentA)) {
				t.Errorf("A: %s with %s named 'Shared one' = %v, want [parentA %s]", e.listField, e.childField, got, f.parentA)
			}
			// "Shared two" exists in A (childA2, unlinked) and in B (childB2,
			// keyed (B, parentA)): no A parent holds a child of that name.
			if got := gqlParentsWithChild(t, e, f.token, f.wsA, e.childNameField, "Shared two"); len(got) != 0 {
				t.Errorf("A: %s with a %s named 'Shared two' = %v, want none (B's foreign child must not match parentA)", e.listField, e.childField, got)
			}
		})
		t.Run("as workspace B", func(t *testing.T) {
			if got := gqlParentsWithChild(t, e, f.token, f.wsB, e.childNameField, "Shared one"); !slices.Equal(got, sorted(f.parentB)) {
				t.Errorf("B: %s with %s named 'Shared one' = %v, want [parentB %s]", e.listField, e.childField, got, f.parentB)
			}
			if got := gqlParentsWithChild(t, e, f.token, f.wsB, e.childNameField, "Shared two"); len(got) != 0 {
				t.Errorf("B: %s with a %s named 'Shared two' = %v, want none (B's foreign child names no parent in B)", e.listField, e.childField, got)
			}
		})
		t.Run("SkipTenancy", func(t *testing.T) {
			parents := []uuid.UUID{uuid.MustParse(f.parentA), uuid.MustParse(f.parentA2), uuid.MustParse(f.parentB)}
			got, err := e.parentsWith(context.Background(), parents, uuid.MustParse(f.childB2))
			if err != nil {
				t.Fatalf("SkipTenancy filter: %v", err)
			}
			if len(got) != 0 {
				t.Errorf("SkipTenancy: parents with B's foreign child = %v, want none (a %s-only EXISTS would match parentA %s)", got, e.childFKCol, f.parentA)
			}
			got, err = e.parentsWith(context.Background(), parents, uuid.MustParse(f.childA1))
			if err != nil {
				t.Fatalf("SkipTenancy filter: %v", err)
			}
			if !slices.Equal(got, []uuid.UUID{uuid.MustParse(f.parentA)}) {
				t.Errorf("SkipTenancy: parents with childA1 = %v, want [parentA %s]", got, f.parentA)
			}
		})
	})

	t.Run("nested writes", func(t *testing.T) {
		t.Run("as workspace A", func(t *testing.T) {
			f := plantComposite(t, e, tag+"-nwa")
			h := workspaceHeaders(f.token, f.wsA)
			// connect an unlinked child.
			gqlExecData(t, connectMutation(e), map[string]any{"parentID": f.parentA, "childIDs": []string{f.childA2}}, h, &map[string]any{})
			assertChildParent(t, e, "A connect", f.childA2, &f.parentA)
			// allow_reparent: move childA1 from parentA to parentA2.
			gqlExecData(t, connectMutation(e), map[string]any{"parentID": f.parentA2, "childIDs": []string{f.childA1}}, h, &map[string]any{})
			assertChildParent(t, e, "A reparent", f.childA1, &f.parentA2)
			// cross-Workspace connect: B's child into A's parent.
			resp := gqlExec(t, connectMutation(e), map[string]any{"parentID": f.parentA, "childIDs": []string{f.childB1}}, h)
			if len(resp.Errors) == 0 {
				t.Error("A: connected B's child into A's parent; cross-Workspace connect not refused")
			} else if c := errorCode(resp.Errors[0]); c != "NOT_FOUND" {
				t.Errorf("A: cross-Workspace connect error code = %q, want NOT_FOUND (must not confirm the row exists)", c)
			}
			assertChildParent(t, e, "A cross-Workspace connect", f.childB1, &f.parentB)
			assertChildParent(t, e, "A leaves foreign child", f.childB2, &f.parentA)
		})
		t.Run("as workspace B", func(t *testing.T) {
			f := plantComposite(t, e, tag+"-nwb")
			h := workspaceHeaders(f.token, f.wsB)
			// connect B's foreign child to B's own parent: a same-Workspace
			// reparent, which repairs the planted key to (B, parentB).
			gqlExecData(t, connectMutation(e), map[string]any{"parentID": f.parentB, "childIDs": []string{f.childB2}}, h, &map[string]any{})
			assertChildParent(t, e, "B connect", f.childB2, &f.parentB)
			// cross-Workspace connect: A's child into B's parent.
			resp := gqlExec(t, connectMutation(e), map[string]any{"parentID": f.parentB, "childIDs": []string{f.childA2}}, h)
			if len(resp.Errors) == 0 {
				t.Error("B: connected A's child into B's parent; cross-Workspace connect not refused")
			}
			assertChildParent(t, e, "B cross-Workspace connect", f.childA2, nil)
			// A's parent is invisible from B.
			resp = gqlExec(t, connectMutation(e), map[string]any{"parentID": f.parentA, "childIDs": []string{f.childB1}}, h)
			if len(resp.Errors) == 0 {
				t.Error("B: wrote through A's parent from B; parent not resolved tenant-scoped")
			}
			assertChildParent(t, e, "B via A's parent", f.childB1, &f.parentB)
		})
		t.Run("SkipTenancy", func(t *testing.T) {
			f := plantComposite(t, e, tag+"-nws")
			ctx := context.Background()
			pA, pA2 := uuid.MustParse(f.parentA), uuid.MustParse(f.parentA2)

			// Same-Workspace connect and reparent work without a tenant.
			if err := e.nested(ctx, pA, []uuid.UUID{uuid.MustParse(f.childA2)}, nil, false); err != nil {
				t.Fatalf("SkipTenancy connect: %v", err)
			}
			assertChildParent(t, e, "SkipTenancy connect", f.childA2, &f.parentA)
			if err := e.nested(ctx, pA2, []uuid.UUID{uuid.MustParse(f.childA1)}, nil, false); err != nil {
				t.Fatalf("SkipTenancy reparent: %v", err)
			}
			assertChildParent(t, e, "SkipTenancy reparent", f.childA1, &f.parentA2)

			// Cross-Workspace connect: B's child is visible without a tenant, so
			// only the composite key can refuse it — its workspace_id is a key
			// member this edge cannot write.
			err := e.nested(ctx, pA, []uuid.UUID{uuid.MustParse(f.childB1)}, nil, false)
			if err == nil {
				t.Error("SkipTenancy: connected B's child into A's parent; composite key not enforced on connect")
			} else if !errors.Is(err, database.ErrAlreadyRelated) {
				t.Errorf("SkipTenancy cross-Workspace connect error = %v, want ErrAlreadyRelated", err)
			}
			assertChildParent(t, e, "SkipTenancy cross-Workspace connect", f.childB1, &f.parentB)

			// disconnect names B's foreign child (cycle_id/team_id = parentA): it
			// is not on the (A, parentA) key, so it must stay put.
			if err := e.nested(ctx, pA, nil, []uuid.UUID{uuid.MustParse(f.childB2)}, false); err != nil {
				t.Fatalf("SkipTenancy disconnect: %v", err)
			}
			assertChildParent(t, e, "SkipTenancy disconnect of foreign child", f.childB2, &f.parentA)

			// clear unlinks parentA's own children only.
			if err := e.nested(ctx, pA, nil, nil, true); err != nil {
				t.Fatalf("SkipTenancy clear: %v", err)
			}
			assertChildParent(t, e, "SkipTenancy clear (own child)", f.childA2, nil)
			assertChildParent(t, e, "SkipTenancy clear (foreign child)", f.childB2, &f.parentA)
			assertChildParent(t, e, "SkipTenancy clear (other parent)", f.childA1, &f.parentA2)
		})
	})
}
