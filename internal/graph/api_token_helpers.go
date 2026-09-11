package graph

// Hand-written helpers for the API-token resolvers (issue 13). Like
// auth_helpers.go / membership_admin_helpers.go, this file does not match the
// `*.resolvers.go` glob, so gqlgen leaves it untouched on regeneration.

import (
	"context"
	"fmt"

	"github.com/teandresmith/sqlgen-example/internal/apitoken"
	"github.com/teandresmith/sqlgen-example/internal/database"
	"github.com/teandresmith/sqlgen-example/internal/graph/model"
	"github.com/teandresmith/sqlgen/omittable"
	"uuid"
)

// mintAPIToken is the shared tail of the personal and service token resolvers: it
// generates a secret, stores only its hash, and returns the created token with
// the plaintext secret (shown exactly once). owner carries the owning User for a
// personal token, or an invalid NullUUID for a service token (user_id → NULL).
// workspace_id is auto-injected by tenant scoping on Create.
func (r *mutationResolver) mintAPIToken(ctx context.Context, input model.CreateAPITokenInput, owner *uuid.UUID) (*model.CreateAPITokenResult, error) {
	secret, hash, err := apitoken.Generate()
	if err != nil {
		return nil, err
	}

	in := &database.CreateAPITokenInput{
		Name:      input.Name,
		TokenHash: hash,
	}
	if owner != nil {
		in.UserID = omittable.Set(owner)
	}
	if input.Scopes != nil {
		in.Scopes = omittable.Set(input.Scopes)
	}
	if input.ExpiresAt != nil {
		in.ExpiresAt = omittable.Set(input.ExpiresAt)
	}

	tok, err := r.Client.APITokens().Create(ctx, in)
	if err != nil {
		return nil, fmt.Errorf("creating api token: %w", err)
	}
	return &model.CreateAPITokenResult{APIToken: tok, Secret: secret}, nil
}
