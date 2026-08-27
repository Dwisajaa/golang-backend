package auth

import (
	"strings"
	"testing"
)

func TestHashProducesBcrypt(t *testing.T) {
	h := NewBcryptHasher()
	got, err := h.Hash("password123")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if !strings.HasPrefix(got, "$2a$12$") && !strings.HasPrefix(got, "$2b$12$") {
		t.Fatalf("expected bcrypt cost 12, got %q", got)
	}
	if got == "password123" {
		t.Fatal("hash must never equal plaintext")
	}
}

func TestCompareCorrectPassword(t *testing.T) {
	h := NewBcryptHasher()
	hash, _ := h.Hash("password123")
	if err := h.Compare(hash, "password123"); err != nil {
		t.Fatalf("expected match, got %v", err)
	}
}

func TestCompareWrongPassword(t *testing.T) {
	h := NewBcryptHasher()
	hash, _ := h.Hash("password123")
	if err := h.Compare(hash, "wrong"); err == nil {
		t.Fatal("expected mismatch error")
	}
}

func TestHashSaltProducesDifferentOutputs(t *testing.T) {
	h := NewBcryptHasher()
	a, _ := h.Hash("same-password")
	b, _ := h.Hash("same-password")
	if a == b {
		t.Fatal("bcrypt includes a random salt; two hashes must differ")
	}
	// both still verify the same password
	if err := h.Compare(a, "same-password"); err != nil {
		t.Fatalf("a verify: %v", err)
	}
	if err := h.Compare(b, "same-password"); err != nil {
		t.Fatalf("b verify: %v", err)
	}
}

func TestConfiguredCost(t *testing.T) {
	_, err := NewBcryptHasher().Hash("cost-check")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if BcryptCost != 12 {
		t.Fatalf("BcryptCost should stay 12 (FASE 3 decision), got %d", BcryptCost)
	}
}
