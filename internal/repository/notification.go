package repository

import (
	"context"
	"time"

	"github.com/Dwisajaa/golang-backend/internal/model"
)

// NotificationStore is the persistence contract for the notifications table.
type NotificationStore interface {
	// InsertNotification writes one database-channel notification (async in
	// Laravel via the database queue; Go writes it synchronously post-commit —
	// the observable row is the same).
	InsertNotification(ctx context.Context, q Queryer, recipientID uint64, n model.SystemNotification) error
	// AdminIDs returns ids of admin + super_admin users (batch recipient lookup).
	AdminIDs(ctx context.Context, q Queryer) ([]uint64, error)
	// Read API (own notifications).
	CountByUser(ctx context.Context, q Queryer, userID uint64) (int, error)
	ListByUser(ctx context.Context, q Queryer, userID uint64, limit, offset int) ([]*model.Notification, error)
	FindByUserAndID(ctx context.Context, q Queryer, userID uint64, id string) (*model.Notification, error)
	MarkRead(ctx context.Context, q Queryer, id string, now time.Time) error
	MarkAllRead(ctx context.Context, q Queryer, userID uint64) error
}
