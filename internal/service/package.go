package service

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Dwisajaa/golang-backend/internal/httperr"
	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/repository"
)

type pkgStore interface {
	CountActive(ctx context.Context, q repository.Queryer, search string) (int, error)
	ListActive(ctx context.Context, q repository.Queryer, search string, limit, offset int) ([]*model.Package, error)
	FindActiveByID(ctx context.Context, q repository.Queryer, id uint64) (*model.Package, error)
	FindByID(ctx context.Context, q repository.Queryer, id uint64) (*model.Package, error)
	NameTaken(ctx context.Context, q repository.Queryer, name string, ignoreID uint64) (bool, error)
	SlugTaken(ctx context.Context, q repository.Queryer, slug string, ignoreID uint64) (bool, error)
	Create(ctx context.Context, q repository.Queryer, p *model.Package) error
	InsertItems(ctx context.Context, q repository.Queryer, packageID uint64, items []model.PackageItemInput) error
	DeleteItems(ctx context.Context, q repository.Queryer, packageID uint64) error
	Update(ctx context.Context, q repository.Queryer, p *model.Package) error
	Delete(ctx context.Context, q repository.Queryer, id uint64) error
}

// pkgServiceChecker validates item service_ids exist (via the existing
// ServiceStore.CategoryExists pattern — here we just check count).
type pkgServiceChecker interface {
	ServiceIDsExist(ctx context.Context, q repository.Queryer, ids []uint64) (bool, error)
}

// PackageService owns package catalog + admin CRUD rules.
type PackageService struct {
	pkgs   pkgStore
	svcChk pkgServiceChecker
	tx     txRunner
}

func NewPackageService(pkgs pkgStore, svcChk pkgServiceChecker, tx txRunner) *PackageService {
	return &PackageService{pkgs: pkgs, svcChk: svcChk, tx: tx}
}

type PackageList struct {
	Items   []*model.Package
	Total   int
	Page    int
	PerPage int
}

type PackageInput struct {
	Name        string
	Slug        string
	Description string
	PriceCents  int64
	Duration    *int64
	IsActive    *bool
	IsPopular   *bool
	Items       []model.PackageItemInput
}

// List mirrors CatalogController@packages.
func (s *PackageService) List(ctx context.Context, search string, page, perPage int) (*PackageList, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = model.DefaultPackagePerPage
	}
	if perPage > 50 {
		perPage = 50
	}
	var out PackageList
	out.Page, out.PerPage = page, perPage
	err := s.tx.Within(ctx, func(tx *sql.Tx) error {
		total, err := s.pkgs.CountActive(ctx, tx, search)
		if err != nil {
			return err
		}
		items, err := s.pkgs.ListActive(ctx, tx, search, perPage, (page-1)*perPage)
		if err != nil {
			return err
		}
		out.Total, out.Items = total, items
		return nil
	})
	if err != nil {
		return nil, httperr.Internal(err)
	}
	return &out, nil
}

// Get mirrors CatalogController@package: 404 if inactive.
func (s *PackageService) Get(ctx context.Context, id uint64) (*model.Package, error) {
	var out *model.Package
	err := s.tx.Within(ctx, func(tx *sql.Tx) error {
		p, err := s.pkgs.FindActiveByID(ctx, tx, id)
		if err != nil {
			return err
		}
		out = p
		return nil
	})
	if err != nil {
		return nil, mapPkgError(err)
	}
	return out, nil
}

