package repository

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/Dwisajaa/golang-backend/internal/model"
)

func timeNowUTC() time.Time { return time.Now().UTC() }

func TestMySQLNotification_Coverage(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; repository integration requires a disposable MySQL test database")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	ctx := context.Background()

	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS notifications (
		id CHAR(36) NOT NULL, type VARCHAR(255) NOT NULL,
		notifiable_type VARCHAR(255) NOT NULL, notifiable_id BIGINT UNSIGNED NOT NULL,
		data TEXT NOT NULL, read_at TIMESTAMP NULL,
		created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL,
		PRIMARY KEY (id)
	) ENGINE=InnoDB`); err != nil {
		t.Fatalf("ensure notifications: %v", err)
	}

	store := NewMySQLNotificationStore()
	bk := uint64(11)
	n := model.SystemNotification{
		Event: "payment_verified", Title: "Payment verified", Message: "ok",
		BookingID: &bk, PaymentID: &bk,
	}
	if err := store.InsertNotification(ctx, db, 7, n); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := store.InsertNotification(ctx, db, 7, model.SystemNotification{Event: "read_me"}); err != nil {
		t.Fatalf("insert2: %v", err)
	}

	total, err := store.CountByUser(ctx, db, 7)
	if err != nil || total != 2 {
		t.Fatalf("count: %d err=%v", total, err)
	}
	items, err := store.ListByUser(ctx, db, 7, 15, 0)
	if err != nil || len(items) != 2 {
		t.Fatalf("list: %d err=%v", len(items), err)
	}
	if items[0].Data.Event != "read_me" && items[0].Data.Event != "payment_verified" {
		t.Fatalf("payload decode wrong: %+v", items[0].Data)
	}

	got, err := store.FindByUserAndID(ctx, db, 7, items[0].ID)
	if err != nil || got.ID != items[0].ID {
		t.Fatalf("find: %+v err=%v", got, err)
	}
	if _, err := store.FindByUserAndID(ctx, db, 99, items[0].ID); err == nil {
		t.Fatal("expected not found for other user")
	}

	if err := store.MarkRead(ctx, db, items[0].ID, timeNowUTC()); err != nil {
		t.Fatalf("mark read: %v", err)
	}
	if err := store.MarkAllRead(ctx, db, 7); err != nil {
		t.Fatalf("mark all: %v", err)
	}

	adminIDs, err := store.AdminIDs(ctx, db)
	if err != nil {
		t.Fatalf("admin ids: %v", err)
	}
	_ = adminIDs
}
