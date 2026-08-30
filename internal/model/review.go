package model

import "time"

// Review status constants mirror Laravel Review model.
const (
	ReviewStatusPublished = "published"
	ReviewStatusHidden    = "hidden"
	ReviewStatusRejected  = "rejected"
)

// ReviewStatuses mirrors Review::statuses().
var ReviewStatuses = []string{ReviewStatusPublished, ReviewStatusHidden, ReviewStatusRejected}

// Review mirrors the reviews table. One review per booking (booking_id unique).
type Review struct {
	ID           uint64
	BookingID    uint64
	CustomerID   uint64
	TechnicianID uint64
	Rating       int
	Comment      *string
	Status       string
	CreatedAt    *time.Time
	// Loaded relations for the resource.
	Customer   *User
	Technician *User
}
