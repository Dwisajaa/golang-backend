package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/Dwisajaa/golang-backend/internal/httperr"
	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/repository"
)

// techProfiles is the persistence slice the technician profile service uses.
type techProfiles interface {
	FindByUserID(ctx context.Context, q repository.Queryer, userID uint64) (*model.TechnicianProfile, error)
	InsertProfile(ctx context.Context, q repository.Queryer, p *model.TechnicianProfile) error
	UpdateProfile(ctx context.Context, q repository.Queryer, userID uint64, p *model.TechnicianProfile) error
}

// TechnicianProfileService owns read/firstOrCreate/update rules. Identity is
// the authenticated user id from context; nothing client-controlled.
type TechnicianProfileService struct {
	profiles techProfiles
	tx       txRunner
}

func NewTechnicianProfileService(profiles techProfiles, tx txRunner) *TechnicianProfileService {
	return &TechnicianProfileService{profiles: profiles, tx: tx}
}

// TechnicianProfileInput mirrors UpdateTechnicianProfileRequest (all nullable).
type TechnicianProfileInput struct {
	Phone          string
	Specialization string
	Address        string
	Bio            string
}

// GetByUserID mirrors TechnicianController@profile: a missing profile is a
// 404 "Technician profile not found." (unlike customer profile's data:null).
func (s *TechnicianProfileService) GetByUserID(ctx context.Context, userID uint64) (*model.TechnicianProfile, error) {
	var out *model.TechnicianProfile
	var notFound bool
	err := s.tx.Within(ctx, func(tx *sql.Tx) error {
		p, err := s.profiles.FindByUserID(ctx, tx, userID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				notFound = true
				return nil
			}
			return err
		}
		out = p
		return nil
	})
	if err != nil {
		return nil, httperr.Internal(err)
	}
	if notFound {
		return nil, httperr.NotFound("Technician profile not found.")
	}
	return out, nil
}

// UpdateProfile mirrors TechnicianController@updateProfile: firstOrCreate by
// user_id then update. First write generates TECH-XXXX (retried on collision,
// mirroring Laravel's generate-while-exists); a concurrent create for the same
// user falls back to the update path via the user_id unique constraint.
func (s *TechnicianProfileService) UpdateProfile(ctx context.Context, userID uint64, in TechnicianProfileInput) (*model.TechnicianProfile, error) {
	now := time.Now().UTC()
	editable := profileFields(in, &now)

	var out *model.TechnicianProfile
	err := s.tx.Within(ctx, func(tx *sql.Tx) error {
		p, err := s.profiles.FindByUserID(ctx, tx, userID)
		if err == nil {
			if err := s.profiles.UpdateProfile(ctx, tx, userID, profileFields(in, &now)); err != nil {
				return err
			}
			p.Phone = editable.Phone
			p.Specialization = editable.Specialization
			p.Address = editable.Address
			p.Bio = editable.Bio
			out = p
			return nil
		}
		if !errors.Is(err, repository.ErrNotFound) {
			return err
		}

		// create path: bounded retries for code collisions
		for attempt := 0; attempt < maxCodeAttempts; attempt++ {
			code, err := generateTechnicianCode()
			if err != nil {
				return err
			}
			np := &model.TechnicianProfile{
				UserID:         userID,
				TechnicianCode: code,
				Phone:          editable.Phone,
				Specialization: editable.Specialization,
				Address:        editable.Address,
				Bio:            editable.Bio,
				IsActive:       true,
				CreatedAt:      &now,
				UpdatedAt:      &now,
			}
			err = s.profiles.InsertProfile(ctx, tx, np)
			switch {
			case err == nil:
				out = np
				return nil
			case errors.Is(err, repository.ErrDuplicateTechnicianCode):
				continue // regenerate like Laravel's do/while
			case errors.Is(err, repository.ErrDuplicate):
				// concurrent create for the same user_id → update instead
				ex, ferr := s.profiles.FindByUserID(ctx, tx, userID)
				if ferr != nil {
					return ferr
				}
				if err := s.profiles.UpdateProfile(ctx, tx, userID, profileFields(in, &now)); err != nil {
					return err
				}
				ex.Phone = editable.Phone
				ex.Specialization = editable.Specialization
				ex.Address = editable.Address
				ex.Bio = editable.Bio
				out = ex
				return nil
			default:
				return err
			}
		}
		return errors.New("generate technician code: too many collisions")
	})
	if err != nil {
		return nil, httperr.Internal(err)
	}
	return out, nil
}

const maxCodeAttempts = 5

// profileFields normalizes empty strings to NULL (nullable rule parity).
func profileFields(in TechnicianProfileInput, updatedAt *time.Time) *model.TechnicianProfile {
	return &model.TechnicianProfile{
		Phone:          nullIfEmpty(in.Phone),
		Specialization: nullIfEmpty(in.Specialization),
		Address:        nullIfEmpty(in.Address),
		Bio:            nullIfEmpty(in.Bio),
		UpdatedAt:      updatedAt,
	}
}

// nullIfEmpty stores NULL for absent optional fields (Laravel nullable).
func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// generateTechnicianCode mirrors Laravel generateTechnicianCode: "TECH-XXXX"
// with random_int(1,9999) left-padded to 4 digits, from crypto/rand.
func generateTechnicianCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(9999))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%04d", model.TechnicianCodePrefix, n.Int64()+1), nil
}
