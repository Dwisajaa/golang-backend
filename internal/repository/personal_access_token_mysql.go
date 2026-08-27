package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Dwisajaa/golang-backend/internal/model"
)

const tokenColumns = "id, tokenable_type, tokenable_id, name, token, abilities, last_used_at, expires_at, created_at, updated_at"

// MySQLTokenStore implements TokenStore via personal_access_tokens.
type MySQLTokenStore struct {
	db *sql.DB
}

func NewMySQLTokenStore(db *sql.DB) *MySQLTokenStore {
	return &MySQLTokenStore{db: db}
}

// Create persists a token row. The Token field is the SHA-256 hash — never the
// raw token (see docs/authentication-core.md).
func (s *MySQLTokenStore) Create(ctx context.Context, t *model.PersonalAccessToken) error {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO personal_access_tokens
		   (tokenable_type, tokenable_id, name, token, abilities, expires_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		t.TokenableType, t.TokenableID, t.Name, t.Token, t.Abilities,
		t.ExpiresAt, t.CreatedAt, t.UpdatedAt)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	t.ID = uint64(id)
	return nil
}

// FindByTokenHash finds the token row by its SHA-256 hash. No row -> ErrNotFound.
func (s *MySQLTokenStore) FindByTokenHash(ctx context.Context, tokenHash string) (*model.PersonalAccessToken, error) {
	row := s.db.QueryRowContext(ctx,
		"SELECT "+tokenColumns+" FROM personal_access_tokens WHERE token = ?", tokenHash)

	t := &model.PersonalAccessToken{}
	var abilities sql.NullString
	var lastUsedAt, expiresAt, createdAt, updatedAt sql.NullTime

	if err := row.Scan(
		&t.ID, &t.TokenableType, &t.TokenableID, &t.Name, &t.Token, &abilities,
		&lastUsedAt, &expiresAt, &createdAt, &updatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	t.Abilities = nullStringPtr(abilities)
	t.LastUsedAt = nullTimePtr(lastUsedAt)
	t.ExpiresAt = nullTimePtr(expiresAt)
	t.CreatedAt = nullTimePtr(createdAt)
	t.UpdatedAt = nullTimePtr(updatedAt)
	return t, nil
}

// RevokeByTokenHash deletes the current token, matching Sanctum logout
// behavior: a missing row is not an error (Laravel still returns success).
func (s *MySQLTokenStore) RevokeByTokenHash(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(ctx,
		"DELETE FROM personal_access_tokens WHERE token = ?", tokenHash)
	return err
}
