package repository

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/Dwisajaa/golang-backend/internal/model"
)

func TestMySQLBooking_Coverage(t *testing.T) {
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
		`CREATE TABLE IF NOT EXISTS bookings (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
			booking_code VARCHAR(255) NOT NULL, customer_id BIGINT UNSIGNED NOT NULL,
			booking_date DATE NOT NULL, booking_time VARCHAR(255) NOT NULL,
			address VARCHAR(255) NOT NULL, address_detail VARCHAR(255) NULL,
			latitude DECIMAL(10,7) NULL, longitude DECIMAL(10,7) NULL,
			customer_note TEXT NULL, additional_jobdesk TEXT NULL,
			subtotal DECIMAL(12,2) NOT NULL DEFAULT 0.00,
			additional_cost DECIMAL(12,2) NOT NULL DEFAULT 0.00,
			total_price DECIMAL(12,2) NOT NULL DEFAULT 0.00,
			status VARCHAR(255) NOT NULL DEFAULT 'pending_payment',
			created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL,
			PRIMARY KEY (id), UNIQUE KEY bookings_booking_code_unique (booking_code)
		) ENGINE=InnoDB`,
		`CREATE TABLE IF NOT EXISTS booking_items (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
			booking_id BIGINT UNSIGNED NOT NULL, service_id BIGINT UNSIGNED NULL,
			package_id BIGINT UNSIGNED NULL, item_type VARCHAR(20) NOT NULL,
			item_name VARCHAR(255) NOT NULL, quantity INT NOT NULL DEFAULT 1,
			unit_price DECIMAL(12,2) NOT NULL DEFAULT 0.00,
			subtotal DECIMAL(12,2) NOT NULL DEFAULT 0.00,
			created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL,
			PRIMARY KEY (id)
		) ENGINE=InnoDB`,
		`CREATE TABLE IF NOT EXISTS invoices (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
			booking_id BIGINT UNSIGNED NOT NULL,
			invoice_number VARCHAR(255) NOT NULL,
			issued_at TIMESTAMP NOT NULL, due_at TIMESTAMP NULL,
			subtotal DECIMAL(12,2) NOT NULL DEFAULT 0.00,
			additional_cost DECIMAL(12,2) NOT NULL DEFAULT 0.00,
			total_amount DECIMAL(12,2) NOT NULL DEFAULT 0.00,
			status VARCHAR(255) NOT NULL DEFAULT 'unpaid',
			notes TEXT NULL,
			created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL,
			PRIMARY KEY (id), UNIQUE KEY invoices_invoice_number_unique (invoice_number)
		) ENGINE=InnoDB`,
	} {
		if _, err := db.ExecContext(ctx, ddl); err != nil {
			t.Fatalf("ensure: %v", err)
		}
	}

	store := NewMySQLBookingStore()
	now := time.Now().UTC()

	b := &model.Booking{
		BookingCode: "BJA-TEST-0001", CustomerID: 7,
		BookingDate: "2026-12-01", BookingTime: "09:00", Address: "Jl Test",
		SubtotalCents: 30000, TotalPriceCents: 30000,
		Status: model.BookingStatusPendingPayment, CreatedAt: &now, UpdatedAt: &now,
	}
	if err := store.Create(ctx, db, b); err != nil {
		t.Fatalf("create: %v", err)
	}
	if b.ID == 0 {
		t.Fatal("id backfill")
	}

	svcID := uint64(5)
	item := &model.BookingItem{
		BookingID: b.ID, ServiceID: &svcID, ItemType: "service",
		ItemName: "AC Service", Quantity: 2, UnitPriceCents: 15000, SubtotalCents: 30000,
	}
	if err := store.CreateItem(ctx, db, item); err != nil {
		t.Fatalf("create item: %v", err)
	}

	inv := &model.Invoice{
		BookingID: b.ID, InvoiceNumber: "INV-BJA-TEST-0001-0001",
		IssuedAt: &now, DueAt: &now, SubtotalCents: 30000, TotalAmountCents: 30000,
		Status: model.InvoiceStatusUnpaid,
	}
	if err := store.CreateInvoice(ctx, db, inv); err != nil {
		t.Fatalf("create invoice: %v", err)
	}

	got, err := store.FindByID(ctx, db, b.ID)
	if err != nil || got.BookingCode != "BJA-TEST-0001" || got.SubtotalCents != 30000 {
		t.Fatalf("find: %+v err=%v", got, err)
	}

	if err := store.AttachItems(ctx, db, []*model.Booking{got}); err != nil {
		t.Fatalf("attach items: %v", err)
	}
	if len(got.Items) != 1 || got.Items[0].ItemName != "AC Service" {
		t.Fatalf("items: %+v", got.Items)
	}

	if err := store.AttachInvoices(ctx, db, []*model.Booking{got}); err != nil {
		t.Fatalf("attach invoices: %v", err)
	}
	if got.Invoice == nil || got.Invoice.Status != model.InvoiceStatusUnpaid {
		t.Fatalf("invoice: %+v", got.Invoice)
	}

	if err := store.UpdateStatus(ctx, db, b.ID, model.BookingStatusCancelled); err != nil {
		t.Fatalf("update status: %v", err)
	}
	got, _ = store.FindByID(ctx, db, b.ID)
	if got.Status != model.BookingStatusCancelled {
		t.Fatalf("status: %s", got.Status)
	}
}
