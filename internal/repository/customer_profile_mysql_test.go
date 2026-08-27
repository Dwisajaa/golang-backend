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

// TestMySQLCustomerProfile_Upsert covers find/upsert and the unique user_id
// behavior for customer profiles (FASE 7C-2). Gated by TEST_DATABASE_URL.
func TestMySQLCustomerProfile_Upsert(t *testing.T) {
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

	store := NewMySQLCustomerProfileStore()
	userStore := NewMySQLUserRepository(db)

	u := &model.User{Name: "CP", Email: "cp@example.test", Password: "h", Role: model.RoleCustomer}
	if err := userStore.Create(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}

	now := time.Now().UTC()
	// missing -> ErrNotFound (Laravel data:null)
	if _, err := store.FindByUserID(ctx, db, u.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound on missing profile, got %v", err)
	}

	// create
	p := &model.CustomerProfile{UserID: u.ID, FullName: "A", Phone: "0812", Address: "Jl", City: "Jkt", CreatedAt: &now, UpdatedAt: &now}
	if err := store.Upsert(ctx, db, p); err != nil {
		t.Fatalf("upsert create: %v", err)
	}
	got, err := store.FindByUserID(ctx, db, u.ID)
	if err != nil || got.FullName != "A" {
		t.Fatalf("after create: %+v err=%v", got, err)
	}

	// update (same user_id, not a duplicate row)
	p2 := &model.CustomerProfile{UserID: u.ID, FullName: "B", Phone: "0813", Address: "Jl2", City: "Jkt2", CreatedAt: &now, UpdatedAt: &now}
	if err := store.Upsert(ctx, db, p2); err != nil {
		t.Fatalf("upsert update: %v", err)
	}
	got, err = store.FindByUserID(ctx, db, u.ID)
	if err != nil || got.FullName != "B" {
		t.Fatalf("after update: %+v err=%v", got, err)
	}

	// exactly one row for the user (unique constraint respected)
	var n int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM customer_profiles WHERE user_id = ?", u.ID).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected exactly 1 row for the user, got %d", n)
	}
}
