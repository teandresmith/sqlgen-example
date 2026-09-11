package test

import (
	"context"
	"log/slog"
	"testing"

	"uuid"

	"github.com/teandresmith/sqlgen-example/internal/seed"
)

// TestSeedDataset drives the seeder through the harness client — the same
// *database.Client the server serves from — and then reads the result back
// through the wired GraphQL endpoint, proving issue 16's acceptance end to end:
// the seeded dataset spans multiple Workspaces with related entities, and every
// row is queryable through the API under the appropriate Active Workspace.
//
// It runs the seed once and asserts against it from several angles: the login
// of a seeded User (credential hashing works), the presence of each Workspace's
// Projects and their Tasks with relationships loaded, and — crucially — tenant
// isolation, that one Workspace's Active-Workspace view never sees another's
// rows.
func TestSeedDataset(t *testing.T) {
	summary, err := seed.Run(context.Background(), testClient, testAuth.HashPassword, slog.Default())
	if err != nil {
		t.Fatalf("seed.Run: %v", err)
	}

	if len(summary.Workspaces) != 2 {
		t.Fatalf("seeded %d workspaces, want 2", len(summary.Workspaces))
	}
	// Locate the two Workspaces by slug so the assertions do not depend on order.
	bySlug := map[string]seed.WorkspaceSummary{}
	for _, ws := range summary.Workspaces {
		bySlug[ws.Slug] = ws
	}
	acme, ok := bySlug["acme"]
	if !ok {
		t.Fatal("no acme workspace in summary")
	}
	globex, ok := bySlug["globex"]
	if !ok {
		t.Fatal("no globex workspace in summary")
	}

	// A seeded User can log in with the shared password — proves the seeder
	// stored a credential the real login path verifies against.
	aliceToken := login(t, acme.OwnerEmail, seed.DefaultPassword)
	daveToken := login(t, globex.OwnerEmail, seed.DefaultPassword)

	// Acme, viewed under its own Active Workspace, holds its two Projects and
	// none of Globex's.
	acmeProjects := projectNames(t, aliceToken, acme.ID)
	assertContains(t, "acme projects", acmeProjects, "Website Redesign", "Mobile App")
	assertExcludes(t, "acme projects", acmeProjects, "Billing Service")

	// Globex holds its one Project and none of Acme's — the isolation is
	// symmetric, so a leak in either direction fails the test.
	globexProjects := projectNames(t, daveToken, globex.ID)
	assertContains(t, "globex projects", globexProjects, "Billing Service")
	assertExcludes(t, "globex projects", globexProjects, "Website Redesign", "Mobile App")

	// The related entities load through the graph: the "Fix nav dropdown" Task
	// carries its assignee, its labels, and a subtask; "Set up design system
	// tokens" carries a comment. Reading them under Acme's Active Workspace
	// exercises the full relationship walk the seeder wrote.
	tasks := workspaceTasks(t, aliceToken, acme.ID)
	nav, ok := tasks["Fix nav dropdown on mobile"]
	if !ok {
		t.Fatal("seeded task 'Fix nav dropdown on mobile' not queryable under acme")
	}
	if nav.Status != "IN_PROGRESS" {
		t.Errorf("nav task status = %q, want IN_PROGRESS", nav.Status)
	}
	if len(nav.Assignees) == 0 {
		t.Error("nav task has no assignees; seeded assignment not queryable")
	}
	navLabels := make([]string, 0, len(nav.Labels))
	for _, l := range nav.Labels {
		navLabels = append(navLabels, l.Name)
	}
	assertContains(t, "nav task labels", navLabels, "bug", "urgent")
	if len(nav.Subtasks) == 0 {
		t.Error("nav task has no subtasks; seeded subtask not queryable")
	}

	tokens := tasks["Set up design system tokens"]
	if len(tokens.Comments) == 0 {
		t.Error("'Set up design system tokens' has no comments; seeded comment not queryable")
	}
}

// login drives the login mutation and returns the JWT, failing the test if the
// credentials are rejected.
func login(t *testing.T, email, password string) string {
	t.Helper()
	const m = `mutation ($input: LoginInput!) { login(input: $input) { token } }`
	var out struct {
		Login struct {
			Token string `json:"token"`
		} `json:"login"`
	}
	gqlExecData(t, m, map[string]any{
		"input": map[string]any{"email": email, "password": password},
	}, nil, &out)
	if out.Login.Token == "" {
		t.Fatalf("login for %s returned empty token", email)
	}
	return out.Login.Token
}

// projectNames lists the Project names visible under the given Active Workspace.
func projectNames(t *testing.T, token string, workspaceID uuid.UUID) []string {
	t.Helper()
	const q = `query { projectList(limit: 50) { items { name } } }`
	var out struct {
		ProjectList struct {
			Items []struct {
				Name string `json:"name"`
			} `json:"items"`
		} `json:"projectList"`
	}
	gqlExecData(t, q, nil, workspaceHeaders(token, workspaceID.String()), &out)
	names := make([]string, 0, len(out.ProjectList.Items))
	for _, p := range out.ProjectList.Items {
		names = append(names, p.Name)
	}
	return names
}

// seededTask is the projection of a Task and its relationships the test reads.
type seededTask struct {
	Title     string `json:"title"`
	Status    string `json:"status"`
	Assignees []struct {
		Email string `json:"email"`
	} `json:"assignees"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
	Subtasks []struct {
		ID string `json:"id"`
	} `json:"subtasks"`
	Comments []struct {
		Body string `json:"body"`
	} `json:"comments"`
}

// workspaceTasks lists every Task visible under the Active Workspace, keyed by
// title, with its relationships loaded.
func workspaceTasks(t *testing.T, token string, workspaceID uuid.UUID) map[string]seededTask {
	t.Helper()
	const q = `query {
		taskList(limit: 100) {
			items {
				title
				status
				assignees { email }
				labels { name }
				subtasks { id }
				comments { body }
			}
		}
	}`
	var out struct {
		TaskList struct {
			Items []seededTask `json:"items"`
		} `json:"taskList"`
	}
	gqlExecData(t, q, nil, workspaceHeaders(token, workspaceID.String()), &out)
	byTitle := make(map[string]seededTask, len(out.TaskList.Items))
	for _, task := range out.TaskList.Items {
		byTitle[task.Title] = task
	}
	return byTitle
}

func assertContains(t *testing.T, what string, got []string, want ...string) {
	t.Helper()
	set := map[string]bool{}
	for _, g := range got {
		set[g] = true
	}
	for _, w := range want {
		if !set[w] {
			t.Errorf("%s = %v, want it to contain %q", what, got, w)
		}
	}
}

func assertExcludes(t *testing.T, what string, got []string, unwanted ...string) {
	t.Helper()
	set := map[string]bool{}
	for _, g := range got {
		set[g] = true
	}
	for _, u := range unwanted {
		if set[u] {
			t.Errorf("%s = %v, want it to NOT contain %q (tenant isolation leak)", what, got, u)
		}
	}
}