// Create mirrors Admin PackageController@store.
func (s *PackageService) Create(ctx context.Context, in PackageInput) (*model.Package, error) {
	now := time.Now().UTC()
	pkg := &model.Package{
		Name: in.Name, Slug: orSlug(in.Slug, in.Name),
		Description: nullIfEmpty(in.Description), PriceCents: in.PriceCents,
		Duration: in.Duration, IsActive: true, IsPopular: false,
		CreatedAt: &now, UpdatedAt: &now,
	}
	if in.IsActive != nil {
		pkg.IsActive = *in.IsActive
	}
	if in.IsPopular != nil {
		pkg.IsPopular = *in.IsPopular
	}

	err := s.tx.Within(ctx, func(tx *sql.Tx) error {
		if taken, err := s.pkgs.NameTaken(ctx, tx, pkg.Name, 0); err != nil {
			return err
		} else if taken {
			return httperr.Validation(map[string][]string{"name": {"The name has already been taken."}})
		}
		if taken, err := s.pkgs.SlugTaken(ctx, tx, pkg.Slug, 0); err != nil {
			return err
		} else if taken {
			return httperr.Validation(map[string][]string{"slug": {"The slug has already been taken."}})
		}
		if ok, err := s.svcChk.ServiceIDsExist(ctx, tx, itemServiceIDs(in.Items)); err != nil {
			return err
		} else if !ok {
			return httperr.Validation(map[string][]string{"items": {"One or more selected service IDs are invalid."}})
		}
		if err := s.pkgs.Create(ctx, tx, pkg); err != nil {
			return err
		}
		return s.pkgs.InsertItems(ctx, tx, pkg.ID, in.Items)
	})
	if err != nil {
		return nil, mapPkgError(err)
	}

	// reload with items+service for response
	var full *model.Package
	_ = s.tx.Within(ctx, func(tx *sql.Tx) error {
		full, _ = s.pkgs.FindByID(ctx, tx, pkg.ID)
		return nil
	})
	if full != nil {
		return full, nil
	}
	return pkg, nil
}

// Update mirrors Admin PackageController@update (items fully replaced).
func (s *PackageService) Update(ctx context.Context, id uint64, in PackageInput) (*model.Package, error) {
	var out *model.Package
	err := s.tx.Within(ctx, func(tx *sql.Tx) error {
		existing, err := s.pkgs.FindByID(ctx, tx, id)
		if err != nil {
			return err
		}

		slug := orSlug(in.Slug, in.Name)
		if taken, err := s.pkgs.NameTaken(ctx, tx, in.Name, id); err != nil {
			return err
		} else if taken {
			return httperr.Validation(map[string][]string{"name": {"The name has already been taken."}})
		}
		if taken, err := s.pkgs.SlugTaken(ctx, tx, slug, id); err != nil {
			return err
		} else if taken {
			return httperr.Validation(map[string][]string{"slug": {"The slug has already been taken."}})
		}
		if ok, err := s.svcChk.ServiceIDsExist(ctx, tx, itemServiceIDs(in.Items)); err != nil {
			return err
		} else if !ok {
			return httperr.Validation(map[string][]string{"items": {"One or more selected service IDs are invalid."}})
		}

		isActive, isPopular := existing.IsActive, existing.IsPopular
		if in.IsActive != nil {
			isActive = *in.IsActive
		}
		if in.IsPopular != nil {
			isPopular = *in.IsPopular
		}

		now := time.Now().UTC()
		pkg := &model.Package{
			ID: id, Name: in.Name, Slug: slug,
			Description: nullIfEmpty(in.Description), PriceCents: in.PriceCents,
			Duration: in.Duration, IsActive: isActive, IsPopular: isPopular,
			UpdatedAt: &now,
		}
		if err := s.pkgs.Update(ctx, tx, pkg); err != nil {
			return err
		}
		if err := s.pkgs.DeleteItems(ctx, tx, id); err != nil {
			return err
		}
		if err := s.pkgs.InsertItems(ctx, tx, id, in.Items); err != nil {
			return err
		}

		// re-read inside same tx for fresh response including items+services
		out, err = s.pkgs.FindByID(ctx, tx, id)
		return err
	})
	if err != nil {
		return nil, mapPkgError(err)
	}
	return out, nil
}

// Delete mirrors Admin PackageController@destroy — hard delete, no guard
// (FK CASCADE removes package_items automatically).
func (s *PackageService) Delete(ctx context.Context, id uint64) error {
	err := s.tx.Within(ctx, func(tx *sql.Tx) error {
		if _, err := s.pkgs.FindByID(ctx, tx, id); err != nil {
			return err
		}
		return s.pkgs.Delete(ctx, tx, id)
	})
	return mapPkgError(err)
}

func mapPkgError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, repository.ErrNotFound) {
		return httperr.NotFound("Resource not found.")
	}
	if errors.Is(err, repository.ErrDuplicateName) {
		return httperr.Validation(map[string][]string{"name": {"The name has already been taken."}})
	}
	if errors.Is(err, repository.ErrDuplicateSlug) {
		return httperr.Validation(map[string][]string{"slug": {"The slug has already been taken."}})
	}
	if he := httperr.As(err); he != nil {
		return he
	}
	return httperr.Internal(err)
}

func itemServiceIDs(items []model.PackageItemInput) []uint64 {
	out := make([]uint64, len(items))
	for i, it := range items {
		out[i] = it.ServiceID
	}
	return out
}
