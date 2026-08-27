package service

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/Dwisajaa/golang-backend/internal/httperr"
	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/repository"
)

// catStore is the persistence slice the service category service uses.
type catStore interface {
	CountActive(ctx context.Context, q repository.Queryer) (int, error)
	ListActive(ctx context.Context, q repository.Queryer, limit, offset int) ([]*model.ServiceCategory, error)
	FindByID(ctx context.Context, q repository.Queryer, id uint64) (*model.ServiceCategory, error)
	HasServices(ctx context.Context, q repository.Queryer, id uint64) (bool, error)
	NameTaken(ctx context.Context, q repository.Queryer, name string, ignoreID uint64) (bool, error)
	SlugTaken(ctx context.Context, q repository.Queryer, slug string, ignoreID uint64) (bool, error)
	Create(ctx context.Context, q repository.Queryer, c *model.ServiceCategory) error
	Update(ctx context.Context, q repository.Queryer, c *model.ServiceCategory) error
	Delete(ctx context.Context, q repository.Queryer, id uint64) error
}

// ServiceCategoryService owns catalog-list and admin category CRUD rules.
type ServiceCategoryService struct {
	cats    catStore
	tx      txRunner
	perPage int
}

func NewServiceCategoryService(cats catStore, tx txRunner) *ServiceCategoryService {
	return &ServiceCategoryService{cats: cats, tx: tx, perPage: model.DefaultCategoryPerPage}
}

// CategoryInput carries the request fields (is_active pointer: nil = not
// provided, keeps the DB default on create / existing value on update).
type CategoryInput struct {
	Name        string
	Slug        string
	Description string
	Icon        string
	IsActive    *bool
}

// CategoryList is the paginated catalog result.
type CategoryList struct {
	Items   []*model.ServiceCategory
	Total   int
	Page    int
	PerPage int
}

// ListCategories mirrors CatalogController@categories: active categories with
// at least one active service, active services nested (ordered by name),
// ordered by name, paginated (fixed per_page default).
func (s *ServiceCategoryService) ListCategories(ctx context.Context, page int) (*CategoryList, error) {
	if page < 1 {
		page = 1
	}
	limit := s.perPage
	offset := (page - 1) * limit

	var out CategoryList
	out.Page = page
	out.PerPage = limit
	err := s.tx.Within(ctx, func(tx *sql.Tx) error {
		total, err := s.cats.CountActive(ctx, tx)
		if err != nil {
			return err
		}
		out.Total = total
		items, err := s.cats.ListActive(ctx, tx, limit, offset)
		if err != nil {
			return err
		}
		out.Items = items
		return nil
	})
	if err != nil {
		return nil, httperr.Internal(err)
	}
	return &out, nil
}

// Create mirrors Admin CategoryController@store. Unique pre-checks and the
// insert share one transaction so the DB constraint remains the final backstop.
func (s *ServiceCategoryService) Create(ctx context.Context, in CategoryInput) (*model.ServiceCategory, error) {
	slug := strings.TrimSpace(in.Slug)
	if slug == "" {
		slug = slugify(in.Name)
	}
	now := time.Now().UTC()
	cat := &model.ServiceCategory{
		Name: in.Name, Slug: slug,
		Description: nullIfEmpty(in.Description),
		Icon:        nullIfEmpty(in.Icon),
		IsActive:    true,
		CreatedAt:   &now, UpdatedAt: &now,
	}
	if in.IsActive != nil {
		cat.IsActive = *in.IsActive
	}

	err := s.tx.Within(ctx, func(tx *sql.Tx) error {
		if taken, err := s.cats.NameTaken(ctx, tx, in.Name, 0); err != nil {
			return err
		} else if taken {
			return httperr.Validation(map[string][]string{"name": {"The name has already been taken."}})
		}
		if taken, err := s.cats.SlugTaken(ctx, tx, cat.Slug, 0); err != nil {
			return err
		} else if taken {
			return httperr.Validation(map[string][]string{"slug": {"The slug has already been taken."}})
		}
		return s.cats.Create(ctx, tx, cat)
	})
	if err != nil {
		return nil, s.mapCatError(err)
	}
	return cat, nil
}

// Update mirrors Admin CategoryController@update: slug is always re-derived
// from name (Laravel prepareForValidation), unique checks ignore self.
func (s *ServiceCategoryService) Update(ctx context.Context, id uint64, in CategoryInput) (*model.ServiceCategory, error) {
	var out *model.ServiceCategory
	err := s.tx.Within(ctx, func(tx *sql.Tx) error {
		existing, err := s.cats.FindByID(ctx, tx, id)
		if err != nil {
			return err
		}
		slug := slugify(in.Name)
		if taken, err := s.cats.NameTaken(ctx, tx, in.Name, id); err != nil {
			return err
		} else if taken {
			return httperr.Validation(map[string][]string{"name": {"The name has already been taken."}})
		}
		if taken, err := s.cats.SlugTaken(ctx, tx, slug, id); err != nil {
			return err
		} else if taken {
			return httperr.Validation(map[string][]string{"slug": {"The slug has already been taken."}})
		}

		now := time.Now().UTC()
		cat := &model.ServiceCategory{
			ID: id, Name: in.Name, Slug: slug,
			Description: nullIfEmpty(in.Description),
			Icon:        nullIfEmpty(in.Icon),
			IsActive:    existing.IsActive,
			UpdatedAt:   &now,
		}
		if in.IsActive != nil {
			cat.IsActive = *in.IsActive
		}
		if err := s.cats.Update(ctx, tx, cat); err != nil {
			return err
		}
		out = cat
		return nil
	})
	if err != nil {
		return nil, s.mapCatError(err)
	}
	return out, nil
}

// Delete mirrors Admin CategoryController@destroy: refuses while the category
// has services (409), then hard-deletes.
func (s *ServiceCategoryService) Delete(ctx context.Context, id uint64) error {
	err := s.tx.Within(ctx, func(tx *sql.Tx) error {
		if _, err := s.cats.FindByID(ctx, tx, id); err != nil {
			return err
		}
		has, err := s.cats.HasServices(ctx, tx, id)
		if err != nil {
			return err
		}
		if has {
			return httperr.Conflict("Category cannot be deleted while it has services. Deactivate it instead.")
		}
		return s.cats.Delete(ctx, tx, id)
	})
	if err != nil {
		return s.mapCatError(err)
	}
	return nil
}

func (s *ServiceCategoryService) mapCatError(err error) error {
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
		return he // conflict/validation from inside the tx passes through
	}
	return httperr.Internal(err)
}

var slugClean = regexp.MustCompile(`[^a-z0-9]+`)

// slugify mirrors Laravel Str::slug: lowercase, non-alphanumerics → "-",
// trimmed.
func slugify(s string) string {
	return strings.Trim(slugClean.ReplaceAllString(strings.ToLower(s), "-"), "-")
}
