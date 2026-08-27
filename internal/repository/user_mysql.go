package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/Dwisajaa/golang-backend/internal/model"
)

const userColumns = "id, name, email, role, email_verified_at, password, remember_token, created_at, updated_at"

// MySQLUserRepository implements UserRepository/UserStore against MySQL.
type MySQLUserRepository struct {
	db *sql.DB
}

func NewMySQLUserRepository(db *sql.DB) *MySQLUserRepository {
	return &MySQLUserRepository{db: db}
}

// FindByID returns one user by primary key (see scanner helpers below).
func (r *MySQLUserRepository) FindByID(ctx context.Context, id uint64) (*model.User, error) {
	row := r.db.QueryRowContext(ctx,
		"SELECT "+userColumns+" FROM users WHERE id = ?", id)
	return scanUser(row)
}

func nullTimePtr(t sql.NullTime) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}

func nullStringPtr(s sql.NullString) *string {
	if !s.Valid {
		return nil
	}
	v := s.String
	return &v
}
