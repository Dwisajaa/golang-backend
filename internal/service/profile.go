package service

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Dwisajaa/golang-backend/internal/auth"
	"github.com/Dwisajaa/golang-backend/internal/httperr"
	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/repository"
)

// ProfileUserSource is the persistence ProfileService needs for reads.
type ProfileUserSource interface {
	FindByID(ctx context.Context, id uint64) (*model.User, error)
}

// profileUsers extends reads with the two update mutations (all Queryer-based
// so they run inside a service transaction).
type profileUsers interface {
	ProfileUserSource
	FindByEmail(ctx context.Context, email string) (*model.User, error)
	UpdateNameEmail(ctx context.Context, q repository.Queryer, id uint64, name, email string, emailVerifiedAt *time.Time) error
	UpdatePassword(ctx context.Context, q repository.Queryer, id uint64, passwordHash string) error
}

// otpDispatcher lets the profile service re-issue a verification OTP after an
// email change (Laravel ProfileController calls $user->sendOtp()).
type otpDispatcher interface {
	ResendVerificationOtp(ctx context.Context, email string) error
}

// ProfileService owns profile read/update/password rules. Handlers give it the
// authenticated user's id from the middleware context — never from the client.
type ProfileService struct {
	users  profileUsers
	tokens tokenRevo
	tx     txRunner
	hasher auth.PasswordHasher
	otp    otpDispatcher
}

type tokenRevo interface {
	RevokeAllForUser(ctx context.Context, q repository.Queryer, userID uint64) error
}

func NewProfileService(users profileUsers, tokens tokenRevo, tx txRunner, hasher auth.PasswordHasher, otp otpDispatcher) *ProfileService {
	return &ProfileService{users: users, tokens: tokens, tx: tx, hasher: hasher, otp: otp}
}

// UpdateProfileInput carries the Laravel UpdateProfileRequest fields.
type UpdateProfileInput struct {
	Name  string
	Email string
}

// UpdateProfile mirrors ProfileController@update:
//   - allows name + email only
//   - email must be unique except self
//   - changing email resets email_verified_at to NULL and re-issues a
//     verification OTP (queued async)
func (s *ProfileService) UpdateProfile(ctx context.Context, userID uint64, in UpdateProfileInput) (*model.User, error) {
	u, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return nil, s.mapUserErr(err)
	}

	emailChanged := u.Email != in.Email
	var verifiedAt *time.Time
	if emailChanged {
		// unique-ignore-self check (mirrors unique:users,email,ignore(id))
		if other, err := s.users.FindByEmail(ctx, in.Email); err == nil {
			if other.ID != userID {
				return nil, httperr.Validation(map[string][]string{
					"email": {"The email has already been taken."},
				})
			}
		} else if !errors.Is(err, repository.ErrNotFound) {
			return nil, httperr.Internal(err)
		}
	} else {
		verifiedAt = u.EmailVerifiedAt
	}

	err = s.tx.Within(ctx, func(tx *sql.Tx) error {
		return s.users.UpdateNameEmail(ctx, tx, userID, in.Name, in.Email, verifiedAt)
	})
	if err != nil {
		if errors.Is(err, repository.ErrDuplicateEmail) {
			return nil, httperr.Validation(map[string][]string{
				"email": {"The email has already been taken."},
			})
		}
		return nil, httperr.Internal(err)
	}

	u.Name = in.Name
	u.Email = in.Email
	if emailChanged {
		u.EmailVerifiedAt = nil
		// Side effect after commit: OTP queue (never inside a DB transaction).
		_ = s.otp.ResendVerificationOtp(ctx, in.Email)
	}
	return u, nil
}

// UpdatePassword mirrors ProfileController@updatePassword: current password is
// verified, new password hashed (bcrypt), all tokens revoked — atomically.
func (s *ProfileService) UpdatePassword(ctx context.Context, userID uint64, currentPassword, newPassword string) error {
	u, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return s.mapUserErr(err)
	}

	if err := s.hasher.Compare(u.Password, currentPassword); err != nil {
		return httperr.Validation(map[string][]string{
			"current_password": {"The current password field does not match your password."},
		})
	}

	newHash, err := s.hasher.Hash(newPassword)
	if err != nil {
		return httperr.Internal(err)
	}

	err = s.tx.Within(ctx, func(tx *sql.Tx) error {
		if err := s.users.UpdatePassword(ctx, tx, userID, newHash); err != nil {
			return err
		}
		return s.tokens.RevokeAllForUser(ctx, tx, userID)
	})
	if err != nil {
		return httperr.Internal(err)
	}
	return nil
}

func (s *ProfileService) mapUserErr(err error) error {
	if errors.Is(err, repository.ErrNotFound) {
		return httperr.NotFound("Resource not found.")
	}
	return httperr.Internal(err)
}
