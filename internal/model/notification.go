package model

import "time"

// SystemNotification mirrors Laravel App\Notifications\SystemNotification data
// payload (stored as JSON in notifications.data). A notification is
// delivered via the database channel only (Laravel via() returns ['database']).
type SystemNotification struct {
	Event        string
	Title        string
	Message      string
	BookingID    *uint64
	InvoiceID    *uint64
	PaymentID    *uint64
	AssignmentID *uint64
	ActionURL    *string
}

// NotificationType is the morph type value Laravel stores in notifications.type.
const NotificationType = "App\\Notifications\\SystemNotification"

// Notification mirrors Laravel DatabaseNotification (notifications table).
type Notification struct {
	ID           string // UUID v4
	Type         string
	NotifiableID uint64
	Data         SystemNotification
	ReadAt       *time.Time
	CreatedAt    *time.Time
}
