package auth

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"uuid"
)

func TestMintParseRoundtrip(t *testing.T) {
	svc := NewService("test-secret", time.Hour)
	userID := uuid.New()

	token, err := svc.Mint(userID)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	got, err := svc.Parse(token)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got != userID {
		t.Errorf("parsed subject = %v, want %v", got, userID)
	}
}

func TestParseRejectsWrongSecret(t *testing.T) {
	minted, _ := NewService("secret-a", time.Hour).Mint(uuid.New())
	if _, err := NewService("secret-b", time.Hour).Parse(minted); err == nil {
		t.Fatal("Parse accepted a token signed with a different secret")
	}
}

func TestParseRejectsExpired(t *testing.T) {
	// Build the struct directly to bypass NewService's non-positive-ttl guard,
	// so Mint stamps an expiry a full hour in the past.
	svc := &Service{secret: []byte("test-secret"), ttl: -time.Hour}
	token, err := svc.Mint(uuid.New())
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	if _, err := svc.Parse(token); err == nil {
		t.Fatal("Parse accepted an expired token")
	}
}

func TestParseRejectsGarbage(t *testing.T) {
	svc := NewService("test-secret", time.Hour)
	if _, err := svc.Parse("not-a-jwt"); err == nil {
		t.Fatal("Parse accepted a non-token string")
	}
}

// TestParseRejectsOtherAlgorithm proves Parse pins HS256: a token signed with a
// different algorithm (even using the same secret) is rejected, closing the
// algorithm-substitution attack surface.
func TestParseRejectsOtherAlgorithm(t *testing.T) {
	const secret = "test-secret"
	claims := jwt.RegisteredClaims{
		Subject:   uuid.New().String(),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}
	hs512, err := jwt.NewWithClaims(jwt.SigningMethodHS512, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign HS512: %v", err)
	}
	if _, err := NewService(secret, time.Hour).Parse(hs512); err == nil {
		t.Fatal("Parse accepted an HS512 token, want HS256-only")
	}
}

func TestPasswordHashCheck(t *testing.T) {
	svc := NewService("test-secret", time.Hour)
	hash, err := svc.HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "correct horse battery staple" {
		t.Fatal("password stored in plaintext")
	}
	if err := svc.CheckPassword(hash, "correct horse battery staple"); err != nil {
		t.Errorf("CheckPassword rejected the correct password: %v", err)
	}
	if err := svc.CheckPassword(hash, "wrong"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("CheckPassword(wrong) = %v, want ErrInvalidCredentials", err)
	}
}
