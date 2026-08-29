package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Dwisajaa/golang-backend/internal/httperr"
	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/repository"
)

type fakeBookingStore struct {
	byID      map[uint64]*model.Booking
	created   []*model.Booking
	items     []*model.BookingItem
	invoices  []*model.Invoice
	total     int
	listItems []*model.Booking
	err       error
}

func newFakeBooking() *fakeBookingStore {
	return &fakeBookingStore{byID: map[uint64]*model.Booking{}}
}

func (f *fakeBookingStore) CountByCustomer(ctx context.Context, q repository.Queryer, cid uint64) (int, error) {
	return f.total, f.err
}
func (f *fakeBookingStore) ListByCustomer(ctx context.Context, q repository.Queryer, cid uint64, l, o int) ([]*model.Booking, error) {
	return f.listItems, f.err
}
func (f *fakeBookingStore) FindByID(ctx context.Context, q repository.Queryer, id uint64) (*model.Booking, error) {
	if f.err != nil {
		return nil, f.err
	}
	b, ok := f.byID[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return b, nil
}
func (f *fakeBookingStore) FindByIDForUpdate(ctx context.Context, q repository.Queryer, id uint64) (*model.Booking, error) {
	return f.FindByID(ctx, q, id)
}
func (f *fakeBookingStore) Create(ctx context.Context, q repository.Queryer, b *model.Booking) error {
	if f.err != nil {
		return f.err
	}
	b.ID = uint64(len(f.created) + 1)
	f.byID[b.ID] = b
	f.created = append(f.created, b)
	return nil
}
func (f *fakeBookingStore) CreateItem(ctx context.Context, q repository.Queryer, it *model.BookingItem) error {
	if f.err != nil {
		return f.err
	}
	it.ID = uint64(len(f.items) + 1)
	f.items = append(f.items, it)
	return nil
}
func (f *fakeBookingStore) CreateInvoice(ctx context.Context, q repository.Queryer, inv *model.Invoice) error {
	if f.err != nil {
		return f.err
	}
	inv.ID = uint64(len(f.invoices) + 1)
	f.invoices = append(f.invoices, inv)
	return nil
}
func (f *fakeBookingStore) UpdateStatus(ctx context.Context, q repository.Queryer, id uint64, s string) error {
	if f.err != nil {
		return f.err
	}
	if b, ok := f.byID[id]; ok {
		b.Status = s
	}
	return nil
}
func (f *fakeBookingStore) UpdateInvoiceStatus(ctx context.Context, q repository.Queryer, bid uint64, s string) error {
	return f.err
}
func (f *fakeBookingStore) AdminCount(ctx context.Context, q repository.Queryer, fi repository.AdminBookingFilters) (int, error) {
	return f.total, f.err
}
func (f *fakeBookingStore) AdminList(ctx context.Context, q repository.Queryer, fi repository.AdminBookingFilters, l, o int) ([]*model.Booking, error) {
	return f.listItems, f.err
}
func (f *fakeBookingStore) AttachItems(ctx context.Context, q repository.Queryer, bs []*model.Booking) error {
	return nil
}
func (f *fakeBookingStore) AttachInvoices(ctx context.Context, q repository.Queryer, bs []*model.Booking) error {
	return nil
}
func (f *fakeBookingStore) BookingCodeExists(ctx context.Context, q repository.Queryer, c string) (bool, error) {
	return false, nil
}
func (f *fakeBookingStore) InvoiceNumberExists(ctx context.Context, q repository.Queryer, n string) (bool, error) {
	return false, nil
}

type fakeCatalog struct{ err error }

func (f *fakeCatalog) FindActiveServiceForUpdate(ctx context.Context, q repository.Queryer, id uint64) (string, int64, error) {
	if f.err != nil {
		return "", 0, f.err
	}
	return "AC Service", 15000, nil
}
func (f *fakeCatalog) FindActivePackageForUpdate(ctx context.Context, q repository.Queryer, id uint64) (string, int64, error) {
	if f.err != nil {
		return "", 0, f.err
	}
	return "Premium AC", 30000, nil
}

type fakeProfile struct{ complete bool }

func (f *fakeProfile) IsProfileComplete(ctx context.Context, uid uint64) (bool, error) {
	return f.complete, nil
}

func newBookingSvc() (*BookingService, *fakeBookingStore) {
	bs := newFakeBooking()
	return NewBookingService(bs, &fakeCatalog{}, &fakeProfile{complete: true}, fakeTx{}), bs
}

func validInput() CreateBookingInput {
	return CreateBookingInput{
		ItemType: "service", ServiceID: 1, Quantity: 2,
		BookingDate: "2026-12-01", BookingTime: "09:00", Address: "Jl Test",
	}
}

func TestBookingCreateSuccess(t *testing.T) {
	svc, bs := newBookingSvc()
	b, err := svc.Create(context.Background(), 7, validInput())
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if b.Status != model.BookingStatusPendingPayment {
		t.Fatalf("status: %s", b.Status)
	}
	if b.SubtotalCents != 30000 {
		t.Fatalf("subtotal: %d (expected 15000*2=30000)", b.SubtotalCents)
	}
	if len(bs.items) != 1 {
		t.Fatalf("items: %d", len(bs.items))
	}
	if bs.items[0].ItemName != "AC Service" {
		t.Fatalf("snapshot name: %s", bs.items[0].ItemName)
	}
	if len(bs.invoices) != 1 {
		t.Fatalf("invoices: %d", len(bs.invoices))
	}
}

func TestBookingCreateIncompleteProfile(t *testing.T) {
	bs := newFakeBooking()
	svc := NewBookingService(bs, &fakeCatalog{}, &fakeProfile{complete: false}, fakeTx{})
	_, err := svc.Create(context.Background(), 7, validInput())
	he := httperr.As(err)
	if he == nil || he.Kind != httperr.KindValidation {
		t.Fatalf("expected 422, got %v", err)
	}
}

func TestBookingCreateInactiveService(t *testing.T) {
	bs := newFakeBooking()
	svc := NewBookingService(bs, &fakeCatalog{err: repository.ErrNotFound}, &fakeProfile{complete: true}, fakeTx{})
	_, err := svc.Create(context.Background(), 7, validInput())
	he := httperr.As(err)
	if he == nil || he.Kind != httperr.KindNotFound {
		t.Fatalf("expected 404, got %v", err)
	}
}

func TestBookingShowOwnership(t *testing.T) {
	svc, bs := newBookingSvc()
	bs.byID[1] = &model.Booking{ID: 1, CustomerID: 7}
	if _, err := svc.Show(context.Background(), 7, 1); err != nil {
		t.Fatalf("show own: %v", err)
	}
	_, err := svc.Show(context.Background(), 99, 1) // wrong user
	he := httperr.As(err)
	if he == nil || he.Kind != httperr.KindForbidden {
		t.Fatalf("expected 403, got %v", err)
	}
}

func TestBookingCancelTransition(t *testing.T) {
	svc, bs := newBookingSvc()
	bs.byID[1] = &model.Booking{ID: 1, CustomerID: 7, Status: model.BookingStatusPendingPayment}
	b, err := svc.Cancel(context.Background(), 7, 1)
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if b.Status != model.BookingStatusCancelled {
		t.Fatalf("status: %s", b.Status)
	}
}

func TestBookingCancelInvalidState(t *testing.T) {
	svc, bs := newBookingSvc()
	bs.byID[1] = &model.Booking{ID: 1, CustomerID: 7, Status: model.BookingStatusCompleted}
	_, err := svc.Cancel(context.Background(), 7, 1)
	he := httperr.As(err)
	if he == nil || he.Kind != httperr.KindConflict {
		t.Fatalf("expected 409, got %v", err)
	}
}

func TestBookingCancelForbidden(t *testing.T) {
	svc, bs := newBookingSvc()
	bs.byID[1] = &model.Booking{ID: 1, CustomerID: 7, Status: model.BookingStatusPendingPayment}
	_, err := svc.Cancel(context.Background(), 99, 1)
	he := httperr.As(err)
	if he == nil || he.Kind != httperr.KindForbidden {
		t.Fatalf("expected 403, got %v", err)
	}
}

func TestBookingRepoError(t *testing.T) {
	bs := newFakeBooking()
	bs.err = errors.New("db")
	svc := NewBookingService(bs, &fakeCatalog{}, &fakeProfile{complete: true}, fakeTx{})
	_, err := svc.ListByCustomer(context.Background(), 7, 1, 15)
	if he := httperr.As(err); he == nil || he.Kind != httperr.KindInternal {
		t.Fatalf("expected 500: %v", err)
	}
}
