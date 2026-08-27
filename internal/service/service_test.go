package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Dwisajaa/golang-backend/internal/httperr"
	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/repository"
)

type fakeSvcStore struct {
	byID       map[uint64]*model.Service
	nameIdx    map[string]uint64
	slugIdx    map[string]uint64
	categories map[uint64]bool
	inPackages map[uint64]bool
	items      []*model.Service
	total      int
	detailErr  error
	err        error
}

func newFakeSvc() *fakeSvcStore {
	return &fakeSvcStore{
		byID: map[uint64]*model.Service{}, nameIdx: map[string]uint64{},
		slugIdx: map[string]uint64{}, categories: map[uint64]bool{1: true},
		inPackages: map[uint64]bool{},
	}
}

func (f *fakeSvcStore) Count(ctx context.Context, q repository.Queryer, categoryID *uint64, search string) (int, error) {
	return f.total, f.err
}
func (f *fakeSvcStore) List(ctx context.Context, q repository.Queryer, categoryID *uint64, search string, limit, offset int) ([]*model.Service, error) {
	return f.items, f.err
}
func (f *fakeSvcStore) FindActiveByID(ctx context.Context, q repository.Queryer, id uint64) (*model.Service, error) {
	if f.detailErr != nil {
		return nil, f.detailErr
	}
	s, ok := f.byID[id]
	if !ok || !s.IsActive {
		return nil, repository.ErrNotFound
	}
	return s, nil
}
func (f *fakeSvcStore) FindByID(ctx context.Context, q repository.Queryer, id uint64) (*model.Service, error) {
	if f.err != nil {
		return nil, f.err
	}
	if s, ok := f.byID[id]; ok {
		return s, nil
	}
	return nil, repository.ErrNotFound
}
func (f *fakeSvcStore) CategoryExists(ctx context.Context, q repository.Queryer, id uint64) (bool, error) {
	return f.categories[id], f.err
}
func (f *fakeSvcStore) NameTaken(ctx context.Context, q repository.Queryer, name string, ignoreID uint64) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	if id, ok := f.nameIdx[name]; ok && id != ignoreID {
		return true, nil
	}
	return false, nil
}
func (f *fakeSvcStore) SlugTaken(ctx context.Context, q repository.Queryer, slug string, ignoreID uint64) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	if id, ok := f.slugIdx[slug]; ok && id != ignoreID {
		return true, nil
	}
	return false, nil
}
func (f *fakeSvcStore) Create(ctx context.Context, q repository.Queryer, s *model.Service) error {
	if f.err != nil {
		return f.err
	}
	s.ID = uint64(len(f.byID) + 1)
	f.byID[s.ID] = s
	f.nameIdx[s.Name] = s.ID
	f.slugIdx[s.Slug] = s.ID
	return nil
}
func (f *fakeSvcStore) Update(ctx context.Context, q repository.Queryer, s *model.Service) error {
	if f.err != nil {
		return f.err
	}
	delete(f.nameIdx, f.byID[s.ID].Name)
	delete(f.slugIdx, f.byID[s.ID].Slug)
	f.byID[s.ID] = s
	f.nameIdx[s.Name] = s.ID
	f.slugIdx[s.Slug] = s.ID
	return nil
}
func (f *fakeSvcStore) HasPackages(ctx context.Context, q repository.Queryer, id uint64) (bool, error) {
	return f.inPackages[id], f.err
}
func (f *fakeSvcStore) Delete(ctx context.Context, q repository.Queryer, id uint64) error {
	if f.err != nil {
		return f.err
	}
	if _, ok := f.byID[id]; !ok {
		return repository.ErrNotFound
	}
	delete(f.byID, id)
	return nil
}

func TestSvcListFiltersAndPagination(t *testing.T) {
	fake := newFakeSvc()
	fake.total = 2
	fake.items = []*model.Service{
		{ID: 1, Name: "A", Slug: "a", PriceCents: 9999, Unit: "per_unit", Category: &model.ServiceCategory{ID: 1, Name: "Cat"}},
		{ID: 2, Name: "B", Slug: "b", PriceCents: 1, Unit: "per_service"},
	}
	svc := NewServiceService(fake, fakeTx{})

	list, err := svc.List(context.Background(), nil, "", 2, 30)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if list.Total != 2 || list.Page != 2 || list.PerPage != 30 {
		t.Fatalf("meta wrong: %+v", list)
	}
}

