package model

import "time"

// Invoice status constants mirror Laravel Invoice model.
const (
	InvoiceStatusUnpaid         = "unpaid"
	InvoiceStatusPendingPayment = "pending_payment"
	InvoiceStatusPaid           = "paid"
	InvoiceStatusCancelled      = "cancelled"
	InvoiceStatusExpired        = "expired"
)

// InvoiceStatuses mirrors Invoice::statuses() (audit; Laravel has no
// transitions() — states are driven by Booking/Payment flows in their
// services, so no transition map lives here).
var InvoiceStatuses = []string{
	InvoiceStatusUnpaid,
	InvoiceStatusPendingPayment,
	InvoiceStatusPaid,
	InvoiceStatusCancelled,
	InvoiceStatusExpired,
}

// Invoice mirrors the invoices table (1:1 with bookings via unique
// booking_id). Money fields are integer cents.
type Invoice struct {
	ID                  uint64
	BookingID           uint64
	InvoiceNumber       string
	IssuedAt            *time.Time
	DueAt               *time.Time
	SubtotalCents       int64
	AdditionalCostCents int64
	TotalAmountCents    int64
	Status              string
	Notes               *string
	CreatedAt           *time.Time
	UpdatedAt           *time.Time
	// Booking loaded for the detail/list endpoints (ownership policy).
	Booking *Booking
}

// InvoiceCodePrefix for generateInvoiceNumber.
const InvoiceCodePrefix = "INV-"
