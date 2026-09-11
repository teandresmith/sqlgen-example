package test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/teandresmith/sqlgen-example/internal/server"
)

// TestHealthEndpoint proves the readiness endpoint reports live and reflects a
// real database ping (the whole infra → client vertical is up).
func TestHealthEndpoint(t *testing.T) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, testServer.URL+server.HealthPath, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("send request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("health status = %d, want 200; body=%s", resp.StatusCode, body)
	}
	var out struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode health body: %v", err)
	}
	if out.Status != "ok" {
		t.Errorf("status = %q, want %q", out.Status, "ok")
	}
}

// TestSchemaIntrospection proves the generated executable schema is served and
// the gqlgen handler answers introspection through the wired seam.
func TestSchemaIntrospection(t *testing.T) {
	const query = `{ __schema { queryType { name } mutationType { name } } }`
	var out struct {
		Schema struct {
			QueryType    struct{ Name string } `json:"queryType"`
			MutationType struct{ Name string } `json:"mutationType"`
		} `json:"__schema"`
	}
	gqlExecData(t, query, nil, nil, &out)
	if out.Schema.QueryType.Name != "Query" {
		t.Errorf("queryType = %q, want %q", out.Schema.QueryType.Name, "Query")
	}
	if out.Schema.MutationType.Name != "Mutation" {
		t.Errorf("mutationType = %q, want %q", out.Schema.MutationType.Name, "Mutation")
	}
}

// TestUntenantedUserRoundtrip drives the whole vertical end to end: a mutation
// writes an (untenanted, shared) User through the generated client to real
// Postgres, and a follow-up query reads it back through the same seam. Users
// are shared per ADR-0004, so this works before any tenant is established —
// exactly the property the walking skeleton must prove.
func TestUntenantedUserRoundtrip(t *testing.T) {
	const createMut = `
		mutation ($inputs: [CreateUserInput!]!) {
			createUsers(inputs: $inputs) { id email displayName }
		}`
	createVars := map[string]any{
		"inputs": []map[string]any{{
			"email":        "skeleton@example.com",
			"displayName":  "Walking Skeleton",
			"passwordHash": "not-a-real-hash",
			"createdAt":    "2026-07-13T00:00:00Z",
			"updatedAt":    "2026-07-13T00:00:00Z",
		}},
	}
	var created struct {
		CreateUsers []struct {
			ID          string `json:"id"`
			Email       string `json:"email"`
			DisplayName string `json:"displayName"`
		} `json:"createUsers"`
	}
	gqlExecData(t, createMut, createVars, nil, &created)
	if len(created.CreateUsers) != 1 {
		t.Fatalf("createUsers returned %d users, want 1", len(created.CreateUsers))
	}
	if created.CreateUsers[0].Email != "skeleton@example.com" {
		t.Errorf("created email = %q, want %q", created.CreateUsers[0].Email, "skeleton@example.com")
	}
	if created.CreateUsers[0].ID == "" {
		t.Error("created user has empty id")
	}

	const readQuery = `
		query ($filter: UserFilter) {
			users(filter: $filter, first: 10) {
				totalCount
				edges { node { id email displayName } }
			}
		}`
	readVars := map[string]any{
		"filter": map[string]any{"email": map[string]any{"eq": "skeleton@example.com"}},
	}
	var read struct {
		Users struct {
			TotalCount int `json:"totalCount"`
			Edges      []struct {
				Node struct {
					ID          string `json:"id"`
					Email       string `json:"email"`
					DisplayName string `json:"displayName"`
				} `json:"node"`
			} `json:"edges"`
		} `json:"users"`
	}
	gqlExecData(t, readQuery, readVars, nil, &read)
	if read.Users.TotalCount != 1 {
		t.Fatalf("users totalCount = %d, want 1", read.Users.TotalCount)
	}
	if got := read.Users.Edges[0].Node.Email; got != "skeleton@example.com" {
		t.Errorf("read email = %q, want %q", got, "skeleton@example.com")
	}
	if read.Users.Edges[0].Node.ID != created.CreateUsers[0].ID {
		t.Errorf("read id = %q, want %q (same row written)", read.Users.Edges[0].Node.ID, created.CreateUsers[0].ID)
	}
}
