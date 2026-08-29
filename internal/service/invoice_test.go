package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Dwisajaa/golang-backend/internal/httperr"
	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/repository"
)

type fakeInvoiceStore struct {
	invoices []*model.Invoice
	byID     map[uint64]*model.Invoice
	total    int
	err      error
}

func (f *fakeInvoiceStore) CountByCustomer(ctx context.Context, q repository.Queryer, cid uint64) (int, error) {
	return f.total, f.err
}
func (f *fakeInvoiceStore) ListByCustomer(ctx context.Context, q repository.Queryer, cid uint64, l, o int) ([]*model.Invoice, error) {
	return f.invoices, f.err
}
func (f *fakeInvoiceStore) FindByID(ctx context.Context, q repository.Queryer, id uint64) (*model.Invoice, error) {
	if f.err != nil {
		return nil, f.err
	}
	if inv, ok := f.byID[id]; ok {
		return inv, nil
	}
	return nil, repository.ErrNotFound
}

func TestInvoiceListOwned(t *testing.T) {
	fake := &fakeInvoiceStore{total: 1, invoices: []*model.Invoice{{ID: 1, InvoiceNumber: "INV-BJA-1", TotalAmountCents: 30000}}}
	svc := NewInvoiceService(fake, fakeTx{})

	list, err := svc.ListByCustomer(context.Background(), 7, 1, 15)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if list.Total != 1 || list.PerPage != 15 {
		t.Fatalf("meta wrong: %+v", list)
	}
}

func TestInvoiceShowOwned(t *testing.T) {
	fake := &fakeInvoiceStore{byID: map[uint64]*model.Invoice{
		1: {ID: 1, BookingID: 10, InvoiceNumber: "INV-1", Booking: &model.Booking{ID: 10, CustomerID: 7}},
	}}
	svc := NewInvoiceService(fake, fakeTx{})

	if _, err := svc.Show(context.Background(), 7, 1); err != nil {
		t.Fatalf("own: %v", err)
	}
	_, err := svc.Show(context.Background(), 99, 1)
	he := httperr.As(err)
	if he == nil || he.Kind != httperr.KindForbidden {
		t.Fatalf("expected 403, got %v", err)
	}
}

func TestInvoiceShowNotFound(t *testing.T) {
	fake := &fakeInvoiceStore{byID: map[uint64]*model.Invoice{}}
	svc := NewInvoiceService(fake, fakeTx{})
	_, err := svc.Show(context.Background(), 7, 99)
	if he := httperr.As(err); he == nil || he.Kind != httperr.KindNotFound {
		t.Fatalf("expected 404, got %v", err)
	}
}

func TestInvoiceShowNoBookingRow(t *testing.T) {
	fake := &fakeInvoiceStore{byID: map[uint64]*model.Invoice{1: {ID: 1, BookingID: 10, InvoiceNumber: "INV-1"}}} // Booking nil
	svc := NewInvoiceService(fake, fakeTx{})
	_, err := svc.Show(context.Background(), 7, 1)
	if he := httperr.As(err); he == nil || he.Kind != httperr.KindForbidden {
		t.Fatalf("expected 403 for nil booking, got %v", err)
	}
}

func TestInvoiceRepoError(t *testing.T) {
	fake := &fakeInvoiceStore{err: errors.New("db")}
	svc := NewInvoiceService(fake, fakeTx{})
	if _, err := svc.ListByCustomer(context.Background(), 7, 1, 15); httperr.As(err) == nil {
		t.Fatalf("expected error, got nil")
	}
}
