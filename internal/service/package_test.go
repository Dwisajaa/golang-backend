package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Dwisajaa/golang-backend/internal/httperr"
	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/repository"
)

type fakePkgStore struct {
	byID    map[uint64]*model.Package
	nameIdx map[string]uint64
	slugIdx map[string]uint64
	total   int
	items   []*model.Package
	err     error
}

func newFakePkg() *fakePkgStore {
	return &fakePkgStore{byID: map[uint64]*model.Package{}, nameIdx: map[string]uint64{}, slugIdx: map[string]uint64{}}
}

func (f *fakePkgStore) CountActive(ctx context.Context, q repository.Queryer, search string) (int, error) {
	return f.total, f.err
}
func (f *fakePkgStore) ListActive(ctx context.Context, q repository.Queryer, search string, limit, offset int) ([]*model.Package, error) {
	return f.items, f.err
}
func (f *fakePkgStore) FindActiveByID(ctx context.Context, q repository.Queryer, id uint64) (*model.Package, error) {
	if f.err != nil {
		return nil, f.err
	}
	p, ok := f.byID[id]
	if !ok || !p.IsActive {
		return nil, repository.ErrNotFound
	}
	return p, nil
}
func (f *fakePkgStore) FindByID(ctx context.Context, q repository.Queryer, id uint64) (*model.Package, error) {
	if f.err != nil {
		return nil, f.err
	}
	p, ok := f.byID[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return p, nil
}
func (f *fakePkgStore) NameTaken(ctx context.Context, q repository.Queryer, name string, ignoreID uint64) (bool, error) {
	if id, ok := f.nameIdx[name]; ok && id != ignoreID {
		return true, nil
	}
	return false, f.err
}
func (f *fakePkgStore) SlugTaken(ctx context.Context, q repository.Queryer, slug string, ignoreID uint64) (bool, error) {
	if id, ok := f.slugIdx[slug]; ok && id != ignoreID {
		return true, nil
	}
	return false, f.err
}
func (f *fakePkgStore) Create(ctx context.Context, q repository.Queryer, p *model.Package) error {
	if f.err != nil {
		return f.err
	}
	p.ID = uint64(len(f.byID) + 1)
	f.byID[p.ID] = p
	f.nameIdx[p.Name] = p.ID
	f.slugIdx[p.Slug] = p.ID
	return nil
}
func (f *fakePkgStore) InsertItems(ctx context.Context, q repository.Queryer, packageID uint64, items []model.PackageItemInput) error {
	return f.err
}
func (f *fakePkgStore) DeleteItems(ctx context.Context, q repository.Queryer, packageID uint64) error {
	return f.err
}
func (f *fakePkgStore) Update(ctx context.Context, q repository.Queryer, p *model.Package) error {
	if f.err != nil {
		return f.err
	}
	f.byID[p.ID] = p
	return nil
}
func (f *fakePkgStore) Delete(ctx context.Context, q repository.Queryer, id uint64) error {
	if f.err != nil {
		return f.err
	}
	if _, ok := f.byID[id]; !ok {
		return repository.ErrNotFound
	}
	delete(f.byID, id)
	return nil
}

type fakeSvcChk struct {
	all bool
	err error
}

func (f *fakeSvcChk) ServiceIDsExist(ctx context.Context, q repository.Queryer, ids []uint64) (bool, error) {
	return f.all, f.err
}

func TestPkgListAndDetail(t *testing.T) {
	fake := newFakePkg()
	fake.total = 1
	fake.items = []*model.Package{{ID: 1, Name: "A", Slug: "a", PriceCents: 50000, IsActive: true, IsPopular: true}}
	svc := NewPackageService(fake, &fakeSvcChk{all: true}, fakeTx{})

	list, err := svc.List(context.Background(), "", 1, 15)
	if err != nil || list.Total != 1 {
		t.Fatalf("list: %v %+v", err, list)
	}

	fake.byID[1] = fake.items[0]
	if _, err := svc.Get(context.Background(), 1); err != nil {
		t.Fatalf("get: %v", err)
	}
	if _, err := svc.Get(context.Background(), 99); httperr.As(err) == nil || httperr.As(err).Kind != httperr.KindNotFound {
		t.Fatalf("expected 404, got %v", err)
	}
}

func TestPkgCreateSlugAndItems(t *testing.T) {
	fake := newFakePkg()
	svc := NewPackageService(fake, &fakeSvcChk{all: true}, fakeTx{})

	p, err := svc.Create(context.Background(), PackageInput{
		Name: "Premium AC", PriceCents: 30000,
		Items: []model.PackageItemInput{{ServiceID: 1, Quantity: 2}},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if p.Slug != "premium-ac" || !p.IsActive || p.IsPopular {
		t.Fatalf("create wrong: %+v", p)
	}
}

func TestPkgCreateDupName(t *testing.T) {
	fake := newFakePkg()
	fake.nameIdx["X"] = 9
	svc := NewPackageService(fake, &fakeSvcChk{all: true}, fakeTx{})
	_, err := svc.Create(context.Background(), PackageInput{Name: "X", PriceCents: 1, Items: []model.PackageItemInput{{ServiceID: 1, Quantity: 1}}})
	if he := httperr.As(err); he == nil || he.Kind != httperr.KindValidation {
		t.Fatalf("expected 422, got %v", err)
	}
}

func TestPkgCreateBadService(t *testing.T) {
	svc := NewPackageService(newFakePkg(), &fakeSvcChk{all: false}, fakeTx{})
	_, err := svc.Create(context.Background(), PackageInput{Name: "A", PriceCents: 1, Items: []model.PackageItemInput{{ServiceID: 99, Quantity: 1}}})
	if he := httperr.As(err); he == nil || he.Kind != httperr.KindValidation {
		t.Fatalf("expected 422 service invalid, got %v", err)
	}
}

func TestPkgUpdateReplacesItems(t *testing.T) {
	fake := newFakePkg()
	fake.byID[1] = &model.Package{ID: 1, Name: "Old", Slug: "old", IsActive: true, PriceCents: 100}
	fake.nameIdx["Old"] = 1
	fake.slugIdx["old"] = 1
	svc := NewPackageService(fake, &fakeSvcChk{all: true}, fakeTx{})

	p, err := svc.Update(context.Background(), 1, PackageInput{Name: "New", PriceCents: 200, Items: []model.PackageItemInput{{ServiceID: 1, Quantity: 3}}})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if p.Slug != "new" {
		t.Fatalf("slug derive: %q", p.Slug)
	}
}

func TestPkgDeleteHardNoGuard(t *testing.T) {
	fake := newFakePkg()
	fake.byID[1] = &model.Package{ID: 1, Name: "A"}
	svc := NewPackageService(fake, &fakeSvcChk{all: true}, fakeTx{})

	if err := svc.Delete(context.Background(), 1); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := svc.Delete(context.Background(), 99); httperr.As(err) == nil || httperr.As(err).Kind != httperr.KindNotFound {
		t.Fatalf("expected 404, got %v", err)
	}
}

func TestPkgRepoError(t *testing.T) {
	fake := newFakePkg()
	fake.err = errors.New("db")
	svc := NewPackageService(fake, &fakeSvcChk{all: true}, fakeTx{})
	if _, err := svc.List(context.Background(), "", 1, 15); httperr.As(err) == nil || httperr.As(err).Kind != httperr.KindInternal {
		t.Fatalf("expected 500: %v", err)
	}
}
