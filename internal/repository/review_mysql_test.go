package repository

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"

	"github.com/Dwisajaa/golang-backend/internal/model"
)

func TestMySQLReview_Coverage(t *testing.T) {
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

	for _, ddl := range []string{
		`CREATE TABLE IF NOT EXISTS reviews (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
			booking_id BIGINT UNSIGNED NOT NULL,
			customer_id BIGINT UNSIGNED NOT NULL,
			technician_id BIGINT UNSIGNED NOT NULL,
			rating TINYINT UNSIGNED NOT NULL,
			comment TEXT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'published',
			created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL,
			PRIMARY KEY (id), UNIQUE KEY reviews_booking_id_unique (booking_id)
		) ENGINE=InnoDB`,
		`CREATE TABLE IF NOT EXISTS booking_assignments (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT, booking_id BIGINT UNSIGNED NOT NULL,
			technician_id BIGINT UNSIGNED NOT NULL, status VARCHAR(20) NOT NULL DEFAULT 'pending',
			created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL,
			PRIMARY KEY (id)) ENGINE=InnoDB`,
	} {
		if _, err := db.ExecContext(ctx, ddl); err != nil {
			t.Fatalf("ensure: %v", err)
		}
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO users (id, name, email, role, password) VALUES (7,'C','c@example.test','customer','h')"); err != nil {
		t.Fatalf("seed customer: %v", err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO users (id, name, email, role, password) VALUES (9,'T','t@example.test','technician','h')"); err != nil {
		t.Fatalf("seed tech: %v", err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO booking_assignments (id, booking_id, technician_id, status) VALUES (1, 55, 9, 'completed')"); err != nil {
		t.Fatalf("seed assignment: %v", err)
	}

	store := NewMySQLReviewStore()

	techID, err := store.LatestAssignmentTechnicianID(ctx, db, 55)
	if err != nil || techID != 9 {
		t.Fatalf("latest tech: %d err=%v", techID, err)
	}
	if _, err := store.LatestAssignmentTechnicianID(ctx, db, 999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	comment := "bagus"
	rv := &model.Review{BookingID: 55, CustomerID: 7, TechnicianID: 9, Rating: 5, Comment: &comment, Status: model.ReviewStatusPublished}
	if err := store.Create(ctx, db, rv); err != nil {
		t.Fatalf("create: %v", err)
	}
	if rv.ID == 0 {
		t.Fatal("id backfill")
	}

	if exists, _ := store.ReviewExists(ctx, db, 55); !exists {
		t.Fatal("ReviewExists should be true")
	}

	got, err := store.FindByBooking(ctx, db, 55)
	if err != nil || got.Rating != 5 || got.Customer == nil || got.Technician == nil {
		t.Fatalf("find: %+v err=%v", got, err)
	}

	// duplicate booking_id
	dup := &model.Review{BookingID: 55, CustomerID: 7, TechnicianID: 9, Rating: 4, Status: model.ReviewStatusPublished}
	if err := store.Create(ctx, db, dup); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("expected ErrDuplicate, got %v", err)
	}

	if err := store.UpdateStatus(ctx, db, rv.ID, model.ReviewStatusHidden); err != nil {
		t.Fatalf("update status: %v", err)
	}
	got, _ = store.FindByID(ctx, db, rv.ID)
	if got.Status != model.ReviewStatusHidden {
		t.Fatalf("status: %s", got.Status)
	}

	total, err := store.AdminCount(ctx, db, model.ReviewStatusHidden)
	if err != nil || total != 1 {
		t.Fatalf("admin count: %d err=%v", total, err)
	}
	list, err := store.AdminList(ctx, db, "", 15, 0)
	if err != nil || len(list) != 1 || list[0].Customer == nil {
		t.Fatalf("admin list: %+v err=%v", list, err)
	}
}
