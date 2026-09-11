package graph

// Hand-written helpers for the membership-administration resolvers (issue 04).
// Like auth_helpers.go, this file does not match the `*.resolvers.go` glob, so
// gqlgen leaves it untouched on regeneration.

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"
)

// invitationTTL is how long a new Invitation stays acceptable. Real email
// delivery is out of scope (dev flow), so this is generous — a week is enough
// for a person to register and accept.
const invitationTTL = 7 * 24 * time.Hour

// newInvitationToken returns an opaque, URL-safe acceptance token with 256 bits
// of entropy. The token is the bearer credential that authorizes acceptance, so
// it must be unguessable; crypto/rand, not math/rand.
func newInvitationToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generating invitation token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}
