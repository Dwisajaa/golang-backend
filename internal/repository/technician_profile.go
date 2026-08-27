package repository

import (
	"context"

	"github.com/Dwisajaa/golang-backend/internal/model"
)

// TechnicianProfileStore is the persistence surface for technician profiles.
// Methods take Queryer so the service controls transaction membership.
type TechnicianProfileStore interface {
	FindByUserID(ctx context.Context, q Queryer, userID uint64) (*model.TechnicianProfile, error)
	// InsertProfile inserts a new profile, translating unique violations:
	//   - collision on technician_code   -> ErrDuplicateTechnicianCode
	//   - any other unique violation     -> ErrDuplicate (e.g. concurrent user_id)
	InsertProfile(ctx context.Context, q Queryer, p *model.TechnicianProfile) error
	// UpdateProfile refreshes the nullable editable fields (phone,
	// specialization, address, bio) — never technician_code or is_active,
	// matching TechnicianController@updateProfile($validated).
	UpdateProfile(ctx context.Context, q Queryer, userID uint64, p *model.TechnicianProfile) error
}
