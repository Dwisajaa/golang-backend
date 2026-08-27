package repository

import (
	"context"

	"github.com/Dwisajaa/golang-backend/internal/model"
)

// ServiceCategoryStore is the persistence contract for service categories.
type ServiceCategoryStore interface {
	// CountActive returns the number of active categories that have at least
	// one active service (drives the paginator's meta.total).
	CountActive(ctx context.Context, q Queryer) (int, error)
	// ListActive returns active categories (with active services, ordered by
	// name) paginated. Category rows ordered by name — Laravel parity.
	ListActive(ctx context.Context, q Queryer, limit, offset int) ([]*model.ServiceCategory, error)
	// FindByID returns one category (admin operations) or ErrNotFound.
	FindByID(ctx context.Context, q Queryer, id uint64) (*model.ServiceCategory, error)
	// HasServices reports whether the category owns any service (delete guard).
	HasServices(ctx context.Context, q Queryer, id uint64) (bool, error)
	// NameTaken/SlugTaken drive unique validation (ignore self on update).
	NameTaken(ctx context.Context, q Queryer, name string, ignoreID uint64) (bool, error)
	SlugTaken(ctx context.Context, q Queryer, slug string, ignoreID uint64) (bool, error)
	// Create inserts; unique violations surface as ErrDuplicateName /
	// ErrDuplicateSlug.
	Create(ctx context.Context, q Queryer, c *model.ServiceCategory) error
	// Update applies the editable fields by id; unique violations surface as
	// ErrDuplicateName / ErrDuplicateSlug.
	Update(ctx context.Context, q Queryer, c *model.ServiceCategory) error
	// Delete removes the row (hard delete — Laravel parity).
	Delete(ctx context.Context, q Queryer, id uint64) error
}
