package model

import "time"

// BookingAssignment status constants mirror Laravel BookingAssignment model.
const (
	AssignmentStatusPending   = "pending"
	AssignmentStatusAccepted  = "accepted"
	AssignmentStatusRejected  = "rejected"
	AssignmentStatusCompleted = "completed"
)

// IsActiveAssignment mirrors the "active" filter used by reassignment +
// admin booking list (pending or accepted).
func IsActiveAssignment(status string) bool {
	return status == AssignmentStatusPending || status == AssignmentStatusAccepted
}

// ReplacementReason is the message Laravel writes when an admin replaces an
// active assignment.
const ReplacementReason = "Assignment replaced by admin."

// BookingAssignment mirrors the booking_assignments table.
type BookingAssignment struct {
	ID                    uint64
	BookingID             uint64
	TechnicianID          uint64
	AssignedBy            *uint64
	AssignedAt            *time.Time
	AcceptedAt            *time.Time
	RejectedAt            *time.Time
	StartedAt             *time.Time
	CompletedAt           *time.Time
	Status                string
	RejectionReason       *string
	TechnicianNote        *string
	AdminVerificationNote *string
	CreatedAt             *time.Time
	UpdatedAt             *time.Time
	// Loaded relations (assign response).
	Booking *Booking
}
