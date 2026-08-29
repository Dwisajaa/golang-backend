package repository

import (
	"context"
	"database/sql"
	"errors"
)

// CatalogLookup provides active-service/package lookup with FOR UPDATE for
// the booking creation flow (price snapshot + lock).
type CatalogLookup struct{}

func NewCatalogLookup() *CatalogLookup { return &CatalogLookup{} }

func (c *CatalogLookup) FindActiveServiceForUpdate(ctx context.Context, q Queryer, id uint64) (string, int64, error) {
	var name, price string
	err := q.QueryRowContext(ctx,
		"SELECT name, price FROM services WHERE id = ? AND is_active = 1 FOR UPDATE", id).
		Scan(&name, &price)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", 0, ErrNotFound
		}
		return "", 0, err
	}
	cents, err := parsePriceString(price)
	return name, cents, err
}

func (c *CatalogLookup) FindActivePackageForUpdate(ctx context.Context, q Queryer, id uint64) (string, int64, error) {
	var name, price string
	err := q.QueryRowContext(ctx,
		"SELECT name, price FROM packages WHERE id = ? AND is_active = 1 FOR UPDATE", id).
		Scan(&name, &price)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", 0, ErrNotFound
		}
		return "", 0, err
	}
	cents, err := parsePriceString(price)
	return name, cents, err
}

// ProfileLookup checks customer profile completeness for booking validation.
type ProfileLookup struct {
	DB *sql.DB
}

func NewProfileLookup(db *sql.DB) *ProfileLookup { return &ProfileLookup{DB: db} }

func (p *ProfileLookup) IsProfileComplete(ctx context.Context, userID uint64) (bool, error) {
	var fullName, phone, address, city string
	err := p.DB.QueryRowContext(ctx,
		"SELECT full_name, phone, address, city FROM customer_profiles WHERE user_id = ?", userID).
		Scan(&fullName, &phone, &address, &city)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return fullName != "" && phone != "" && address != "" && city != "", nil
}
