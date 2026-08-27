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

// TestMySQLUserRepository_FindByID requires a disposable test database:
//
//	TEST_DATABASE_URL="user:pass@tcp(host:3306)/testdb?parseTime=true&charset=utf8mb4"
//
// It never touches production data. Skipped (clearly) when the URL is absent.
func TestMySQLUserRepository_FindByID(t *testing.T) {
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

	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS users (
		id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
		name VARCHAR(255) NOT NULL,
		email VARCHAR(255) NOT NULL,
		role VARCHAR(30) NOT NULL DEFAULT 'customer',
		email_verified_at TIMESTAMP NULL,
		password VARCHAR(255) NOT NULL,
		remember_token VARCHAR(100) NULL,
		created_at TIMESTAMP NULL,
		updated_at TIMESTAMP NULL,
		PRIMARY KEY (id),
		UNIQUE KEY users_email_unique (email)
	) ENGINE=InnoDB`); err != nil {
		t.Fatalf("ensure users: %v", err)
	}

	res, err := db.ExecContext(ctx,
		`INSERT INTO users (name, email, role, password) VALUES (?, ?, ?, ?)`,
		"Repo Test", "repo-test@example.test", "customer", "hashed")
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	id, _ := res.LastInsertId()

	repo := NewMySQLUserRepository(db)
	u, err := repo.FindByID(ctx, uint64(id))
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if u.Name != "Repo Test" || u.Email != "repo-test@example.test" || u.Role != "customer" {
		t.Fatalf("wrong user: %+v", u)
	}
	if u.Password != "hashed" {
		t.Fatalf("password not scanned: %q", u.Password)
	}

	if _, err := repo.FindByID(ctx, 9999999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// TestMySQLUserRepository_ProfileWrites covers profile/password updates and
// all-token revocation (FASE 7C-1). Gated by TEST_DATABASE_URL.
func TestMySQLUserRepository_ProfileWrites(t *testing.T) {
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

	repo := NewMySQLUserRepository(db)
	u := &model.User{Name: "P", Email: "p@example.test", Password: "h1", Role: model.RoleCustomer}
	if err := repo.Create(ctx, u); err != nil {
		t.Fatalf("create: %v", err)
	}

	now := time.Now().UTC()

	// UpdateNameEmail with unchanged email keeps verification.
	verified := now
	if err := repo.UpdateNameEmail(ctx, db, u.ID, "P2", "p@example.test", &verified); err != nil {
		t.Fatalf("update name/email: %v", err)
	}
	got, err := repo.FindByID(ctx, u.ID)
	if err != nil || got.Name != "P2" || got.EmailVerifiedAt == nil || !got.EmailVerifiedAt.Equal(verified.Truncate(time.Second)) {
		t.Fatalf("profile update not reflected: %+v err=%v", got, err)
	}

	// email change with nil verifiedAt
	if err := repo.UpdateNameEmail(ctx, db, u.ID, "P2", "p2@example.test", nil); err != nil {
		t.Fatalf("email change: %v", err)
	}
	// duplicate email on another user -> ErrDuplicateEmail
	u2 := &model.User{Name: "Q", Email: "q@example.test", Password: "h", Role: model.RoleCustomer}
	if err := repo.Create(ctx, u2); err != nil {
		t.Fatalf("create u2: %v", err)
	}
	if err := repo.UpdateNameEmail(ctx, db, u2.ID, "Q", "p2@example.test", nil); !errors.Is(err, ErrDuplicateEmail) {
		t.Fatalf("expected ErrDuplicateEmail, got %v", err)
	}

	// UpdatePassword writes hash
	if err := repo.UpdatePassword(ctx, db, u.ID, "new-hash"); err != nil {
		t.Fatalf("update password: %v", err)
	}
	if got, err := repo.FindByID(ctx, u.ID); err != nil || got.Password != "new-hash" {
		t.Fatalf("password not updated: %+v err=%v", got, err)
	}

	// tokens: create one then RevokeAllForUser removes it
	tokenStore := NewMySQLTokenStore(db)
	exp := now.Add(time.Hour).UTC()
	tok := &model.PersonalAccessToken{
		TokenableType: model.TokenableType, TokenableID: u.ID, Name: "mobile-app",
		Token: "profile-hash", ExpiresAt: &exp, CreatedAt: &now, UpdatedAt: &now,
	}
	if err := tokenStore.Create(ctx, tok); err != nil {
		t.Fatalf("create token: %v", err)
	}
	if err := tokenStore.RevokeAllForUser(ctx, db, u.ID); err != nil {
		t.Fatalf("revoke all: %v", err)
	}
	if _, err := tokenStore.FindByTokenHash(ctx, "profile-hash"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected token gone after revoke-all, got %v", err)
	}
}
