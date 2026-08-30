package repository

import (
	"context"

	"github.com/Dwisajaa/golang-backend/internal/model"
)

// ReviewStore is the persistence contract for reviews.
type ReviewStore interface {
	// Create inserts a review; a duplicate booking_id surfaces as ErrDuplicate.
	Create(ctx context.Context, q Queryer, r *model.Review) error
	// FindByBooking returns the review for a booking (with customer+technician)
	// or ErrNotFound.
	FindByBooking(ctx context.Context, q Queryer, bookingID uint64) (*model.Review, error)
	// FindByID returns a review row (admin moderate).
	FindByID(ctx context.Context, q Queryer, id uint64) (*model.Review, error)
	// ReviewExists reports whether the booking already has a review.
	ReviewExists(ctx context.Context, q Queryer, bookingID uint64) (bool, error)
	// LatestAssignmentTechnicianID mirrors Booking::assignedTechnician() — the
	// technician of the newest assignment (by id) regardless of status.
	LatestAssignmentTechnicianID(ctx context.Context, q Queryer, bookingID uint64) (uint64, error)
	AdminCount(ctx context.Context, q Queryer, status string) (int, error)
	AdminList(ctx context.Context, q Queryer, status string, limit, offset int) ([]*model.Review, error)
	UpdateStatus(ctx context.Context, q Queryer, id uint64, status string) error
}
