package repository

import (
	"context"

	"github.com/Dwisajaa/golang-backend/internal/model"
)

// TokenStore is the persistence contract for personal access tokens.
type TokenStore interface {
	Create(ctx context.Context, t *model.PersonalAccessToken) error
	FindByTokenHash(ctx context.Context, tokenHash string) (*model.PersonalAccessToken, error)
	RevokeByTokenHash(ctx context.Context, tokenHash string) error
	// RevokeAllForUser deletes every token of a user (Laravel tokens()->delete()
	// parity after a password change).
	RevokeAllForUser(ctx context.Context, q Queryer, userID uint64) error
}
