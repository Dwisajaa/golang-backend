package service

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Dwisajaa/golang-backend/internal/httperr"
	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/notify"
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

	FindWorkForUpdate(ctx context.Context, q repository.Queryer, id uint64) (*model.BookingAssignment, error)
	FetchByID(ctx context.Context, q repository.Queryer, id uint64) (*model.BookingAssignment, error)
	CountByTechnician(ctx context.Context, q repository.Queryer, technicianID uint64) (int, error)
	ListByTechnician(ctx context.Context, q repository.Queryer, technicianID uint64, limit, offset int) ([]*model.BookingAssignment, error)
	AttachJobRelations(ctx context.Context, q repository.Queryer, assignments []*model.BookingAssignment) error
	MarkAccepted(ctx context.Context, q repository.Queryer, id uint64, acceptedAt time.Time) error
	MarkRejected(ctx context.Context, q repository.Queryer, id uint64, rejectedAt time.Time, reason string) error
	MarkStarted(ctx context.Context, q repository.Queryer, id uint64, startedAt time.Time) error
	MarkCompleted(ctx context.Context, q repository.Queryer, id uint64, completedAt time.Time, note string) error
}

// AssignmentService owns the admin assign flow.
type AssignmentService struct {
	assignments assignmentStore
	tx          txRunner
	notify      notify.Notifier
}

