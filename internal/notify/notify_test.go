package notify

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/repository"
)

type errBoom struct{}

func (errBoom) Error() string { return "boom" }

type recordingStore struct {
	mu       sync.Mutex
	rows     []model.SystemNotification
	adminIDs []uint64
	err      error
}

func (r *recordingStore) InsertNotification(ctx context.Context, q repository.Queryer, recipientID uint64, n model.SystemNotification) error {
	if r.err != nil {
		return r.err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rows = append(r.rows, n)
	return nil
}

func (r *recordingStore) AdminIDs(ctx context.Context, q repository.Queryer) ([]uint64, error) {
	return r.adminIDs, r.err
}
func (r *recordingStore) CountByUser(ctx context.Context, q repository.Queryer, userID uint64) (int, error) {
	return 0, r.err
}
func (r *recordingStore) ListByUser(ctx context.Context, q repository.Queryer, userID uint64, l, o int) ([]*model.Notification, error) {
	return nil, r.err
}
func (r *recordingStore) FindByUserAndID(ctx context.Context, q repository.Queryer, userID uint64, id string) (*model.Notification, error) {
	return nil, r.err
}
func (r *recordingStore) MarkRead(ctx context.Context, q repository.Queryer, id string, now time.Time) error {
	return r.err
}
func (r *recordingStore) MarkAllRead(ctx context.Context, q repository.Queryer, userID uint64) error {
	return r.err
}

// stubQ satisfies repository.Queryer for the notifier's pool handle.
type stubQ struct{}

func (stubQ) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return nil, nil
}
func (stubQ) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row { return nil }
func (stubQ) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return nil, nil
}

func loggerNoop() *slog.Logger { return slog.New(slog.NewTextHandler(nopWriter{}, nil)) }

type nopWriter struct{}

func (nopWriter) Write(p []byte) (int, error) { return len(p), nil }

func TestNotifyUserAndAdmins(t *testing.T) {
	st := &recordingStore{adminIDs: []uint64{1, 2}}
	n := &DBNotifier{store: st, db: stubQ{}, log: loggerNoop()}
	bk := uint64(9)
	n.NotifyUser(context.Background(), 7, model.SystemNotification{Event: "e", Title: "t", Message: "m", BookingID: &bk})
	st.mu.Lock()
	userRows := len(st.rows)
	st.mu.Unlock()
	if userRows != 1 {
		t.Fatalf("user rows: %d", userRows)
	}

	n.NotifyAdmins(context.Background(), model.SystemNotification{Event: "e2"})
	st.mu.Lock()
	total := len(st.rows)
	st.mu.Unlock()
	if total != 3 {
		t.Fatalf("expected 1 user + 2 admin rows, got %d", total)
	}
}

func TestNotifyFailureIsLoggedNotFatal(t *testing.T) {
	st := &recordingStore{err: errBoom{}, adminIDs: []uint64{1}}
	n := &DBNotifier{store: st, db: stubQ{}, log: loggerNoop()}
	n.NotifyUser(context.Background(), 7, model.SystemNotification{Event: "e"})
	n.NotifyAdmins(context.Background(), model.SystemNotification{Event: "e"}) // no panic, logged
}
