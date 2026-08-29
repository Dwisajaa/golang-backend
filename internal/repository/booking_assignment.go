package repository

import (
	"context"
	"time"

	"github.com/Dwisajaa/golang-backend/internal/model"
)

// AssignmentStore is the persistence contract for booking assignments.
type AssignmentStore interface {
	// FindBookingForAssign locks the booking row and loads its invoice
	// (assignment eligibility + serialization of concurrent assigns).
	FindBookingForAssign(ctx context.Context, q Queryer, bookingID uint64) (*model.Booking, error)
	// FindTechnicianForAssign locks the user row and loads its technician
	// profile (eligibility check). ErrNotFound when no user.
	FindTechnicianForAssign(ctx context.Context, q Queryer, technicianID uint64) (*TechnicianUser, error)
	// FindActiveAssignment returns the latest pending/accepted assignment for
	// a booking, or ErrNotFound.
	FindActiveAssignment(ctx context.Context, q Queryer, bookingID uint64) (*model.BookingAssignment, error)
	// ReplaceAssignment marks an active assignment rejected with a reason.
	ReplaceAssignment(ctx context.Context, q Queryer, id uint64, rejectedAt time.Time, reason string) error
	// Create inserts a pending assignment and backfills its ID.
	Create(ctx context.Context, q Queryer, a *model.BookingAssignment) error
	UpdateBookingStatus(ctx context.Context, q Queryer, bookingID uint64, status string) error
	// LoadBookingForResponse loads booking + items + customer for the response.
	LoadBookingForResponse(ctx context.Context, q Queryer, bookingID uint64) (*model.Booking, error)
}

// TechnicianUser is the user row + its technician profile for eligibility.
type TechnicianUser struct {
	User              *model.User
	TechnicianProfile *model.TechnicianProfile
}
