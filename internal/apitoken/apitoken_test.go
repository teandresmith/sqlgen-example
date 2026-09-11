package apitoken

import (
	"strings"
	"testing"
)

// TestGenerateProducesPrefixedSecretAndMatchingHash proves Generate returns a
// prefixed plaintext secret whose SHA-256 hash is exactly what Hash computes for
// it — the contract the middleware relies on to look a presented token up by hash.
func TestGenerateProducesPrefixedSecretAndMatchingHash(t *testing.T) {
	secret, hash, err := Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !strings.HasPrefix(secret, TokenPrefix) {
		t.Errorf("secret %q does not carry the %q prefix that classifies it as an API token", secret, TokenPrefix)
	}
	if got := Hash(secret); got != hash {
		t.Errorf("Hash(secret) = %q, want the hash Generate returned %q", got, hash)
	}
	if hash == secret {
		t.Error("the stored hash equals the plaintext secret; only the hash may be persisted")
	}
}

// TestHashIsDeterministic proves Hash is a pure function of its input, so a lookup
// by hash equality works: the same secret always hashes to the same digest.
func TestHashIsDeterministic(t *testing.T) {
	const secret = TokenPrefix + "fixed-input-for-determinism"
	first, second := Hash(secret), Hash(secret)
	if first != second {
		t.Errorf("Hash is not deterministic (%q vs %q); lookup by token_hash equality would fail", first, second)
	}
}

// TestGenerateIsUnique proves two mints do not collide — the secrets carry real
// entropy, so token_hash (a unique column) will not clash across tokens.
func TestGenerateIsUnique(t *testing.T) {
	seen := make(map[string]struct{}, 100)
	for range 100 {
		secret, hash, err := Generate()
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		if _, dup := seen[hash]; dup {
			t.Fatalf("Generate produced a duplicate token %q within 100 mints", secret)
		}
		seen[hash] = struct{}{}
	}
}
