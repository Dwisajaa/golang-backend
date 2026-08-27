package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Dwisajaa/golang-backend/internal/model"
)

// MySQLUserRepository implements UserRepository against MySQL via database/sql.
type MySQLUserRepository struct {
	db *sql.DB
}

func NewMySQLUserRepository(db *sql.DB) *MySQLUserRepository {
	return &MySQLUserRepository{db: db}
}

const userColumns = "id, name, email, role, email_verified_at, password, remember_token, created_at, updated_at"

// FindByID returns one user by primary key. No row found is translated to
// ErrNotFound so callers don't depend on the sql package.
func (r *MySQLUserRepository) FindByID(ctx context.Context, id uint64) (*model.User, error) {
	row := r.db.QueryRowContext(ctx,
		"SELECT "+userColumns+" FROM users WHERE id = ?", id)

	u := &model.User{}
	var emailVerifiedAt, createdAt, updatedAt sql.NullTime
	var rememberToken sql.NullString

	if err := row.Scan(
		&u.ID, &u.Name, &u.Email, &u.Role,
		&emailVerifiedAt, &u.Password, &rememberToken,
		&createdAt, &updatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	u.EmailVerifiedAt = nullTimePtr(emailVerifiedAt)
	u.RememberToken = nullStringPtr(rememberToken)
	u.CreatedAt = nullTimePtr(createdAt)
	u.UpdatedAt = nullTimePtr(updatedAt)
	return u, nil
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
