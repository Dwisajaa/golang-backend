package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Dwisajaa/golang-backend/internal/model"
)

const customerProfileColumns = "id, user_id, full_name, phone, address, city, postal_code, created_at, updated_at"

// MySQLCustomerProfileStore implements CustomerProfileStore.
type MySQLCustomerProfileStore struct{}

func NewMySQLCustomerProfileStore() *MySQLCustomerProfileStore { return &MySQLCustomerProfileStore{} }

func (s *MySQLCustomerProfileStore) FindByUserID(ctx context.Context, q Queryer, userID uint64) (*model.CustomerProfile, error) {
	row := q.QueryRowContext(ctx,
		"SELECT "+customerProfileColumns+" FROM customer_profiles WHERE user_id = ?", userID)

	p := &model.CustomerProfile{}
	var postalCode sql.NullString
	var createdAt, updatedAt sql.NullTime
	if err := row.Scan(
		&p.ID, &p.UserID, &p.FullName, &p.Phone, &p.Address, &p.City,
		&postalCode, &createdAt, &updatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	p.PostalCode = nullStringPtr(postalCode)
	p.CreatedAt = nullTimePtr(createdAt)
	p.UpdatedAt = nullTimePtr(updatedAt)
	return p, nil
}

// Upsert mirrors Eloquent updateOrCreate. The unique(user_id) constraint makes
// collision handling atomic: the same statement inserts or updates, so two
// concurrent first-time requests cannot create duplicate profiles.
func (s *MySQLCustomerProfileStore) Upsert(ctx context.Context, q Queryer, p *model.CustomerProfile) error {
	_, err := q.ExecContext(ctx,
		`INSERT INTO customer_profiles
		   (user_id, full_name, phone, address, city, postal_code, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE
		   full_name = VALUES(full_name),
		   phone = VALUES(phone),
		   address = VALUES(address),
		   city = VALUES(city),
		   postal_code = VALUES(postal_code),
		   updated_at = VALUES(updated_at)`,
		p.UserID, p.FullName, p.Phone, p.Address, p.City, p.PostalCode,
		p.CreatedAt, p.UpdatedAt)
	return err
}
