package model

import "time"

// Booking status constants mirror Laravel Booking model.
const (
	BookingStatusPendingPayment       = "pending_payment"
	BookingStatusWaitingVerification  = "waiting_verification"
	BookingStatusPaid                 = "paid"
	BookingStatusConfirmed            = "confirmed"
	BookingStatusTechnicianAssigned   = "technician_assigned"
	BookingStatusInProgress           = "in_progress"
	BookingStatusAwaitingVerification = "awaiting_verification"
	BookingStatusCompleted            = "completed"
	BookingStatusCancelled            = "cancelled"
)

// BookingTimeSlots mirrors Booking::TIME_SLOTS.
var BookingTimeSlots = []string{"08:00", "09:00", "10:00", "11:00", "13:00", "14:00", "15:00", "16:00"}

// BookingTransitions mirrors Booking::transitions() — the centralized state
// machine. Each key maps to the set of valid next states.
var BookingTransitions = map[string][]string{
	BookingStatusPendingPayment:       {BookingStatusWaitingVerification, BookingStatusCancelled},
	BookingStatusWaitingVerification:  {BookingStatusPendingPayment, BookingStatusPaid, BookingStatusCancelled},
	BookingStatusPaid:                 {BookingStatusConfirmed, BookingStatusCancelled},
	BookingStatusConfirmed:            {BookingStatusTechnicianAssigned, BookingStatusCancelled},
	BookingStatusTechnicianAssigned:   {BookingStatusInProgress, BookingStatusConfirmed, BookingStatusCancelled},
	BookingStatusInProgress:           {BookingStatusAwaitingVerification, BookingStatusCancelled},
	BookingStatusAwaitingVerification: {BookingStatusCompleted, BookingStatusInProgress},
}

// CanTransition checks whether the given transition is allowed.
func CanTransition(from, to string) bool {
	for _, s := range BookingTransitions[from] {
		if s == to {
			return true
		}
	}
	return false
}

// BookingCodePrefix for generateBookingCode.
const BookingCodePrefix = "BJA-"

// Booking mirrors the bookings table. Money fields are integer cents.
type Booking struct {
	ID                  uint64
	BookingCode         string
	CustomerID          uint64
	BookingDate         string // "2006-01-02"
	BookingTime         string
	Address             string
	AddressDetail       *string
	Latitude            *string // DECIMAL(10,7) stored as string for precision
	Longitude           *string
	CustomerNote        *string
	AdditionalJobdesk   *string
	SubtotalCents       int64
	AdditionalCostCents int64
	TotalPriceCents     int64
	Status              string
	CreatedAt           *time.Time
	UpdatedAt           *time.Time
	// Loaded relations (nil when not eager-loaded).
	Items   []*BookingItem
	Invoice *Invoice
	// Customer loaded by the assignment response (id, name, email).
	Customer *User
	// Assignments loaded by the verify response (latest_assignment).
	Assignments []*BookingAssignment
}

// BookingItem mirrors booking_items. Snapshot fields: item_name, unit_price,
// subtotal — these reflect the price at booking time, NOT current catalog.
type BookingItem struct {
	ID             uint64
	BookingID      uint64
	ServiceID      *uint64
	PackageID      *uint64
	ItemType       string // "service" or "package"
	ItemName       string // snapshot
	Quantity       int
	UnitPriceCents int64 // snapshot
	SubtotalCents  int64 // snapshot
	CreatedAt      *time.Time
	UpdatedAt      *time.Time
}

// Invoice is declared in invoice.go (same package).
