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

func TestMySQLPayment_Coverage(t *testing.T) {
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

	// Ensure tables used by payment wiring exist (users for FK, invoices, bookings).
	ensureTables(t, db, ctx)
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS bookings (
		id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
		booking_code VARCHAR(255) NOT NULL, customer_id BIGINT UNSIGNED NOT NULL,
		booking_date DATE NOT NULL, booking_time VARCHAR(255) NOT NULL,
		address VARCHAR(255) NOT NULL, status VARCHAR(255) NOT NULL DEFAULT 'pending_payment',
		created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL,
		PRIMARY KEY (id)
	) ENGINE=InnoDB`); err != nil {
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
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS payments (
		id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
		invoice_id BIGINT UNSIGNED NOT NULL,
		payment_code VARCHAR(255) NOT NULL,
		payment_method VARCHAR(30) NOT NULL,
		amount DECIMAL(12,2) NOT NULL,
		paid_at TIMESTAMP NULL,
		status VARCHAR(20) NOT NULL DEFAULT 'waiting_verification',
		proof_image VARCHAR(255) NULL,
		customer_note TEXT NULL, admin_note TEXT NULL,
		verified_by BIGINT UNSIGNED NULL, verified_at TIMESTAMP NULL,
		created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL,
		PRIMARY KEY (id), UNIQUE KEY payments_payment_code_unique (payment_code)
	) ENGINE=InnoDB`); err != nil {
		t.Fatalf("ensure payments: %v", err)
	}

	// Minimal bookings/invoices referenced by payments (no FK constraints in
	// these test DDL to stay engine-friendly).
	now := time.Now().UTC()
	if _, err := db.ExecContext(ctx, "INSERT INTO bookings (id, booking_code, customer_id, booking_date, address, status) VALUES (1,'BJA-T',7,'2026-12-01','x','pending_payment')"); err != nil {
		t.Fatalf("seed booking: %v", err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO invoices (id, booking_id, invoice_number, status) VALUES (1,1,'INV-T','unpaid')"); err != nil {
		t.Fatalf("seed invoice: %v", err)
	}

	store := NewMySQLPaymentStore()

	lockInv, err := store.FindInvoiceForUpdate(ctx, db, 1)
	if err != nil || lockInv.Booking == nil || lockInv.Booking.Status != model.BookingStatusPendingPayment {
		t.Fatalf("find invoice for update: %+v err=%v", lockInv, err)
	}

	p := &model.Payment{
		InvoiceID: 1, PaymentCode: "PAY-BJA-T-0001", PaymentMethod: model.PaymentMethodBankTransfer,
		AmountCents: 30000, Status: model.PaymentStatusWaitingVerification,
		ProofImage: &[]string{"payment-proof-test.png"}[0],
		CreatedAt:  &now, UpdatedAt: &now,
	}
	if err := store.Create(ctx, db, p); err != nil {
		t.Fatalf("create: %v", err)
	}
	if p.ID == 0 {
		t.Fatal("id backfill")
	}

	if err := store.UpdateInvoiceStatus(ctx, db, 1, model.InvoiceStatusPendingPayment); err != nil {
		t.Fatalf("update invoice: %v", err)
	}
	if err := store.UpdateBookingStatus(ctx, db, 1, model.BookingStatusWaitingVerification); err != nil {
		t.Fatalf("update booking: %v", err)
	}

	locked, err := store.FindByIDForUpdate(ctx, db, p.ID)
	if err != nil || locked.Invoice == nil || locked.Invoice.Booking == nil {
		t.Fatalf("find for update: %+v err=%v", locked, err)
	}

	if err := store.MarkVerified(ctx, db, p.ID, 9); err != nil {
		t.Fatalf("mark verified: %v", err)
	}
	got, err := store.FindByIDNoLock(ctx, db, p.ID)
	if err != nil || got.Status != model.PaymentStatusPaid {
		t.Fatalf("after verify: %+v err=%v", got, err)
	}
	if got.VerifiedBy == nil || *got.VerifiedBy != 9 {
		t.Fatalf("verified_by not set: %+v", got)
	}

	if err := store.MarkRejected(ctx, db, p.ID, 9, "blur"); err != nil {
		t.Fatalf("mark rejected from paid: %v", err)
	}
	got, _ = store.FindByIDNoLock(ctx, db, p.ID)
	if got.Status != model.PaymentStatusRejected || got.AdminNote == nil {
		t.Fatalf("after reject: %+v", got)
	}

	total, err := store.AdminCount(ctx, db, "")
	if err != nil || total != 1 {
		t.Fatalf("admin count: %d err=%v", total, err)
	}
	list, err := store.AdminList(ctx, db, model.PaymentStatusRejected, 15, 0)
	if err != nil || len(list) != 1 {
		t.Fatalf("admin list: %+v err=%v", list, err)
	}

	// pending-duplicate guard query
	hasPending, err := store.HasPendingPayment(ctx, db, 1)
	if err != nil {
		t.Fatalf("has pending: %v", err)
	}
	if hasPending {
		t.Fatal("rejected payment should not count as pending")
	}

	// latest with proof
	latest, err := store.FindLatestWithProofByInvoice(ctx, db, 1)
	if err != nil || latest == nil || latest.ProofImage == nil {
		t.Fatalf("latest with proof: err=%v", err)
	}

	// code existence
	if exists, _ := store.PaymentCodeExists(ctx, db, "PAY-BJA-T-0001"); !exists {
		t.Fatal("payment code should exist")
	}
	// duplicate code classification
	dup := &model.Payment{InvoiceID: 1, PaymentCode: "PAY-BJA-T-0001", PaymentMethod: model.PaymentMethodBankTransfer, AmountCents: 1, Status: model.PaymentStatusWaitingVerification}
	if err := store.Create(ctx, db, dup); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("expected ErrDuplicate on dup code, got %v", err)
	}
}
