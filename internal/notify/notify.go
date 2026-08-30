// Package notify delivers database-channel system notifications (Laravel
// SystemNotification via ['database']). Business services call Notify only
// AFTER their business transaction commits — a failure here must never roll
// back or fail the committed business result.
package notify

import (
	"context"
	"log/slog"

	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/repository"
)

// Notifier is the seam business services use to emit DB notifications.
type Notifier interface {
	// NotifyUser writes one notification for a single user.
	NotifyUser(ctx context.Context, recipientID uint64, n model.SystemNotification)
	// NotifyAdmins writes one notification per admin/super_admin.
	NotifyAdmins(ctx context.Context, n model.SystemNotification)
}

// DBNotifier writes database-channel notification rows on the pool (post-commit).
type DBNotifier struct {
	store repository.NotificationStore
	db    repository.Queryer
	log   *slog.Logger
}

func NewDBNotifier(store repository.NotificationStore, db repository.Queryer, log *slog.Logger) *DBNotifier {
	return &DBNotifier{store: store, db: db, log: log}
}

func (n *DBNotifier) NotifyUser(ctx context.Context, recipientID uint64, notif model.SystemNotification) {
	if err := n.store.InsertNotification(ctx, n.db, recipientID, notif); err != nil {
		n.log.Warn("notification_dispatch_failed", "recipient", recipientID, "event", notif.Event, "error", err.Error())
	}
}

func (n *DBNotifier) NotifyAdmins(ctx context.Context, notif model.SystemNotification) {
	ids, err := n.store.AdminIDs(ctx, n.db)
	if err != nil {
		n.log.Warn("notification_admin_lookup_failed", "event", notif.Event, "error", err.Error())
		return
	}
	for _, id := range ids {
		n.NotifyUser(ctx, id, notif)
	}
}

// Noop is the nil-safe fallback for tests/optional wiring.
type Noop struct{}

func (Noop) NotifyUser(ctx context.Context, recipientID uint64, n model.SystemNotification) {}
func (Noop) NotifyAdmins(ctx context.Context, n model.SystemNotification)                   {}
