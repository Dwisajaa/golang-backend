package service

import (
	"context"
	"errors"

	"github.com/Dwisajaa/golang-backend/internal/httperr"
	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/repository"
)

// UserReader is the read surface UserService depends on.
type UserReader interface {
	FindByID(ctx context.Context, id uint64) (*model.User, error)
}

// UserService owns the read user business rules. It knows nothing about HTTP:
// no gin, no JSON, no status codes. Errors are typed (httperr) so the handler
// layer can map them.
type UserService struct {
	users UserReader
}

func NewUserService(users UserReader) *UserService {
	return &UserService{users: users}
}

// GetUserByID returns the user or classifies the failure:
//   - repository.ErrNotFound          -> 404 (typed)
//   - any other repository error      -> 500 (typed, details kept for logging)
func (s *UserService) GetUserByID(ctx context.Context, id uint64) (*model.User, error) {
	u, err := s.users.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, httperr.NotFound("Resource not found.")
		}
		return nil, httperr.Internal(err)
	}
	return u, nil
}
