package repository

import (
	"context"

	"github.com/Dwisajaa/golang-backend/internal/model"
)

// PackageStore is the persistence contract for packages. Items are always
// managed alongside the package (store/update create/replace items atomically
// inside the same transaction).
type PackageStore interface {
	CountActive(ctx context.Context, q Queryer, search string) (int, error)
	ListActive(ctx context.Context, q Queryer, search string, limit, offset int) ([]*model.Package, error)
	FindActiveByID(ctx context.Context, q Queryer, id uint64) (*model.Package, error)
	FindByID(ctx context.Context, q Queryer, id uint64) (*model.Package, error)
	NameTaken(ctx context.Context, q Queryer, name string, ignoreID uint64) (bool, error)
	SlugTaken(ctx context.Context, q Queryer, slug string, ignoreID uint64) (bool, error)
	Create(ctx context.Context, q Queryer, p *model.Package) error
	InsertItems(ctx context.Context, q Queryer, packageID uint64, items []model.PackageItemInput) error
	DeleteItems(ctx context.Context, q Queryer, packageID uint64) error
	Update(ctx context.Context, q Queryer, p *model.Package) error
	Delete(ctx context.Context, q Queryer, id uint64) error
}
