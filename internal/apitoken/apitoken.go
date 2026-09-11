// Package apitoken provides non-interactive authentication for the task tracker:
// it mints and hashes API Tokens and exposes the HTTP middleware that accepts a
// valid token as an alternative to a bearer JWT (issue 13, extending issue 02's
// auth middleware). An API Token is a hashed, workspace-bound credential a User
// creates so a service or script can call the API on their behalf (personal
// token) or so automation can authenticate with no personal account at all
// (service token, user_id NULL).
//
// Unlike the JWT — which carries only the User identity and leaves the Active
// Workspace to be selected per request via X-Workspace-Id (ADR-0004) — an API
// Token is self-contained: the row already pins the Workspace (workspace_id) and,
// for a personal token, the owning User. So a token-authenticated request needs
// no X-Workspace-Id header; the middleware establishes the Active Workspace (and,
// for a personal token, the identity and the owner's current Role) straight from
// the token and dispatches to the GraphQL handler, bypassing the JWT and
// header-tenancy middlewares entirely.
//
// Tokens are high-entropy random secrets, so they are hashed with SHA-256 rather
// than bcrypt: a fast, deterministic digest is both sufficient (there is no
// low-entropy password to brute-force) and necessary (lookup is by hash equality
// against the unique token_hash column, which a per-row bcrypt salt would make
// impossible). Only the hash is ever stored; the plaintext secret is returned to
// the creator exactly once. Token scope enforcement is out of scope (PRD) — the
// scopes are recorded but not yet checked.
package apitoken

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/teandresmith/sqlgen/comparator"
	"github.com/teandresmith/sqlgen/omittable"
	"uuid"

	"github.com/teandresmith/sqlgen-example/internal/auth"
	"github.com/teandresmith/sqlgen-example/internal/database"
	"github.com/teandresmith/sqlgen-example/internal/tenancy"
)

// TokenPrefix marks a plaintext API Token so the middleware can tell it apart
// from a bearer JWT in the same Authorization header. A JWT is base64url of a
// JSON header and always begins `eyJ`; this prefix never collides with that, so
// classification is unambiguous. The prefix is part of the hashed secret.
const TokenPrefix = "tskr_"

// secretEntropyBytes is the number of random bytes behind a token's secret. 256
// bits makes the secret unguessable, so an attacker cannot forge a token_hash
// preimage — the whole point of storing only the hash.
const secretEntropyBytes = 32

// errInvalidToken is returned for any credential that does not authenticate — an
// unknown, expired, or orphaned token. It is deliberately uniform so a caller
// cannot distinguish "no such token" from "expired" by the error alone.
var errInvalidToken = errors.New("invalid or expired api token")

// Generate mints a new token: a URL-safe plaintext secret (shown to the creator
// once) and its SHA-256 hash (all that is ever persisted). The secret carries
// TokenPrefix so the middleware can classify it on a later request.
func Generate() (secret, hash string, err error) {
	var b [secretEntropyBytes]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", "", fmt.Errorf("generating api token: %w", err)
	}
	secret = TokenPrefix + base64.RawURLEncoding.EncodeToString(b[:])
	return secret, Hash(secret), nil
}

