package test

import (
	"net/http"
	"testing"

	"uuid"
)

// bearerFor mints a JWT for userID with the harness auth service — the same
// signing code the server verifies with — and returns it as an Authorization
// header map ready for gqlExec.
func bearerFor(t *testing.T, userID uuid.UUID) map[string]string {
	t.Helper()
	token, err := testAuth.Mint(userID)
	if err != nil {
		t.Fatalf("mint token: %v", err)
	}
	return map[string]string{"Authorization": "Bearer " + token}
}

const registerMutation = `
	mutation ($input: RegisterInput!) {
		register(input: $input) { token user { id email displayName } }
	}`

type authUser struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
}

// register drives the register mutation and returns the token and created user.
func register(t *testing.T, email, password, displayName string) (string, authUser) {
	t.Helper()
	var out struct {
		Register struct {
			Token string   `json:"token"`
			User  authUser `json:"user"`
		} `json:"register"`
	}
	gqlExecData(t, registerMutation, map[string]any{
		"input": map[string]any{"email": email, "password": password, "displayName": displayName},
	}, nil, &out)
	return out.Register.Token, out.Register.User
}

// TestPasswordHashNotExposed proves users.password_hash is off the public API
// surface: it is not a field on the User type (PRD §32 write_only access),
// while register still succeeds. The credential's hashing is proven
// behaviorally by TestRegisterLoginMe (login verifies against the stored hash).
func TestPasswordHashNotExposed(t *testing.T) {
	_, user := register(t, "hash@example.com", "s3cr3t-pw", "Hash Tester")
	if user.ID == "" {
		t.Fatal("register returned empty user id")
	}
	// Introspect the User type — passwordHash must not appear among its fields.
	const q = `{ __type(name: "User") { fields { name } } }`
	var out struct {
		Type struct {
			Fields []struct {
				Name string `json:"name"`
			} `json:"fields"`
		} `json:"__type"`
	}
	gqlExecData(t, q, nil, nil, &out)
	for _, f := range out.Type.Fields {
		if f.Name == "passwordHash" {
			t.Error("User.passwordHash is exposed on the GraphQL API; write_only column must not be readable")
		}
	}
}

// TestRegisterLoginMe is the end-to-end tracer: register → login → an
// authenticated request (me) using the login-issued token (stories 1, 2).
func TestRegisterLoginMe(t *testing.T) {
	const (
		email    = "e2e@example.com"
		password = "hunter2-hunter2"
	)
	_, registered := register(t, email, password, "E2E User")

	// Login with the correct credentials returns a token.
	const loginMutation = `
		mutation ($input: LoginInput!) {
			login(input: $input) { token user { id email } }
		}`
	var login struct {
		Login struct {
			Token string   `json:"token"`
			User  authUser `json:"user"`
		} `json:"login"`
	}
	gqlExecData(t, loginMutation, map[string]any{
		"input": map[string]any{"email": email, "password": password},
	}, nil, &login)
	if login.Login.Token == "" {
		t.Fatal("login returned empty token")
	}
	if login.Login.User.ID != registered.ID {
		t.Errorf("login user id = %q, want %q", login.Login.User.ID, registered.ID)
	}

	// The login-issued token authenticates the `me` query.
	const meQuery = `{ me { id email } }`
	var me struct {
		Me authUser `json:"me"`
	}
	gqlExecData(t, meQuery, nil, map[string]string{"Authorization": "Bearer " + login.Login.Token}, &me)
	if me.Me.ID != registered.ID {
		t.Errorf("me id = %q, want %q", me.Me.ID, registered.ID)
	}
	if me.Me.Email != email {
		t.Errorf("me email = %q, want %q", me.Me.Email, email)
	}
}

// TestMintedTokenAuthenticates proves the JWT test helper (server's own signing
// code) mints tokens the server accepts.
func TestMintedTokenAuthenticates(t *testing.T) {
	_, user := register(t, "minted@example.com", "pw-minted-123", "Minted User")
	uid, err := uuid.Parse(user.ID)
	if err != nil {
		t.Fatalf("parse user id: %v", err)
	}
	const meQuery = `{ me { id email } }`
	var me struct {
		Me authUser `json:"me"`
	}
	gqlExecData(t, meQuery, nil, bearerFor(t, uid), &me)
	if me.Me.ID != user.ID {
		t.Errorf("me id = %q, want %q", me.Me.ID, user.ID)
	}
}

// TestLoginWrongPassword proves bad credentials are rejected.
func TestLoginWrongPassword(t *testing.T) {
	register(t, "wrongpw@example.com", "the-right-password", "PW User")
	const loginMutation = `
		mutation ($input: LoginInput!) {
			login(input: $input) { token }
		}`
	resp := gqlExec(t, loginMutation, map[string]any{
		"input": map[string]any{"email": "wrongpw@example.com", "password": "not-it"},
	}, nil)
	if len(resp.Errors) == 0 {
		t.Fatal("login with wrong password succeeded, want an error")
	}
}

// TestMeWithoutTokenRejected proves an authenticated operation is rejected when
// no bearer token is present (the request passes the middleware, but the
// resolver refuses). PRD story 10 fail-closed shape for authenticated ops.
func TestMeWithoutTokenRejected(t *testing.T) {
	const meQuery = `{ me { id } }`
	resp := gqlExec(t, meQuery, nil, nil)
	if len(resp.Errors) == 0 {
		t.Fatal("me without a token succeeded, want an error")
	}
}

// TestInvalidTokenRejectedAtMiddleware proves a present-but-invalid bearer
// token is rejected fail-closed with HTTP 401 by the middleware, before the
// GraphQL layer runs.
func TestInvalidTokenRejectedAtMiddleware(t *testing.T) {
	status := postStatus(t, `{ me { id } }`, map[string]string{"Authorization": "Bearer not-a-real-jwt"})
	if status != http.StatusUnauthorized {
		t.Errorf("HTTP status = %d, want 401 for an invalid token", status)
	}
}
