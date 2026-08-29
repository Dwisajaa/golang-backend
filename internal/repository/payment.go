package repository

import (
	"context"

	"github.com/Dwisajaa/golang-backend/internal/model"
)

// PaymentStore is the persistence contract for payments.
type PaymentStore interface {
	// Customer upload flow
	FindInvoiceForUpdate(ctx context.Context, q Queryer, invoiceID uint64) (*model.Invoice, error)
	HasPendingPayment(ctx context.Context, q Queryer, invoiceID uint64) (bool, error)
	Create(ctx context.Context, q Queryer, p *model.Payment) error
	UpdateInvoiceStatus(ctx context.Context, q Queryer, invoiceID uint64, status string) error
	UpdateBookingStatus(ctx context.Context, q Queryer, bookingID uint64, status string) error
	PaymentCodeExists(ctx context.Context, q Queryer, code string) (bool, error)
	// Latest payment with a proof for an invoice (customer proof download).
	FindLatestWithProofByInvoice(ctx context.Context, q Queryer, invoiceID uint64) (*model.Payment, error)

	// Admin flows
	AdminCount(ctx context.Context, q Queryer, status string) (int, error)
	AdminList(ctx context.Context, q Queryer, status string, limit, offset int) ([]*model.Payment, error)
	FindByID(ctx context.Context, q Queryer, id uint64) (*model.Payment, error)
	FindByIDForUpdate(ctx context.Context, q Queryer, id uint64) (*model.Payment, error)
	MarkVerified(ctx context.Context, q Queryer, id, verifiedBy uint64) error
	MarkRejected(ctx context.Context, q Queryer, id, verifiedBy uint64, adminNote string) error

	// Relation loading (batch, no N+1)
	AttachInvoices(ctx context.Context, q Queryer, payments []*model.Payment) error
}
