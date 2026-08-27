package repository

import (
	"context"

	"github.com/Dwisajaa/golang-backend/internal/model"
)

// CustomerProfileStore is the persistence surface for customer profiles.
// Methods take Queryer so the service controls transaction membership.
type CustomerProfileStore interface {
	// FindByUserID returns the profile or ErrNotFound. The unique user_id
	// constraint guarantees at most one row.
	FindByUserID(ctx context.Context, q Queryer, userID uint64) (*model.CustomerProfile, error)
	// Upsert performs create-or-update (Laravel updateOrCreate). Relies on the
	// users.user_id unique constraint as the atomic switch — a concurrent
	// insert cannot duplicate the row.
	Upsert(ctx context.Context, q Queryer, p *model.CustomerProfile) error
}