func TestSvcGetActiveOnly(t *testing.T) {
	fake := newFakeSvc()
	fake.byID[1] = &model.Service{ID: 1, Name: "A", IsActive: true}
	fake.byID[2] = &model.Service{ID: 2, Name: "B", IsActive: false}
	svc := NewServiceService(fake, fakeTx{})

	if _, err := svc.Get(context.Background(), 1); err != nil {
		t.Fatalf("get active: %v", err)
	}
	if _, err := svc.Get(context.Background(), 2); httperr.As(err) == nil || httperr.As(err).Kind != httperr.KindNotFound {
		t.Fatalf("inactive must be 404, got %v", err)
	}
}

func TestSvcCreateAutoSlugAndCategoryValidation(t *testing.T) {
	fake := newFakeSvc()
	svc := NewServiceService(fake, fakeTx{})

	// invalid category
	_, err := svc.Create(context.Background(), ServiceInput{ServiceCategoryID: 99, Name: "A", Unit: "per_service", PriceCents: 100})
	he := httperr.As(err)
	if he == nil || he.Kind != httperr.KindValidation || len(he.Errors["service_category_id"]) == 0 {
		t.Fatalf("expected 422 invalid category, got %v", err)
	}

	s, err := svc.Create(context.Background(), ServiceInput{ServiceCategoryID: 1, Name: "AC Repair", Unit: "per_service", PriceCents: 15000})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if s.Slug != "ac-repair" || !s.IsActive || s.PriceCents != 15000 {
		t.Fatalf("create wrong: %+v", s)
	}
}

func TestSvcCreateDuplicateName(t *testing.T) {
	fake := newFakeSvc()
	fake.nameIdx["X"] = 9
	svc := NewServiceService(fake, fakeTx{})

	_, err := svc.Create(context.Background(), ServiceInput{ServiceCategoryID: 1, Name: "X", Unit: "per_service", PriceCents: 100})
	he := httperr.As(err)
	if he == nil || he.Kind != httperr.KindValidation || len(he.Errors["name"]) == 0 {
		t.Fatalf("expected 422 name taken, got %v", err)
	}
}

func TestSvcUpdateDerivesSlugAndKeepsActive(t *testing.T) {
	fake := newFakeSvc()
	fake.byID[1] = &model.Service{ID: 1, ServiceCategoryID: 1, Name: "Old", Slug: "old", Unit: "per_service", IsActive: true}
	fake.nameIdx["Old"] = 1
	fake.slugIdx["old"] = 1
	svc := NewServiceService(fake, fakeTx{})

	s, err := svc.Update(context.Background(), 1, ServiceInput{ServiceCategoryID: 1, Name: "New Name", Unit: "per_unit", PriceCents: 1})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if s.Slug != "new-name" || !s.IsActive {
		t.Fatalf("update wrong: %+v", s)
	}
}

func TestSvcDeleteGuardedByPackages(t *testing.T) {
	fake := newFakeSvc()
	fake.byID[1] = &model.Service{ID: 1, Name: "A"}
	fake.inPackages[1] = true
	svc := NewServiceService(fake, fakeTx{})

	err := svc.Delete(context.Background(), 1)
	he := httperr.As(err)
	if he == nil || he.Kind != httperr.KindConflict || he.Message != "Service cannot be deleted while it is used by a package. Deactivate it instead." {
		t.Fatalf("expected 409 Laravel message, got %v", err)
	}
}

func TestSvcDeleteSuccessAnd404(t *testing.T) {
	fake := newFakeSvc()
	fake.byID[1] = &model.Service{ID: 1, Name: "A"}
	svc := NewServiceService(fake, fakeTx{})

	if err := svc.Delete(context.Background(), 1); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := svc.Delete(context.Background(), 2); httperr.As(err) == nil || httperr.As(err).Kind != httperr.KindNotFound {
		t.Fatalf("expected 404, got %v", err)
	}
}

func TestSvcRepositoryErrorInternal(t *testing.T) {
	fake := newFakeSvc()
	fake.err = errors.New("db down")
	svc := NewServiceService(fake, fakeTx{})

	_, err := svc.Create(context.Background(), ServiceInput{ServiceCategoryID: 1, Name: "A", Unit: "per_service", PriceCents: 1})
	if he := httperr.As(err); he == nil || he.Kind != httperr.KindInternal {
		t.Fatalf("expected 500, got %v", err)
	}
	_, err = svc.List(context.Background(), nil, "", 1, 15)
	if he := httperr.As(err); he == nil || he.Kind != httperr.KindInternal {
		t.Fatalf("expected 500, got %v", err)
	}
}