// Hash returns the SHA-256 hex digest of a plaintext secret. It is the stored
// token_hash and the lookup key: the middleware hashes a presented token and
// queries token_hash for equality. Deterministic by design (see the package doc)
// — the secret's own entropy, not a salt, is what makes it unforgeable.
func Hash(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

// Middleware authenticates requests presenting an API Token. It sits outermost,
// ahead of the bearer-JWT middleware: a credential carrying TokenPrefix is an API
// Token, which this middleware validates and dispatches straight to apiHandler
// (the post-auth GraphQL handler) with the Active Workspace — and, for a personal
// token, the identity and Role — established in context. Anything else (a JWT, or
// no credential at all) falls through to next, the interactive JWT + header-
// tenancy chain, unchanged.
//
// A present-but-invalid API Token is rejected fail-closed with 401: the caller
// asserted a credential that does not verify, so — as with the JWT middleware —
// the request does not silently continue unauthenticated.
func Middleware(client *database.Client, apiHandler http.Handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			secret, ok := bearerToken(r)
			if !ok || !strings.HasPrefix(secret, TokenPrefix) {
				next.ServeHTTP(w, r)
				return
			}
			ctx, err := authenticate(r.Context(), client, secret)
			if err != nil {
				writeUnauthorized(w)
				return
			}
			apiHandler.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// authenticate looks a presented token up by hash and, when valid, returns a
// context carrying the token's Active Workspace plus — for a personal token — the
// owning User identity and their current Role in that Workspace. A service token
// (user_id NULL) carries the Active Workspace but no identity and no Role, so it
// authenticates for ordinary member work yet is fail-closed on the role-guarded
// administrative surfaces (internal/authz) — a sensible default given token scope
// enforcement is out of scope.
func authenticate(ctx context.Context, client *database.Client, secret string) (context.Context, error) {
	hash := Hash(secret)
	limit := 1
	// The token lookup precedes any Active Workspace, so it bypasses tenant
	// scoping (like the pre-tenant Membership lookup in tenancy.Middleware). The
	// unique token_hash pins the row; GetMany is not cached, so no stale read.
	tokens, err := client.APITokens().GetMany(ctx, &database.GetAPITokensInput{
		Filter: &database.APITokenFilter{TokenHash: &comparator.String{Eq: &hash}},
		Limit:  &limit,
	}, skipTokenTenancy)
	if err != nil {
		return nil, err
	}
	if len(tokens) == 0 {
		return nil, errInvalidToken
	}
	tok := tokens[0]

	if tok.ExpiresAt != nil && time.Now().After(*tok.ExpiresAt) {
		return nil, errInvalidToken
	}

	ctx = tenancy.WithActiveWorkspace(ctx, tok.WorkspaceID)

	if tok.UserID != nil {
		// Personal token: carry the owner's identity and their *current* Role, so a
		// token grants exactly what its owner may do today. If the owner has since
		// lost their Membership the token is orphaned and cannot authenticate —
		// mirroring how a JWT holder with no Membership is rejected at the tenancy
		// middleware.
		membership, err := client.Memberships().Get(ctx,
			database.MembershipPK{WorkspaceID: tok.WorkspaceID, UserID: *tok.UserID},
			skipMembershipTenancy,
		)
		if errors.Is(err, database.ErrNotFound) {
			return nil, errInvalidToken
		}
		if err != nil {
			return nil, err
		}
		ctx = auth.WithUserID(ctx, *tok.UserID)
		ctx = tenancy.WithRole(ctx, membership.Role)
	}

	touchLastUsed(ctx, client, tok.ID)
	return ctx, nil
}

// touchLastUsed records that the token just authenticated a request. It is
// best-effort: a failure to stamp last_used_at must not fail an otherwise valid
// request, so the error is ignored. The write skips hooks deliberately — this is
// bookkeeping, not a domain mutation, so it must not emit an Activity event on
// every request or churn the cache; and it skips tenancy because the PK already
// pins the row (the lookup above was cross-tenant).
func touchLastUsed(ctx context.Context, client *database.Client, id uuid.UUID) {
	now := time.Now()
	_, _ = client.APITokens().Update(ctx, id, &database.UpdateAPITokenInput{
		LastUsedAt: omittable.Set(&now),
	}, func(o *database.CallOptions[database.APITokenFieldOptions]) {
		o.SkipTenancy = true
		o.SkipHooks = true
	})
}

// skipTokenTenancy bypasses tenant auto-filtering for the pre-tenant token
// lookup (no Active Workspace is established yet; token_hash is globally unique).
func skipTokenTenancy(o *database.CallOptions[database.APITokenFieldOptions]) {
	o.SkipTenancy = true
}

// skipMembershipTenancy bypasses tenant auto-filtering for the owner's Membership
// lookup — it runs before the token's Active Workspace is in force, and the
// composite PK already pins the (workspace, user) pair.
func skipMembershipTenancy(o *database.CallOptions[database.MembershipFieldOptions]) {
	o.SkipTenancy = true
}

// bearerToken extracts the token from an "Authorization: Bearer <token>" header,
// reporting whether a bearer credential was present. It mirrors the JWT
// middleware's extractor so both read the same header the same way.
func bearerToken(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(h, prefix) {
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(h, prefix))
	return token, token != ""
}

// writeUnauthorized emits a 401 with a small JSON body, matching the shape the
// JWT middleware uses so a rejected credential looks the same whichever kind it
// claimed to be.
func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": errInvalidToken.Error()})
}
