package auth

import (
	"testing"
)

func TestGenerateReturnsRawAndHash(t *testing.T) {
	g := NewRandomTokenGenerator()
	raw, hash, err := g.Generate()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if raw == "" {
		t.Fatal("raw token must not be empty")
	}
	if hash == "" {
		t.Fatal("token hash must not be empty")
	}
	if len(raw) != RawTokenLength {
		t.Fatalf("raw token length %d != %d", len(raw), RawTokenLength)
	}
	if raw == hash {
		t.Fatal("raw token must differ from its hash")
	}
}

func TestGenerateIsUnique(t *testing.T) {
	g := NewRandomTokenGenerator()
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		raw, _, err := g.Generate()
		if err != nil {
			t.Fatalf("generate: %v", err)
		}
		if seen[raw] {
			t.Fatal("collision: generated token repeated")
		}
		seen[raw] = true
	}
}

func TestHashIsDeterministic(t *testing.T) {
	g := NewRandomTokenGenerator()
	a := g.Hash("abc123")
	b := g.Hash("abc123")
	if a != b {
		t.Fatal("hash must be deterministic for the same raw token")
	}
	if len(a) != 64 {
		t.Fatalf("expected 64 hex chars (SHA-256), got %d", len(a))
	}
}

func TestDifferentRawsHashDiffer(t *testing.T) {
	g := NewRandomTokenGenerator()
	if g.Hash("token-one") == g.Hash("token-two") {
		t.Fatal("different raw tokens must hash differently")
	}
}
