package repository

import (
	"context"

	"github.com/Dwisajaa/golang-backend/internal/model"
)

// InvoiceStore is the persistence contract for invoice reads (creation is a
// booking side-effect via BookingStore.CreateInvoice).
type InvoiceStore interface {
	CountByCustomer(ctx context.Context, q Queryer, customerID uint64) (int, error)
	ListByCustomer(ctx context.Context, q Queryer, customerID uint64, limit, offset int) ([]*model.Invoice, error)
	FindByID(ctx context.Context, q Queryer, id uint64) (*model.Invoice, error)
}
