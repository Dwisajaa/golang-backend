package repository

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
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
