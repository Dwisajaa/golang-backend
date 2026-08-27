package service

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Dwisajaa/golang-backend/internal/httperr"
	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/repository"
)

// customerProfiles is the persistence slice the customer profile service uses.
type customerProfiles interface {
	FindByUserID(ctx context.Context, q repository.Queryer, userID uint64) (*model.CustomerProfile, error)
	Upsert(ctx context.Context, q repository.Queryer, p *model.CustomerProfile) error
}

// CustomerProfileService owns the read/upsert rules for a customer's profile.
// Identity is the authenticated user id passed by the handler (never client
// input); ownership is implied by the unique user_id row.
type CustomerProfileService struct {
	profiles customerProfiles
	tx       txRunner
}

func NewCustomerProfileService(profiles customerProfiles, tx txRunner) *CustomerProfileService {
	return &CustomerProfileService{profiles: profiles, tx: tx}
}

// GetByUserID mirrors CustomerProfileController@show: a missing profile is
// returned as nil (handler serializes {"data":null}) â€” NOT an error.
func (s *CustomerProfileService) GetByUserID(ctx context.Context, userID uint64) (*model.CustomerProfile, error) {
	var out *model.CustomerProfile
	err := s.tx.Within(ctx, func(tx *sql.Tx) error {
		p, err := s.profiles.FindByUserID(ctx, tx, userID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return nil // data:null, Laravel parity
			}
			return err
		}
		out = p
		return nil
	})
	if err != nil {
		return nil, httperr.Internal(err)
	}
	return out, nil
}

// UpdateInput mirrors UpdateCustomerProfileRequest validated fields.
type UpdateInput struct {
	FullName   string
	Phone      string
	Address    string
	City       string
	PostalCode string // empty => stored as NULL
}

// Upsert mirrors CustomerProfileController@update (updateOrCreate): creates on
// first write, updates on subsequent ones, atomically via the unique user_id.
func (s *CustomerProfileService) Upsert(ctx context.Context, userID uint64, in UpdateInput) (*model.CustomerProfile, error) {
	var postalCode *string
	if in.PostalCode != "" {
		v := in.PostalCode
		postalCode = &v
	}
	now := time.Now().UTC()
	p := &model.CustomerProfile{
		UserID:     userID,
		FullName:   in.FullName,
		Phone:      in.Phone,
		Address:    in.Address,
		City:       in.City,
		PostalCode: postalCode,
		CreatedAt:  &now,
		UpdatedAt:  &now,
	}

	err := s.tx.Within(ctx, func(tx *sql.Tx) error {
		if err := s.profiles.Upsert(ctx, tx, p); err != nil {
			return err
		}
		// Backfill the row id (INSERT ON DUPLICATE does not return it reliably).
		got, err := s.profiles.FindByUserID(ctx, tx, userID)
		if err != nil {
			return err
		}
		p.ID = got.ID
		return nil
	})
	if err != nil {
		return nil, httperr.Internal(err)
	}
	return p, nil
}
