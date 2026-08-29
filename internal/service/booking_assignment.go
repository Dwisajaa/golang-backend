package service

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Dwisajaa/golang-backend/internal/httperr"
	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/repository"
)

type assignmentStore interface {
	FindBookingForAssign(ctx context.Context, q repository.Queryer, bookingID uint64) (*model.Booking, error)
	FindTechnicianForAssign(ctx context.Context, q repository.Queryer, technicianID uint64) (*repository.TechnicianUser, error)
	FindActiveAssignment(ctx context.Context, q repository.Queryer, bookingID uint64) (*model.BookingAssignment, error)
	ReplaceAssignment(ctx context.Context, q repository.Queryer, id uint64, rejectedAt time.Time, reason string) error
	Create(ctx context.Context, q repository.Queryer, a *model.BookingAssignment) error
	UpdateBookingStatus(ctx context.Context, q repository.Queryer, bookingID uint64, status string) error
	LoadBookingForResponse(ctx context.Context, q repository.Queryer, bookingID uint64) (*model.Booking, error)
}

// AssignmentService owns the admin assign flow.
type AssignmentService struct {
	assignments assignmentStore
	tx          txRunner
}

func NewAssignmentService(assignments assignmentStore, tx txRunner) *AssignmentService {
	return &AssignmentService{assignments: assignments, tx: tx}
}

// Assign mirrors Admin AssignmentController@assign.
func (s *AssignmentService) Assign(ctx context.Context, adminID, bookingID, technicianID uint64) (*model.BookingAssignment, error) {
	var out *model.BookingAssignment
	err := s.tx.Within(ctx, func(tx *sql.Tx) error {
		// Booking lock serializes concurrent assigns to the same booking.
		booking, err := s.assignments.FindBookingForAssign(ctx, tx, bookingID)
		if err != nil {
			return err
		}
		if booking.Status == model.BookingStatusCancelled {
			return httperr.Validation(map[string][]string{"booking": {"Cancelled bookings cannot be assigned."}})
		}
		if booking.Status != model.BookingStatusConfirmed || booking.Invoice == nil || booking.Invoice.Status != model.InvoiceStatusPaid {
			return httperr.Validation(map[string][]string{"booking": {"Booking must be confirmed and paid before assignment."}})
		}

		tech, err := s.assignments.FindTechnicianForAssign(ctx, tx, technicianID)
		if err != nil {
			return err
		}
		if tech.User.Role != model.RoleTechnician || tech.TechnicianProfile == nil || !tech.TechnicianProfile.IsActive {
			return httperr.Validation(map[string][]string{"technician_id": {"Technician is invalid or inactive."}})
		}

		now := time.Now().UTC()

		active, err := s.assignments.FindActiveAssignment(ctx, tx, bookingID)
		if err == nil {
			if err := s.assignments.ReplaceAssignment(ctx, tx, active.ID, now, model.ReplacementReason); err != nil {
				return err
			}
		} else if !errors.Is(err, repository.ErrNotFound) {
			return err
		}

		assignedBy := adminID
		a := &model.BookingAssignment{
			BookingID:    bookingID,
			TechnicianID: technicianID,
			AssignedBy:   &assignedBy,
			AssignedAt:   &now,
			Status:       model.AssignmentStatusPending,
		}
		if err := s.assignments.Create(ctx, tx, a); err != nil {
			return err
		}
		if err := s.assignments.UpdateBookingStatus(ctx, tx, bookingID, model.BookingStatusTechnicianAssigned); err != nil {
			return err
		}

		loaded, err := s.assignments.LoadBookingForResponse(ctx, tx, bookingID)
		if err != nil {
			return err
		}
		a.Booking = loaded
		out = a
		return nil
	})
	if err != nil {
		return nil, mapAssignmentErr(err)
	}
	// Notification to technician: DEFERRED (Notification domain not wired).
	return out, nil
}

func mapAssignmentErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, repository.ErrNotFound) {
		return httperr.NotFound("Resource not found.")
	}
	if he := httperr.As(err); he != nil {
		return he
	}
	return httperr.Internal(err)
}
