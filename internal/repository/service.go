package repository

import (
	"context"

	"github.com/Dwisajaa/golang-backend/internal/model"
)

// ServiceStore is the persistence contract for services.
type ServiceStore interface {
	// Count returns active services (whose category is active) matching the
	// optional category filter + search; drives pagination meta.
	Count(ctx context.Context, q Queryer, categoryID *uint64, search string) (int, error)
	// List returns active services (category active) with their category
	// loaded (batch, no N+1), ordered by name, paginated.
	List(ctx context.Context, q Queryer, categoryID *uint64, search string, limit, offset int) ([]*model.Service, error)
	// FindActiveByID returns the service only when active AND its category is
	// active (else NotFound), with category loaded — public detail parity.
	FindActiveByID(ctx context.Context, q Queryer, id uint64) (*model.Service, error)
	// FindByID returns any service row (admin update/delete path).
	FindByID(ctx context.Context, q Queryer, id uint64) (*model.Service, error)
	CategoryExists(ctx context.Context, q Queryer, id uint64) (bool, error)
	NameTaken(ctx context.Context, q Queryer, name string, ignoreID uint64) (bool, error)
	SlugTaken(ctx context.Context, q Queryer, slug string, ignoreID uint64) (bool, error)
	// Create/Update translate 1062 to ErrDuplicateName / ErrDuplicateSlug.
	Create(ctx context.Context, q Queryer, s *model.Service) error
	Update(ctx context.Context, q Queryer, s *model.Service) error
	// HasPackages reports usage by any package (package_items) — delete guard.
	HasPackages(ctx context.Context, q Queryer, id uint64) (bool, error)
	Delete(ctx context.Context, q Queryer, id uint64) error
}
