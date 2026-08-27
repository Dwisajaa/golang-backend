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

// TestMySQLTechnicianProfile_Coverage covers find/insert/update + unique
// behavior for technician profiles (FASE 7C-3). Gated by TEST_DATABASE_URL.
func TestMySQLTechnicianProfile_Coverage(t *testing.T) {
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

	store := NewMySQLTechnicianProfileStore()
	userStore := NewMySQLUserRepository(db)

	u := &model.User{Name: "T", Email: "tech-profile@example.test", Password: "h", Role: model.RoleTechnician}
	if err := userStore.Create(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}

	if _, err := store.FindByUserID(ctx, db, u.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound on missing profile, got %v", err)
	}

	now := time.Now().UTC()
	phone := "0811"
	p := &model.TechnicianProfile{
		UserID: u.ID, TechnicianCode: "TECH-9999", Phone: &phone,
		Specialization: strPtr("AC"), CreatedAt: &now, UpdatedAt: &now,
	}
	if err := store.InsertProfile(ctx, db, p); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if p.ID == 0 || !p.IsActive {
		t.Fatalf("insert must backfill id and default is_active=true: %+v", p)
	}
	got, err := store.FindByUserID(ctx, db, u.ID)
	if err != nil || got.TechnicianCode != "TECH-9999" || got.Specialization == nil {
		t.Fatalf("find after insert: %+v err=%v", got, err)
	}

	// code collision -> ErrDuplicateTechnicianCode
	u2 := &model.User{Name: "T2", Email: "tech-profile2@example.test", Password: "h", Role: model.RoleTechnician}
	if err := userStore.Create(ctx, u2); err != nil {
		t.Fatalf("create u2: %v", err)
	}
	dup := &model.TechnicianProfile{UserID: u2.ID, TechnicianCode: "TECH-9999", CreatedAt: &now, UpdatedAt: &now}
	if err := store.InsertProfile(ctx, db, dup); !errors.Is(err, ErrDuplicateTechnicianCode) {
		t.Fatalf("expected ErrDuplicateTechnicianCode, got %v", err)
	}

	// update path preserves code and refreshes nullable fields
	sp := strPtr("Wiring")
	up := &model.TechnicianProfile{Phone: nil, Specialization: sp, Address: nil, Bio: nil, UpdatedAt: &now}
	if err := store.UpdateProfile(ctx, db, u.ID, up); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, err = store.FindByUserID(ctx, db, u.ID)
	if err != nil || got.TechnicianCode != "TECH-9999" || got.Phone != nil {
		t.Fatalf("update not applied correctly: %+v err=%v", got, err)
	}
	if got.Specialization == nil || *got.Specialization != "Wiring" {
		t.Fatalf("specialization not updated: %+v", got)
	}
}

func strPtr(s string) *string { return &s }
