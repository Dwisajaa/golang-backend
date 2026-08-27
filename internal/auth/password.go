// Package auth owns credential and bearer-token primitives that must never
// depend on HTTP. Handlers and even services only call into these.
package auth

import "golang.org/x/crypto/bcrypt"

// BcryptCost follows the FASE 3 security decision (matches Laravel
// BCRYPT_ROUNDS=12).
const BcryptCost = 12

// PasswordHasher is the seam behind which bcrypt lives, so services can be
// tested with a fake and the algorithm can change without touching callers.
type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hash, password string) error
}

// BcryptHasher implements PasswordHasher with golang.org/x/crypto/bcrypt.
type BcryptHasher struct{}

func NewBcryptHasher() *BcryptHasher { return &BcryptHasher{} }

func (h *BcryptHasher) Hash(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	return string(b), err
}

func (h *BcryptHasher) Compare(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
