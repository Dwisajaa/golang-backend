package repository

import (
	"context"

	"github.com/Dwisajaa/golang-backend/internal/model"
)

// UserRepository is the contract the user service needs. Defining it as an
// interface lets the service be tested with a fake and swapped to any
// data source without touching business code.
type UserRepository interface {
	FindByID(ctx context.Context, id uint64) (*model.User, error)
}
