package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Dwisajaa/golang-backend/internal/httperr"
	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/repository"
)

type fakeCatStore struct {
	byID    map[uint64]*model.ServiceCategory
	nameIdx map[string]uint64
	slugIdx map[string]uint64
	total   int
	items   []*model.ServiceCategory
	hasSvcs map[uint64]bool
	err     error
}

func newFakeCat() *fakeCatStore {
	return &fakeCatStore{
		byID:    map[uint64]*model.ServiceCategory{},
		nameIdx: map[string]uint64{},
		slugIdx: map[string]uint64{},
		hasSvcs: map[uint64]bool{},
	}
}

func (f *fakeCatStore) CountActive(ctx context.Context, q repository.Queryer) (int, error) {
	return f.total, f.err
}
func (f *fakeCatStore) ListActive(ctx context.Context, q repository.Queryer, limit, offset int) ([]*model.ServiceCategory, error) {
	return f.items, f.err
}
func (f *fakeCatStore) FindByID(ctx context.Context, q repository.Queryer, id uint64) (*model.ServiceCategory, error) {
	if f.err != nil {
		return nil, f.err
	}
	if c, ok := f.byID[id]; ok {
		return c, nil
	}
	return nil, repository.ErrNotFound
}
func (f *fakeCatStore) HasServices(ctx context.Context, q repository.Queryer, id uint64) (bool, error) {
	return f.hasSvcs[id], f.err
}
func (f *fakeCatStore) NameTaken(ctx context.Context, q repository.Queryer, name string, ignoreID uint64) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	if id, ok := f.nameIdx[name]; ok && id != ignoreID {
		return true, nil
	}
	return false, nil
}
func (f *fakeCatStore) SlugTaken(ctx context.Context, q repository.Queryer, slug string, ignoreID uint64) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	if id, ok := f.slugIdx[slug]; ok && id != ignoreID {
		return true, nil
	}
	return false, nil
}
func (f *fakeCatStore) Create(ctx context.Context, q repository.Queryer, c *model.ServiceCategory) error {
	if f.err != nil {
		return f.err
	}
	c.ID = uint64(len(f.byID) + 1)
	f.byID[c.ID] = c
	f.nameIdx[c.Name] = c.ID
	f.slugIdx[c.Slug] = c.ID
	return nil
}
func (f *fakeCatStore) Update(ctx context.Context, q repository.Queryer, c *model.ServiceCategory) error {
	if f.err != nil {
		return f.err
	}
	delete(f.nameIdx, f.byID[c.ID].Name)
	delete(f.slugIdx, f.byID[c.ID].Slug)
	f.byID[c.ID] = c
	f.nameIdx[c.Name] = c.ID
	f.slugIdx[c.Slug] = c.ID
	return nil
}
func (f *fakeCatStore) Delete(ctx context.Context, q repository.Queryer, id uint64) error {
	if f.err != nil {
		return f.err
	}
	if _, ok := f.byID[id]; !ok {
		return repository.ErrNotFound
	}
	delete(f.byID, id)
	return nil
}

func TestCatListPaginated(t *testing.T) {
	fake := newFakeCat()
	fake.total = 2
	fake.items = []*model.ServiceCategory{
		{ID: 1, Name: "A", Slug: "a", Services: []*model.ServiceLite{{ID: 1, Name: "S", Unit: "per_service", PriceCents: 10000}}},
		{ID: 2, Name: "B", Slug: "b"},
	}
	svc := NewServiceCategoryService(fake, fakeTx{})

	list, err := svc.ListCategories(context.Background(), 2)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if list.Total != 2 || list.Page != 2 || list.PerPage != model.DefaultCategoryPerPage {
		t.Fatalf("pagination metadata wrong: %+v", list)
	}
	if len(list.Items) != 2 || list.Items[0].Services[0].PriceCents != 10000 {
		t.Fatalf("items wrong: %+v", list.Items)
	}
}

func TestCatCreateAutoSlugAndDefaults(t *testing.T) {
	fake := newFakeCat()
	svc := NewServiceCategoryService(fake, fakeTx{})

	c, err := svc.Create(context.Background(), CategoryInput{Name: "AC Service"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if c.Slug != "ac-service" {
		t.Fatalf("expected auto slug ac-service, got %q", c.Slug)
	}
	if !c.IsActive {
		t.Fatal("is_active must default true")
	}
}

func TestCatCreateDuplicateName(t *testing.T) {
	fake := newFakeCat()
	fake.nameIdx["X"] = 9
	svc := NewServiceCategoryService(fake, fakeTx{})

	_, err := svc.Create(context.Background(), CategoryInput{Name: "X"})
	he := httperr.As(err)
	if he == nil || he.Kind != httperr.KindValidation || len(he.Errors["name"]) == 0 {
		t.Fatalf("expected 422 name taken, got %v", err)
	}
}

func TestCatUpdateDerivesSlugAndIgnoresSelf(t *testing.T) {
	fake := newFakeCat()
	fake.byID[1] = &model.ServiceCategory{ID: 1, Name: "Old", Slug: "old"}
	fake.nameIdx["Old"] = 1
	fake.slugIdx["old"] = 1
	svc := NewServiceCategoryService(fake, fakeTx{})

	c, err := svc.Update(context.Background(), 1, CategoryInput{Name: "New Name"})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if c.Slug != "new-name" {
		t.Fatalf("slug must be derived from name on update, got %q", c.Slug)
	}
}

func TestCatUpdateNotFound(t *testing.T) {
	svc := NewServiceCategoryService(newFakeCat(), fakeTx{})
	_, err := svc.Update(context.Background(), 404, CategoryInput{Name: "X"})
	if he := httperr.As(err); he == nil || he.Kind != httperr.KindNotFound {
		t.Fatalf("expected 404, got %v", err)
	}
}

func TestCatDeleteGuardedByServices(t *testing.T) {
	fake := newFakeCat()
	fake.byID[1] = &model.ServiceCategory{ID: 1, Name: "A"}
	fake.hasSvcs[1] = true
	svc := NewServiceCategoryService(fake, fakeTx{})

	err := svc.Delete(context.Background(), 1)
	he := httperr.As(err)
	if he == nil || he.Kind != httperr.KindConflict || he.Message != "Category cannot be deleted while it has services. Deactivate it instead." {
		t.Fatalf("expected 409 Laravel message, got %v", err)
	}
}

func TestCatDeleteSuccess(t *testing.T) {
	fake := newFakeCat()
	fake.byID[1] = &model.ServiceCategory{ID: 1, Name: "A"}
	svc := NewServiceCategoryService(fake, fakeTx{})

	if err := svc.Delete(context.Background(), 1); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, ok := fake.byID[1]; ok {
		t.Fatal("category must be gone")
	}
}

func TestCatRepositoryErrorIsInternal(t *testing.T) {
	fake := newFakeCat()
	fake.err = errors.New("db down")
	svc := NewServiceCategoryService(fake, fakeTx{})

	if _, err := svc.ListCategories(context.Background(), 1); httperr.As(err) == nil || httperr.As(err).Kind != httperr.KindInternal {
		t.Fatalf("expected 500, got %v", err)
	}
	_, err := svc.Create(context.Background(), CategoryInput{Name: "A"})
	if httperr.As(err) == nil || httperr.As(err).Kind != httperr.KindInternal {
		t.Fatalf("expected 500, got %v", err)
	}
}
