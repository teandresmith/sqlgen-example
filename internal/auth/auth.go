// Package auth provides interactive authentication for the task tracker: it
// hashes and verifies passwords, mints and parses the JWTs a login returns, and
// exposes the bearer-token HTTP middleware that puts the authenticated User
// into the request context. Per ADR-0004 a token carries the User identity only
// — it is not bound to any Workspace, so a User can act in many Workspaces with
// one token (the Active Workspace is selected per request, in a later ticket).
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"uuid"
)

// ErrInvalidCredentials is returned by a login flow when the email is unknown
// or the password does not match. It is deliberately identical for both cases
// so callers cannot use it to probe which emails are registered.
var ErrInvalidCredentials = errors.New("invalid email or password")

// DefaultTokenTTL is how long a minted token stays valid unless overridden.
const DefaultTokenTTL = 24 * time.Hour

// Service signs and verifies JWTs with a single HMAC secret and hashes
// passwords with bcrypt. One Service is built at boot from the configured JWT
// secret and shared across the resolver and the middleware.
type Service struct {
	secret []byte
	ttl    time.Duration
}

// NewService builds a Service from the raw JWT secret. A non-positive ttl falls
// back to DefaultTokenTTL.
func NewService(secret string, ttl time.Duration) *Service {
	if ttl <= 0 {
		ttl = DefaultTokenTTL
	}
	return &Service{secret: []byte(secret), ttl: ttl}
}

// HashPassword returns the bcrypt hash of a plaintext password, suitable for
// storing in users.password_hash.
func (s *Service) HashPassword(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hashing password: %w", err)
	}
	return string(h), nil
}

// CheckPassword reports whether password matches the stored bcrypt hash,
// returning ErrInvalidCredentials on any mismatch.
func (s *Service) CheckPassword(hash, password string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return ErrInvalidCredentials
	}
	return nil
}

// dummyHash is a valid bcrypt hash computed once, used only to burn comparison
// time. Its plaintext is irrelevant — it exists so an unknown-email login costs
// the same wall-clock time as a real password check.
var dummyHash = sync.OnceValue(func() string {
	h, _ := bcrypt.GenerateFromPassword([]byte("timing-equalizer"), bcrypt.DefaultCost)
	return string(h)
})

// EqualizeCompareCost performs a throwaway bcrypt comparison so a login for an
// unknown email spends the same time as one for a known email — closing the
// timing side-channel a caller could otherwise use to enumerate accounts.
func (s *Service) EqualizeCompareCost(password string) {
	_ = bcrypt.CompareHashAndPassword([]byte(dummyHash()), []byte(password))
}

// Mint issues a signed JWT whose subject is the User's id. The token carries no
// Workspace binding (ADR-0004).
func (s *Service) Mint(userID uuid.UUID) (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Subject:   userID.String(),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(s.ttl)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.secret)
	if err != nil {
		return "", fmt.Errorf("signing token: %w", err)
	}
	return signed, nil
}

// Parse verifies a token's signature and expiry and returns the User id from
// its subject. It accepts only HS256 — the exact algorithm Mint issues — so a
// caller cannot downgrade the algorithm (e.g. to "none") or slip in another
// HMAC variant.
func (s *Service) Parse(tokenString string) (uuid.UUID, error) {
	var claims jwt.RegisteredClaims
	_, err := jwt.ParseWithClaims(tokenString, &claims, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("parsing token: %w", err)
	}
	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("token subject is not a valid user id: %w", err)
	}
	return userID, nil
}

// ctxKey is the unexported context-key type for the authenticated User id.
type ctxKey struct{}

// WithUserID returns a copy of ctx carrying the authenticated User id.
func WithUserID(ctx context.Context, userID uuid.UUID) context.Context {
	return context.WithValue(ctx, ctxKey{}, userID)
}

// UserID returns the authenticated User id from ctx and whether one is present.
// Resolvers for authenticated operations use the ok result to reject
// unauthenticated callers.
func UserID(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(ctxKey{}).(uuid.UUID)
	return id, ok
}

// Middleware validates the request's bearer token and, when valid, places the
// User id into the request context. A request with no Authorization header
// passes through unauthenticated — public operations like register and login
// (and schema introspection) share the single GraphQL endpoint, so absence of a
// token is not an HTTP error; authenticated resolvers reject it via UserID. A
// present-but-invalid token is rejected fail-closed with 401, since the caller
// asserted an identity that does not verify.
func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearerToken(r)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		userID, err := s.Parse(token)
		if err != nil {
			writeUnauthorized(w)
			return
		}
		next.ServeHTTP(w, r.WithContext(WithUserID(r.Context(), userID)))
	})
}

// bearerToken extracts the token from an "Authorization: Bearer <token>"
// header, reporting whether a bearer credential was present.
func bearerToken(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	if h == "" {
		return "", false
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(h, prefix) {
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(h, prefix))
	return token, token != ""
}

// writeUnauthorized emits a 401 with a small JSON body.
func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid or expired token"})
}
