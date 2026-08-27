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

// TestMySQLTokenStore_CRUD requires a disposable test database via
// TEST_DATABASE_URL. Skipped (clearly) when the URL is absent; never touches
// production data.
func TestMySQLTokenStore_CRUD(t *testing.T) {
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

	user := &model.User{Name: "Token Test", Email: "token-test@example.test", Password: "h", Role: model.RoleCustomer}
	if err := NewMySQLUserRepository(db).Create(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}

	now := time.Now().UTC()
	expiresAt := now.Add(time.Hour).UTC()
	tok := &model.PersonalAccessToken{
		TokenableType: model.TokenableType,
		TokenableID:   user.ID,
		Name:          "mobile-app",
		Token:         "abcd1234hash",
		ExpiresAt:     &expiresAt,
		CreatedAt:     &now,
		UpdatedAt:     &now,
	}
	store := NewMySQLTokenStore(db)
	if err := store.Create(ctx, tok); err != nil {
		t.Fatalf("create token: %v", err)
	}
	if tok.ID == 0 {
		t.Fatal("token id not backfilled")
	}

	found, err := store.FindByTokenHash(ctx, "abcd1234hash")
	if err != nil {
		t.Fatalf("find by hash: %v", err)
	}
	if found.TokenableID != user.ID || found.Name != "mobile-app" || found.ExpiresAt == nil {
		t.Fatalf("unexpected token row: %+v", found)
	}

	if _, err := store.FindByTokenHash(ctx, "does-not-exist"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	if err := store.RevokeByTokenHash(ctx, "abcd1234hash"); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	// revoking again (missing row) must not error — Laravel logout parity
	if err := store.RevokeByTokenHash(ctx, "abcd1234hash"); err != nil {
		t.Fatalf("revoke missing row should be a no-op: %v", err)
	}
	// after revoke the token is gone
	if _, err := store.FindByTokenHash(ctx, "abcd1234hash"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after revoke, got %v", err)
	}
}

func ensureTables(t *testing.T, db *sql.DB, ctx context.Context) {
	t.Helper()
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users (
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
		) ENGINE=InnoDB`,
		`CREATE TABLE IF NOT EXISTS personal_access_tokens (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
			tokenable_type VARCHAR(255) NOT NULL,
			tokenable_id BIGINT UNSIGNED NOT NULL,
			name TEXT NOT NULL,
			token VARCHAR(64) NOT NULL,
			abilities TEXT NULL,
			last_used_at TIMESTAMP NULL,
			expires_at TIMESTAMP NULL,
			created_at TIMESTAMP NULL,
			updated_at TIMESTAMP NULL,
			PRIMARY KEY (id),
			UNIQUE KEY personal_access_tokens_token_unique (token)
		) ENGINE=InnoDB`,
	}
	for _, s := range stmts {
		if _, err := db.ExecContext(ctx, s); err != nil {
			t.Fatalf("ensure table: %v", err)
		}
	}
}
