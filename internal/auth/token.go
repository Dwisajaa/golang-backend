package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"math/big"
)

// TokenGenerator creates opaque bearer tokens. The raw token is given to the
// client once; only its SHA-256 hash is ever persisted (Sanctum parity).
type TokenGenerator interface {
	Generate() (rawToken string, tokenHash string, err error)
	Hash(rawToken string) string
}

const rawTokenAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// RawTokenLength matches Laravel Sanctum's 40-character random token.
// 40 × log2(62) ≈ 238 bits of entropy per token.
const RawTokenLength = 40

// RandomTokenGenerator produces 40 alphanumeric characters from crypto/rand.
type RandomTokenGenerator struct{}

func NewRandomTokenGenerator() *RandomTokenGenerator { return &RandomTokenGenerator{} }

func (g *RandomTokenGenerator) Generate() (string, string, error) {
	raw := make([]byte, RawTokenLength)
	base := big.NewInt(int64(len(rawTokenAlphabet)))
	for i := range raw {
		n, err := rand.Int(rand.Reader, base)
		if err != nil {
			return "", "", err
		}
		raw[i] = rawTokenAlphabet[n.Int64()]
	}
	return string(raw), g.Hash(string(raw)), nil
}

// Hash returns the hex SHA-256 of a raw token — deterministic, one-way.
func (g *RandomTokenGenerator) Hash(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(sum[:])
}

var _ TokenGenerator = (*RandomTokenGenerator)(nil)
