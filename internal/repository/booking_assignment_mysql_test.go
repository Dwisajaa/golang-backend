package repository

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/Dwisajaa/golang-backend/internal/model"
)

func TestMySQLAssignment_Coverage(t *testing.T) {
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
	ensureTables(t, db, ctx)

	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS bookings (
		id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
		booking_code VARCHAR(255) NOT NULL, customer_id BIGINT UNSIGNED NOT NULL,
		booking_date DATE NOT NULL, booking_time VARCHAR(255) NOT NULL,
		address VARCHAR(255) NOT NULL, status VARCHAR(255) NOT NULL DEFAULT 'pending_payment',
		created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL,
		PRIMARY KEY (id) ) ENGINE=InnoDB`); err != nil {
		t.Fatalf("ensure bookings: %v", err)
	}
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS invoices (
		id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
		booking_id BIGINT UNSIGNED NOT NULL, invoice_number VARCHAR(255) NOT NULL,
		status VARCHAR(255) NOT NULL DEFAULT 'unpaid',
		created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL,
		PRIMARY KEY (id) ) ENGINE=InnoDB`); err != nil {
		t.Fatalf("ensure invoices: %v", err)
	}
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS technician_profiles (
		id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
		user_id BIGINT UNSIGNED NOT NULL,
		technician_code VARCHAR(255) NOT NULL, is_active TINYINT(1) NOT NULL DEFAULT 1,
		created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL,
		PRIMARY KEY (id) ) ENGINE=InnoDB`); err != nil {
		t.Fatalf("ensure technician_profiles: %v", err)
	}
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS booking_assignments (
		id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
		booking_id BIGINT UNSIGNED NOT NULL,
		technician_id BIGINT UNSIGNED NOT NULL,
		assigned_by BIGINT UNSIGNED NULL,
		assigned_at TIMESTAMP NULL, accepted_at TIMESTAMP NULL, rejected_at TIMESTAMP NULL,
		started_at TIMESTAMP NULL, completed_at TIMESTAMP NULL,
		status VARCHAR(20) NOT NULL DEFAULT 'pending',
		rejection_reason TEXT NULL, technician_note TEXT NULL, admin_verification_note TEXT NULL,
		created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL,
		PRIMARY KEY (id), KEY b_a_status_tech (status, technician_id), KEY b_a_booking (booking_id)
	) ENGINE=InnoDB`); err != nil {
		t.Fatalf("ensure booking_assignments: %v", err)
	}
	// technician user (referenced by technician_id)
	if _, err := db.ExecContext(ctx, "INSERT INTO users (id, name, email, role, password) VALUES (9,'T','t@example.test','technician','h')"); err != nil {
		t.Fatalf("seed technician user: %v", err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO technician_profiles (user_id, technician_code, is_active) VALUES (9,'TECH-0001',1)"); err != nil {
		t.Fatalf("seed tech profile: %v", err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO bookings (id, booking_code, customer_id, booking_date, address, status) VALUES (1,'BJA-1',7,'2026-12-01','x','confirmed')"); err != nil {
		t.Fatalf("seed booking: %v", err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO invoices (id, booking_id, invoice_number, status) VALUES (1,1,'INV-1','paid')"); err != nil {
		t.Fatalf("seed invoice: %v", err)
	}

	store := NewMySQLAssignmentStore()

	booking, err := store.FindBookingForAssign(ctx, db, 1)
	if err != nil || booking.Status != model.BookingStatusConfirmed || booking.Invoice == nil {
		t.Fatalf("find booking for assign: %+v err=%v", booking, err)
	}

	tech, err := store.FindTechnicianForAssign(ctx, db, 9)
	if err != nil || tech.User.Role != model.RoleTechnician || tech.TechnicianProfile == nil || !tech.TechnicianProfile.IsActive {
		t.Fatalf("find tech: %+v err=%v", tech, err)
	}

	now := time.Now().UTC()
	a := &model.BookingAssignment{
		BookingID: 1, TechnicianID: 9, AssignedBy: &[]uint64{2}[0],
		AssignedAt: &now, Status: model.AssignmentStatusPending,
	}
	if err := store.Create(ctx, db, a); err != nil {
		t.Fatalf("create: %v", err)
	}
	if a.ID == 0 {
		t.Fatal("id backfill")
	}

	active, err := store.FindActiveAssignment(ctx, db, 1)
	if err != nil || active.ID != a.ID || active.Status != model.AssignmentStatusPending {
		t.Fatalf("find active: %+v err=%v", active, err)
	}

	if err := store.ReplaceAssignment(ctx, db, a.ID, now, model.ReplacementReason); err != nil {
		t.Fatalf("replace: %v", err)
	}
	active, _ = store.FindActiveAssignment(ctx, db, 1)
	if active != nil {
		t.Fatalf("active assignment should be gone after replace, got %+v", active)
	}

	// create a second pending to test LoadBookingForResponse preserves relations
	a2 := &model.BookingAssignment{BookingID: 1, TechnicianID: 9, AssignedBy: &[]uint64{2}[0], AssignedAt: &now, Status: model.AssignmentStatusPending}
	if err := store.Create(ctx, db, a2); err != nil {
		t.Fatalf("create a2: %v", err)
	}
	if err := store.UpdateBookingStatus(ctx, db, 1, model.BookingStatusTechnicianAssigned); err != nil {
		t.Fatalf("update booking status: %v", err)
	}
	loaded, err := store.LoadBookingForResponse(ctx, db, 1)
	if err != nil || loaded.BookingCode != "BJA-1" || loaded.Status != model.BookingStatusTechnicianAssigned {
		t.Fatalf("load for response: %+v err=%v", loaded, err)
	}

	if _, err := store.FindBookingForAssign(ctx, db, 999999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
