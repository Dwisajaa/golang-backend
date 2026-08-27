package service

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/Dwisajaa/golang-backend/internal/httperr"
	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/repository"
)

// svcStore is the persistence slice the service service uses.
type svcStore interface {
	Count(ctx context.Context, q repository.Queryer, categoryID *uint64, search string) (int, error)
	List(ctx context.Context, q repository.Queryer, categoryID *uint64, search string, limit, offset int) ([]*model.Service, error)
	FindActiveByID(ctx context.Context, q repository.Queryer, id uint64) (*model.Service, error)
	FindByID(ctx context.Context, q repository.Queryer, id uint64) (*model.Service, error)
	CategoryExists(ctx context.Context, q repository.Queryer, id uint64) (bool, error)
	NameTaken(ctx context.Context, q repository.Queryer, name string, ignoreID uint64) (bool, error)
	SlugTaken(ctx context.Context, q repository.Queryer, slug string, ignoreID uint64) (bool, error)
	Create(ctx context.Context, q repository.Queryer, s *model.Service) error
	Update(ctx context.Context, q repository.Queryer, s *model.Service) error
	HasPackages(ctx context.Context, q repository.Queryer, id uint64) (bool, error)
	Delete(ctx context.Context, q repository.Queryer, id uint64) error
}

// ServiceService owns service catalog list/detail + admin CRUD rules.
type ServiceService struct {
	svc svcStore
	tx  txRunner
}

func NewServiceService(svc svcStore, tx txRunner) *ServiceService {
	return &ServiceService{svc: svc, tx: tx}
}

// ServiceList is the paginated public catalog result (per_page from request).
type ServiceList struct {
	Items   []*model.Service
	Total   int
	Page    int
	PerPage int
}

// ServiceInput mirrors Store/UpdateServiceRequest (price in integer cents).
type ServiceInput struct {
	ServiceCategoryID uint64
	Name              string
	Slug              string
	Description       string
	PriceCents        int64
	Unit              string
	EstimatedDuration *int64
	IsActive          *bool
}

// List mirrors CatalogController@services.
func (s *ServiceService) List(ctx context.Context, categoryID *uint64, search string, page, perPage int) (*ServiceList, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = model.DefaultServicePerPage
	}
	if perPage > 50 {
		perPage = 50
	}
	var out ServiceList
	out.Page, out.PerPage = page, perPage
	err := s.tx.Within(ctx, func(tx *sql.Tx) error {
		total, err := s.svc.Count(ctx, tx, categoryID, search)
		if err != nil {
			return err
		}
		items, err := s.svc.List(ctx, tx, categoryID, search, perPage, (page-1)*perPage)
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

// Get mirrors CatalogController@service: 404 when inactive or category inactive.
func (s *ServiceService) Get(ctx context.Context, id uint64) (*model.Service, error) {
	var out *model.Service
	err := s.tx.Within(ctx, func(tx *sql.Tx) error {
		got, err := s.svc.FindActiveByID(ctx, tx, id)
		if err != nil {
			return err
		}
		out = got
		return nil
	})
	if err != nil {
		return nil, mapSvcError(err)
	}
	return out, nil
}

// Create mirrors Admin ServiceController@store.
func (s *ServiceService) Create(ctx context.Context, in ServiceInput) (*model.Service, error) {
	now := time.Now().UTC()
	svc := &model.Service{
		ServiceCategoryID: in.ServiceCategoryID,
		Name:              in.Name,
		Slug:              orSlug(in.Slug, in.Name),
		Description:       nullIfEmpty(in.Description),
		PriceCents:        in.PriceCents,
		Unit:              in.Unit,
		EstimatedDuration: in.EstimatedDuration,
		IsActive:          true,
		CreatedAt:         &now,
		UpdatedAt:         &now,
	}
	if in.IsActive != nil {
		svc.IsActive = *in.IsActive
	}

	err := s.tx.Within(ctx, func(tx *sql.Tx) error {
		if err := checkCategory(ctx, tx, s.svc.CategoryExists, in.ServiceCategoryID); err != nil {
			return err
		}
		if taken, err := s.svc.NameTaken(ctx, tx, svc.Name, 0); err != nil {
			return err
		} else if taken {
			return httperr.Validation(map[string][]string{"name": {"The name has already been taken."}})
		}
		if taken, err := s.svc.SlugTaken(ctx, tx, svc.Slug, 0); err != nil {
			return err
		} else if taken {
			return httperr.Validation(map[string][]string{"slug": {"The slug has already been taken."}})
		}
		return s.svc.Create(ctx, tx, svc)
	})
	if err != nil {
		return nil, mapSvcError(err)
	}
	return svc, nil
}

// Update mirrors Admin ServiceController@update (slug always re-derived).
func (s *ServiceService) Update(ctx context.Context, id uint64, in ServiceInput) (*model.Service, error) {
	var out *model.Service
	err := s.tx.Within(ctx, func(tx *sql.Tx) error {
		existing, err := s.svc.FindByID(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := checkCategory(ctx, tx, s.svc.CategoryExists, in.ServiceCategoryID); err != nil {
			return err
		}
		slug := orSlug(in.Slug, in.Name)
		if taken, err := s.svc.NameTaken(ctx, tx, in.Name, id); err != nil {
			return err
		} else if taken {
			return httperr.Validation(map[string][]string{"name": {"The name has already been taken."}})
		}
		if taken, err := s.svc.SlugTaken(ctx, tx, slug, id); err != nil {
			return err
		} else if taken {
			return httperr.Validation(map[string][]string{"slug": {"The slug has already been taken."}})
		}
		isActive := existing.IsActive
		if in.IsActive != nil {
			isActive = *in.IsActive
		}
		now := time.Now().UTC()
		svc := &model.Service{
			ID: id, ServiceCategoryID: in.ServiceCategoryID,
			Name: in.Name, Slug: slug, Description: nullIfEmpty(in.Description),
			PriceCents: in.PriceCents, Unit: in.Unit,
			EstimatedDuration: in.EstimatedDuration,
			IsActive:          isActive,
			UpdatedAt:         &now,
		}
		if err := s.svc.Update(ctx, tx, svc); err != nil {
			return err
		}
		out = svc
		return nil
	})
	if err != nil {
		return nil, mapSvcError(err)
	}
	return out, nil
}

// Delete mirrors Admin ServiceController@destroy (package guard 409).
func (s *ServiceService) Delete(ctx context.Context, id uint64) error {
	err := s.tx.Within(ctx, func(tx *sql.Tx) error {
		if _, err := s.svc.FindByID(ctx, tx, id); err != nil {
			return err
		}
		used, err := s.svc.HasPackages(ctx, tx, id)
		if err != nil {
			return err
		}
		if used {
			return httperr.Conflict("Service cannot be deleted while it is used by a package. Deactivate it instead.")
		}
		return s.svc.Delete(ctx, tx, id)
	})
	if err != nil {
		return mapSvcError(err)
	}
	return nil
}

// poolQ and friend removed: all repository reads run through s.tx.Within so no
// direct Queryer is ever required by the service.

func mapSvcError(err error) error {
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

func checkCategory(ctx context.Context, tx *sql.Tx, existsFn func(context.Context, repository.Queryer, uint64) (bool, error), id uint64) error {
	ok, err := existsFn(ctx, tx, id)
	if err != nil {
		return err
	}
	if !ok {
		return httperr.Validation(map[string][]string{"service_category_id": {"The selected service category id is invalid."}})
	}
	return nil
}

func orSlug(slug, name string) string {
	if strings.TrimSpace(slug) != "" {
		return strings.TrimSpace(slug)
	}
	return slugify(name)
}
