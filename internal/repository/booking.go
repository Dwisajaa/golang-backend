package repository

import (
	"context"

	"github.com/Dwisajaa/golang-backend/internal/model"
)

// BookingStore is the persistence contract for bookings + their items + the
// invoice side-effect write.
type BookingStore interface {
	// Customer endpoints
	ListByCustomer(ctx context.Context, q Queryer, customerID uint64, limit, offset int) ([]*model.Booking, error)
	CountByCustomer(ctx context.Context, q Queryer, customerID uint64) (int, error)
	FindByID(ctx context.Context, q Queryer, id uint64) (*model.Booking, error)
	FindByIDForUpdate(ctx context.Context, q Queryer, id uint64) (*model.Booking, error)
	Create(ctx context.Context, q Queryer, b *model.Booking) error
	CreateItem(ctx context.Context, q Queryer, item *model.BookingItem) error
	CreateInvoice(ctx context.Context, q Queryer, inv *model.Invoice) error
	UpdateStatus(ctx context.Context, q Queryer, id uint64, status string) error
	UpdateInvoiceStatus(ctx context.Context, q Queryer, bookingID uint64, status string) error

	// Admin endpoints
	AdminCount(ctx context.Context, q Queryer, filters AdminBookingFilters) (int, error)
	AdminList(ctx context.Context, q Queryer, filters AdminBookingFilters, limit, offset int) ([]*model.Booking, error)

	// Attach helpers (batch, no N+1)
	AttachItems(ctx context.Context, q Queryer, bookings []*model.Booking) error
	AttachInvoices(ctx context.Context, q Queryer, bookings []*model.Booking) error

	// Code uniqueness
	BookingCodeExists(ctx context.Context, q Queryer, code string) (bool, error)
	InvoiceNumberExists(ctx context.Context, q Queryer, number string) (bool, error)
}

// AdminBookingFilters mirrors the admin booking list query parameters.
type AdminBookingFilters struct {
	Search string
	Status string
}
