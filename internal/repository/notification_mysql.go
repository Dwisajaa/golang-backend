package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Dwisajaa/golang-backend/internal/model"
)

const notificationColumns = "id, type, notifiable_id, data, read_at, created_at, updated_at"

// MySQLNotificationStore implements NotificationStore.
type MySQLNotificationStore struct{}

func NewMySQLNotificationStore() *MySQLNotificationStore { return &MySQLNotificationStore{} }

func (r *MySQLNotificationStore) InsertNotification(ctx context.Context, q Queryer, recipientID uint64, n model.SystemNotification) error {
	id := uuidV4()
	payload, err := payloadJSON(n)
	if err != nil {
		return err
	}
	_, err = q.ExecContext(ctx,
		`INSERT INTO notifications (id, type, notifiable_type, notifiable_id, data, read_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, NULL, NOW(), NOW())`,
		id, model.NotificationType, model.TokenableType, recipientID, payload)
	return err
}

func (r *MySQLNotificationStore) AdminIDs(ctx context.Context, q Queryer) ([]uint64, error) {
	rows, err := q.QueryContext(ctx,
		"SELECT id FROM users WHERE role IN ('admin', 'super_admin') ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uint64
	for rows.Next() {
		var id uint64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *MySQLNotificationStore) CountByUser(ctx context.Context, q Queryer, userID uint64) (int, error) {
	var n int
	err := q.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM notifications WHERE notifiable_type = ? AND notifiable_id = ?",
		model.TokenableType, userID).Scan(&n)
	return n, err
}

func (r *MySQLNotificationStore) ListByUser(ctx context.Context, q Queryer, userID uint64, limit, offset int) ([]*model.Notification, error) {
	rows, err := q.QueryContext(ctx,
		"SELECT "+notificationColumns+" FROM notifications WHERE notifiable_type = ? AND notifiable_id = ? ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?",
		model.TokenableType, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNotifications(rows)
}

func (r *MySQLNotificationStore) FindByUserAndID(ctx context.Context, q Queryer, userID uint64, id string) (*model.Notification, error) {
	row := q.QueryRowContext(ctx,
		"SELECT "+notificationColumns+" FROM notifications WHERE id = ? AND notifiable_type = ? AND notifiable_id = ?",
		id, model.TokenableType, userID)
	return scanNotificationRow(row)
}

func (r *MySQLNotificationStore) MarkRead(ctx context.Context, q Queryer, id string, now time.Time) error {
	_, err := q.ExecContext(ctx,
		"UPDATE notifications SET read_at = ?, updated_at = NOW() WHERE id = ?", now, id)
	return err
}

func (r *MySQLNotificationStore) MarkAllRead(ctx context.Context, q Queryer, userID uint64) error {
	_, err := q.ExecContext(ctx,
		"UPDATE notifications SET read_at = NOW(), updated_at = NOW() WHERE notifiable_type = ? AND notifiable_id = ? AND read_at IS NULL",
		model.TokenableType, userID)
	return err
}

func scanNotifications(rows *sql.Rows) ([]*model.Notification, error) {
	var out []*model.Notification
	for rows.Next() {
		n, err := scanNotificationCols(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func scanNotificationRow(s rowScanner) (*model.Notification, error) {
	n, err := scanNotificationCols(s)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return n, nil
}

func scanNotificationCols(s rowScanner) (*model.Notification, error) {
	n := &model.Notification{}
	var data string
	var readAt, ca, ua sql.NullTime
	if err := s.Scan(&n.ID, &n.Type, &n.NotifiableID, &data, &readAt, &ca, &ua); err != nil {
		return nil, err
	}
	n.Data = parsePayload(data)
	n.ReadAt = nullTimePtr(readAt)
	n.CreatedAt = nullTimePtr(ca)
	return n, nil
}

// payloadJSON encodes the notification data exactly like Laravel
// SystemNotification::toDatabase (JSON map with null leave keys).
func payloadJSON(n model.SystemNotification) (string, error) {
	kv := map[string]any{
		"event": n.Event, "title": n.Title, "body": n.Message,
		"booking_id": ptrOrNil(n.BookingID), "invoice_id": ptrOrNil(n.InvoiceID),
		"payment_id": ptrOrNil(n.PaymentID), "assignment_id": ptrOrNil(n.AssignmentID),
		"action_url": ptrOrNilString(n.ActionURL),
	}
	b, err := json.Marshal(kv)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func parsePayload(data string) model.SystemNotification {
	var m map[string]any
	if json.Unmarshal([]byte(data), &m) != nil {
		return model.SystemNotification{}
	}
	return model.SystemNotification{
		Event:        strVal(m["event"]),
		Title:        strVal(m["title"]),
		Message:      strVal(m["body"]),
		BookingID:    u64OrNil(m["booking_id"]),
		InvoiceID:    u64OrNil(m["invoice_id"]),
		PaymentID:    u64OrNil(m["payment_id"]),
		AssignmentID: u64OrNil(m["assignment_id"]),
		ActionURL:    strPtrOrNil(m["action_url"]),
	}
}

func uuidV4() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("fallback-%x", time.Now().UnixNano())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	hexstr := hex.EncodeToString(b[:])
	return hexstr[0:8] + "-" + hexstr[8:12] + "-" + hexstr[12:16] + "-" + hexstr[16:20] + "-" + hexstr[20:32]
}

func ptrOrNil(v *uint64) any {
	if v == nil {
		return nil
	}
	return *v
}

func ptrOrNilString(v *string) any {
	if v == nil {
		return nil
	}
	return *v
}

func strPtrOrNil(v any) *string {
	if s, ok := v.(string); ok && s != "" {
		return &s
	}
	return nil
}

func u64OrNil(v any) *uint64 {
	if f, ok := v.(float64); ok && f > 0 {
		u := uint64(f)
		return &u
	}
	return nil
}

func strVal(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
