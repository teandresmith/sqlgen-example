package graph

// Hand-written helpers for the auth resolvers. This file is NOT a gqlgen
// resolver file (it does not match the `*.resolvers.go` glob), so gqlgen's
// resolvergen leaves it untouched on regeneration — helper methods belong here,
// not in auth.resolvers.go, which gqlgen would relocate on the next `generate`.

import (
	"fmt"

	"github.com/teandresmith/sqlgen-example/internal/database"
	"github.com/teandresmith/sqlgen-example/internal/graph/model"
)

// authPayload mints a token for the given User and wraps it with the User in an
// AuthPayload — the shared tail of the register and login resolvers.
func (r *mutationResolver) authPayload(user *database.User) (*model.AuthPayload, error) {
	token, err := r.Auth.Mint(user.ID)
	if err != nil {
		return nil, fmt.Errorf("minting token: %w", err)
	}
	return &model.AuthPayload{Token: token, User: user}, nil
}