func NewAssignmentService(assignments assignmentStore, tx txRunner, notify ...notify.Notifier) *AssignmentService {
	s := &AssignmentService{assignments: assignments, tx: tx}
	if len(notify) > 0 {
		s.notify = notify[0]
	}
	return s
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
	// Post-commit notification to the technician (assignment_created).
	if s.notify != nil && out != nil && out.Booking != nil {
		bk := out.BookingID
		ai := out.ID
		s.notify.NotifyUser(ctx, out.TechnicianID, model.SystemNotification{
			Event:     "assignment_created",
			Title:     "New job assignment",
			Message:   "You have been assigned booking " + out.Booking.BookingCode + ".",
			BookingID: &bk, AssignmentID: &ai,
		})
	}
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

// ---- Technician workflow (FASE 12B) ----

// JobList is the paginated technician job list.
type JobList struct {
	Items   []*model.BookingAssignment
	Total   int
	Page    int
	PerPage int
}

// ListJobs mirrors TechnicianJobController@index.
func (s *AssignmentService) ListJobs(ctx context.Context, technicianID uint64, page, perPage int) (*JobList, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 15
	}
	if perPage > 50 {
		perPage = 50
	}
	var out JobList
	out.Page, out.PerPage = page, perPage
	err := s.tx.Within(ctx, func(tx *sql.Tx) error {
		total, err := s.assignments.CountByTechnician(ctx, tx, technicianID)
		if err != nil {
			return err
		}
		items, err := s.assignments.ListByTechnician(ctx, tx, technicianID, perPage, (page-1)*perPage)
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

// ShowJob mirrors TechnicianJobController@show (policy view = owner technician).
func (s *AssignmentService) ShowJob(ctx context.Context, technicianID, assignmentID uint64) (*model.BookingAssignment, error) {
	var out *model.BookingAssignment
	err := s.tx.Within(ctx, func(tx *sql.Tx) error {
		a, err := s.assignments.FetchByID(ctx, tx, assignmentID)
		if err != nil {
			return err
		}
		if a.TechnicianID != technicianID {
			return htForbidden
		}
		if err := s.assignments.AttachJobRelations(ctx, tx, []*model.BookingAssignment{a}); err != nil {
			return err
		}
		out = a
		return nil
	})
	if err != nil {
		return nil, mapAssignmentErr(err)
	}
	return out, nil
}

// ShowJob mirrors TechnicianJobController@show (policy view = owner technician).
func (s *AssignmentService) Accept(ctx context.Context, technicianID, assignmentID uint64) (*model.BookingAssignment, error) {
	var out *model.BookingAssignment
	err := s.tx.Within(ctx, func(tx *sql.Tx) error {
		a, err := s.assignments.FindWorkForUpdate(ctx, tx, assignmentID)
		if err != nil {
			return err
		}
		if a.TechnicianID != technicianID {
			return htForbidden
		}
		if a.Status != model.AssignmentStatusPending {
			return httperr.Validation(map[string][]string{"assignment": {"Assignment is not in the required state."}})
		}
		if a.Booking == nil || a.Booking.Invoice == nil || a.Booking.Invoice.Status != model.InvoiceStatusPaid ||
			a.Booking.Status != model.BookingStatusTechnicianAssigned {
			return httperr.Validation(map[string][]string{"assignment": {"Booking is not ready for acceptance."}})
		}
		now := time.Now().UTC()
		if err := s.assignments.MarkAccepted(ctx, tx, assignmentID, now); err != nil {
			return err
		}
		a.Status = model.AssignmentStatusAccepted
		a.AcceptedAt = &now
		out = a
		return nil
	})
	if err != nil {
		return nil, mapAssignmentErr(err)
	}
	_ = s.loadForResponse(ctx, out)
	// Post-commit notification to admins (assignment_accepted).
	if s.notify != nil && out != nil && out.Booking != nil {
		bk := out.BookingID
		ai := out.ID
		s.notify.NotifyAdmins(ctx, model.SystemNotification{
			Event: "assignment_accepted", Title: "Assignment accepted",
			Message:   "Technician accepted booking " + out.Booking.BookingCode + ".",
			BookingID: &bk, AssignmentID: &ai,
		})
	}
	return out, nil
}

// Reject mirrors TechnicianJobController@reject.
func (s *AssignmentService) Reject(ctx context.Context, technicianID, assignmentID uint64, reason, detail string) (*model.BookingAssignment, error) {
	var out *model.BookingAssignment
	err := s.tx.Within(ctx, func(tx *sql.Tx) error {
		a, err := s.assignments.FindWorkForUpdate(ctx, tx, assignmentID)
		if err != nil {
			return err
		}
		if a.TechnicianID != technicianID {
			return htForbidden
		}
		if a.Status != model.AssignmentStatusPending {
			return httperr.Validation(map[string][]string{"assignment": {"Assignment is not in the required state."}})
		}
		fullReason := reason
		if detail != "" {
			fullReason = reason + " - " + detail
		}
		now := time.Now().UTC()
		if err := s.assignments.MarkRejected(ctx, tx, assignmentID, now, fullReason); err != nil {
			return err
		}
		if a.Booking != nil && a.Booking.Status == model.BookingStatusTechnicianAssigned {
			if !model.CanTransition(a.Booking.Status, model.BookingStatusConfirmed) {
				return httperr.Validation(map[string][]string{"booking": {"Invalid booking status transition."}})
			}
			if err := s.assignments.UpdateBookingStatus(ctx, tx, a.BookingID, model.BookingStatusConfirmed); err != nil {
				return err
			}
			a.Booking.Status = model.BookingStatusConfirmed
		}
		a.Status = model.AssignmentStatusRejected
		a.RejectedAt = &now
		frs := fullReason
		a.RejectionReason = &frs
		out = a
		return nil
	})
	if err != nil {
		return nil, mapAssignmentErr(err)
	}
	_ = s.loadForResponse(ctx, out)
	// Post-commit notification to admins (assignment_rejected).
	if s.notify != nil && out != nil && out.Booking != nil {
		bk := out.BookingID
		ai := out.ID
		s.notify.NotifyAdmins(ctx, model.SystemNotification{
			Event: "assignment_rejected", Title: "Assignment rejected",
			Message:   "Technician rejected booking " + out.Booking.BookingCode + ".",
			BookingID: &bk, AssignmentID: &ai,
		})
	}
	return out, nil
}

// Start mirrors TechnicianJobController@start.
func (s *AssignmentService) Start(ctx context.Context, technicianID, assignmentID uint64) (*model.BookingAssignment, error) {
	var out *model.BookingAssignment
	err := s.tx.Within(ctx, func(tx *sql.Tx) error {
		a, err := s.assignments.FindWorkForUpdate(ctx, tx, assignmentID)
		if err != nil {
			return err
		}
		if a.TechnicianID != technicianID {
			return htForbidden
		}
		if a.Status != model.AssignmentStatusAccepted {
			return httperr.Validation(map[string][]string{"assignment": {"Assignment is not in the required state."}})
		}
		if a.StartedAt != nil || a.Booking == nil || a.Booking.Status != model.BookingStatusTechnicianAssigned {
			return httperr.Validation(map[string][]string{"assignment": {"Assignment cannot be started in its current state."}})
		}
		now := time.Now().UTC()
		if err := s.assignments.MarkStarted(ctx, tx, assignmentID, now); err != nil {
			return err
		}
		if !model.CanTransition(a.Booking.Status, model.BookingStatusInProgress) {
			return httperr.Validation(map[string][]string{"booking": {"Invalid booking status transition."}})
		}
		if err := s.assignments.UpdateBookingStatus(ctx, tx, a.BookingID, model.BookingStatusInProgress); err != nil {
			return err
		}
		a.StartedAt = &now
		a.Booking.Status = model.BookingStatusInProgress
		out = a
		return nil
	})
	if err != nil {
		return nil, mapAssignmentErr(err)
	}
	_ = s.loadForResponse(ctx, out)
	// Post-commit notification to the customer (job_started).
	if s.notify != nil && out != nil && out.Booking != nil {
		bk := out.BookingID
		ai := out.ID
		s.notify.NotifyUser(ctx, out.Booking.CustomerID, model.SystemNotification{
			Event: "job_started", Title: "Job started",
			Message:   "Work has started for booking " + out.Booking.BookingCode + ".",
			BookingID: &bk, AssignmentID: &ai,
		})
	}
	return out, nil
}

// Complete mirrors TechnicianJobController@complete.
func (s *AssignmentService) Complete(ctx context.Context, technicianID, assignmentID uint64, note string) (*model.BookingAssignment, error) {
	var out *model.BookingAssignment
	err := s.tx.Within(ctx, func(tx *sql.Tx) error {
		a, err := s.assignments.FindWorkForUpdate(ctx, tx, assignmentID)
		if err != nil {
			return err
		}
		if a.TechnicianID != technicianID {
			return htForbidden
		}
		if a.Status != model.AssignmentStatusAccepted {
			return httperr.Validation(map[string][]string{"assignment": {"Assignment is not in the required state."}})
		}
		if a.StartedAt == nil || a.Booking == nil || a.Booking.Status != model.BookingStatusInProgress {
			return httperr.Validation(map[string][]string{"assignment": {"Job must be started before completion."}})
		}
		now := time.Now().UTC()
		if err := s.assignments.MarkCompleted(ctx, tx, assignmentID, now, note); err != nil {
			return err
		}
		if !model.CanTransition(a.Booking.Status, model.BookingStatusAwaitingVerification) {
			return httperr.Validation(map[string][]string{"booking": {"Invalid booking status transition."}})
		}
		if err := s.assignments.UpdateBookingStatus(ctx, tx, a.BookingID, model.BookingStatusAwaitingVerification); err != nil {
			return err
		}
		a.Status = model.AssignmentStatusCompleted
		a.CompletedAt = &now
		a.TechnicianNote = &note
		a.Booking.Status = model.BookingStatusAwaitingVerification
		out = a
		return nil
	})
	if err != nil {
		return nil, mapAssignmentErr(err)
	}
	_ = s.loadForResponse(ctx, out)
	// Post-commit notifications (job_completed): admins + customer.
	if s.notify != nil && out != nil && out.Booking != nil {
		bk := out.BookingID
		ai := out.ID
		s.notify.NotifyAdmins(ctx, model.SystemNotification{
			Event: "job_completed", Title: "Job completed",
			Message:   "Booking " + out.Booking.BookingCode + " is awaiting verification.",
			BookingID: &bk, AssignmentID: &ai,
		})
		s.notify.NotifyUser(ctx, out.Booking.CustomerID, model.SystemNotification{
			Event: "job_completed", Title: "Job completed",
			Message:   "Work for booking " + out.Booking.BookingCode + " has been completed.",
			BookingID: &bk, AssignmentID: &ai,
		})
	}
	return out, nil
}

// loadForResponse attaches booking+items+customer (best-effort post-commit).
func (s *AssignmentService) loadForResponse(ctx context.Context, a *model.BookingAssignment) error {
	if a == nil {
		return nil
	}
	return s.tx.Within(ctx, func(tx *sql.Tx) error {
		b, err := s.assignments.LoadBookingForResponse(ctx, tx, a.BookingID)
		if err != nil {
			return err
		}
		a.Booking = b
		return nil
	})
}
