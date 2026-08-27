package repository

import (
	"context"

	"github.com/Dwisajaa/golang-backend/internal/model"
)

// UserStore is the slice of user persistence the auth service needs.
type UserStore interface {
	FindByEmail(ctx context.Context, email string) (*model.User, error)
	Create(ctx context.Context, u *model.User) error
}

// UserRepository is the full user persistence contract (auth + read paths).
type UserRepository interface {
	UserStore
	FindByID(ctx context.Context, id uint64) (*model.User, error)
}
