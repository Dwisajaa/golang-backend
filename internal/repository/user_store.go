package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/go-sql-driver/mysql"

	"github.com/Dwisajaa/golang-backend/internal/model"
)

// FindByEmail returns one user by email, parameterized. No row -> ErrNotFound.
func (r *MySQLUserRepository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	row := r.db.QueryRowContext(ctx,
		"SELECT "+userColumns+" FROM users WHERE email = ?", email)
	return scanUser(row)
}

// Create inserts a user and writes back its auto-increment ID. A duplicate
// email surfaces as ErrDuplicateEmail (driver error 1062 translated), never a
// driver message.
func (r *MySQLUserRepository) Create(ctx context.Context, u *model.User) error {
	res, err := r.db.ExecContext(ctx,
		"INSERT INTO users (name, email, password, role) VALUES (?, ?, ?, ?)",
		u.Name, u.Email, u.Password, u.Role)
	if err != nil {
		var myErr *mysql.MySQLError
		if errors.As(err, &myErr) && myErr.Number == 1062 {
			return ErrDuplicateEmail
		}
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	u.ID = uint64(id)
	u.CreatedAt = nownil()
	u.UpdatedAt = nownil()
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUser(row rowScanner) (*model.User, error) {
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

func nownil() *time.Time {
	t := time.Now().UTC()
	return &t
}
