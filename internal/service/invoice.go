package service

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Dwisajaa/golang-backend/internal/httperr"
	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/repository"
)

type invoiceStore interface {
	CountByCustomer(ctx context.Context, q repository.Queryer, customerID uint64) (int, error)
	ListByCustomer(ctx context.Context, q repository.Queryer, customerID uint64, limit, offset int) ([]*model.Invoice, error)
	FindByID(ctx context.Context, q repository.Queryer, id uint64) (*model.Invoice, error)
}

// InvoiceService owns the customer invoice read flows (list/detail). Creation
// is a booking side-effect (BookingStore.CreateInvoice); payment-state flows
// belong to the deferred Payment domain.
type InvoiceService struct {
	invoices invoiceStore
	tx       txRunner
}

func NewInvoiceService(invoices invoiceStore, tx txRunner) *InvoiceService {
	return &InvoiceService{invoices: invoices, tx: tx}
}

// InvoiceList is the paginated customer invoice list.
type InvoiceList struct {
	Items   []*model.Invoice
	Total   int
	Page    int
	PerPage int
}

// ListByCustomer mirrors InvoiceController@index: invoices whose booking
// belongs to the customer, booking loaded, latest first.
func (s *InvoiceService) ListByCustomer(ctx context.Context, customerID uint64, page, perPage int) (*InvoiceList, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 15
	}
	if perPage > 50 {
		perPage = 50
	}
	var out InvoiceList
	out.Page, out.PerPage = page, perPage
	err := s.tx.Within(ctx, func(tx *sql.Tx) error {
		total, err := s.invoices.CountByCustomer(ctx, tx, customerID)
		if err != nil {
			return err
		}
		items, err := s.invoices.ListByCustomer(ctx, tx, customerID, perPage, (page-1)*perPage)
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

// Show mirrors InvoiceController@show with InvoicePolicy.view: the caller
// must own the invoice's booking (403 otherwise).
func (s *InvoiceService) Show(ctx context.Context, customerID, invoiceID uint64) (*model.Invoice, error) {
	var out *model.Invoice
	err := s.tx.Within(ctx, func(tx *sql.Tx) error {
		inv, err := s.invoices.FindByID(ctx, tx, invoiceID)
		if err != nil {
			return err
		}
		if inv.Booking == nil || inv.Booking.CustomerID != customerID {
			return htForbidden
		}
		out = inv
		return nil
	})
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, httperr.NotFound("Resource not found.")
		}
		return nil, mapInvoiceErr(err)
	}
	return out, nil
}

// htForbidden is a package-level typed forbidden error.
var htForbidden = httperr.Forbidden("Forbidden.")

func mapInvoiceErr(err error) error {
	if err == nil {
		return nil
	}
	if he := httperr.As(err); he != nil {
		return he
	}
	return httperr.Internal(err)
}
