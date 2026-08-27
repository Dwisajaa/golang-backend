package service

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Dwisajaa/golang-backend/internal/auth"
	"github.com/Dwisajaa/golang-backend/internal/config"
	"github.com/Dwisajaa/golang-backend/internal/httperr"
	"github.com/Dwisajaa/golang-backend/internal/mailer"
	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/repository"
)

// txRunner is the transaction boundary OtpService uses. db.TxManager satisfies
// it; tests substitute a fake that runs fn without a real database.
type txRunner interface {
	Within(ctx context.Context, fn func(tx *sql.Tx) error) error
}

// otpUsers is the user persistence OtpService needs.
type otpUsers interface {
	FindByEmail(ctx context.Context, email string) (*model.User, error)
	UpdateVerified(ctx context.Context, q repository.Queryer, id uint64, at time.Time) error
}

// otpTokens reuses the token persistence from auth.
type otpTokens interface {
	Create(ctx context.Context, t *model.PersonalAccessToken) error
}

// otpStore is the OTP persistence surface.
type otpStore interface {
	PruneAndInvalidate(ctx context.Context, q repository.Queryer, userID uint64, otpType string) error
	Create(ctx context.Context, q repository.Queryer, otp *model.EmailVerificationOtp) error
	FindActiveForUpdate(ctx context.Context, q repository.Queryer, userID uint64, otpType string, now time.Time) (*model.EmailVerificationOtp, error)
	IncrementAttempts(ctx context.Context, q repository.Queryer, id uint64) error
	MarkUsed(ctx context.Context, q repository.Queryer, id uint64, usedAt time.Time) error
}

// OtpService owns OTP + email-verification business rules and, crucially, the
// atomic single-use enforcement during verification.
type OtpService struct {
	otps     otpStore
	users    otpUsers
	tokens   otpTokens
	tx       txRunner
	hasher   auth.PasswordHasher
	otpGen   *auth.OtpGenerator
	tokenGen auth.TokenGenerator
	mailer   mailer.Mailer
	cfg      config.OtpConfig
}

func NewOtpService(
	otps otpStore,
	users otpUsers,
	tokens otpTokens,
	tx txRunner,
	hasher auth.PasswordHasher,
	otpGen *auth.OtpGenerator,
	tokenGen auth.TokenGenerator,
	mail mailer.Mailer,
	cfg config.OtpConfig,
) *OtpService {
	return &OtpService{
		otps: otps, users: users, tokens: tokens, tx: tx,
		hasher: hasher, otpGen: otpGen, tokenGen: tokenGen, mailer: mail,
		cfg: cfg,
	}
}

// ResendVerificationOtp mirrors Laravel resend(): unknown user and already
// verified user both silently succeed (anti user-enumeration). Otherwise a new
// OTP is created (previous active ones invalidated) and queued asynchronously.
func (s *OtpService) ResendVerificationOtp(ctx context.Context, email string) error {
	u, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil // generic success, no enumeration
		}
		return httperr.Internal(err)
	}
	if u.EmailVerifiedAt != nil {
		return nil
	}

	otp, raw, err := s.newOtp()
	if err != nil {
		return httperr.Internal(err)
	}
	otp.UserID = u.ID
	otp.Type = model.OtpTypeEmailVerification

	now := time.Now().UTC()
	expiresAt := now.Add(time.Duration(s.cfg.ExpirationMinutes) * time.Minute).UTC()
	otp.ExpiresAt = &expiresAt
	otp.CreatedAt = &now
	otp.UpdatedAt = &now

	if err := s.tx.Within(ctx, func(tx *sql.Tx) error {
		if err := s.otps.PruneAndInvalidate(ctx, tx, u.ID, model.OtpTypeEmailVerification); err != nil {
			return err
		}
		return s.otps.Create(ctx, tx, otp)
	}); err != nil {
		return httperr.Internal(err)
	}

	// Async: enqueue after the transaction commits (never hold a DB tx while
	// doing network I/O). A full/down mail queue must not fail the request.
	msg := mailer.VerificationEmail(u.Name, raw, expiresAt)
	msg.ToEmail = u.Email
	msg.ToName = u.Name
	_ = s.mailer.Send(ctx, msg)
	return nil
}

// VerifyEmail mirrors Laravel verifyOtp(): validates the latest active OTP,
// enforces attempts, and atomically marks it used + sets email_verified_at,
// then mints a token like the login flow.
func (s *OtpService) VerifyEmail(ctx context.Context, email, otp string) (*LoginResult, error) {
	u, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, httperr.Unprocessable("Kode verifikasi tidak valid atau tidak ditemukan.")
		}
		return nil, httperr.Internal(err)
	}
	if u.EmailVerifiedAt != nil {
		return nil, httperr.BadRequest("Email sudah diverifikasi.")
	}

	now := time.Now().UTC()

	// invalidOTP records a wrong-code attempt that still must be committed so
	// the attempt counter persists; the typed error is returned after commit.
	var invalidOTP bool
	err = s.tx.Within(ctx, func(tx *sql.Tx) error {
		// Row lock: two concurrent verify requests cannot both consume this OTP.
		rec, err := s.otps.FindActiveForUpdate(ctx, tx, u.ID, model.OtpTypeEmailVerification, now)
		if err != nil {
			return err // ErrNotFound -> "tidak valid / kedaluwarsa"
		}

		if int(rec.Attempts) >= s.cfg.MaxAttempts {
			return httperr.TooManyRequests("Terlalu banyak percobaan. Silakan kirim ulang OTP.")
		}

		if err := s.hasher.Compare(rec.OtpHash, otp); err != nil {
			if err := s.otps.IncrementAttempts(ctx, tx, rec.ID); err != nil {
				return err
			}
			invalidOTP = true
			return nil // commit the increment
		}

		if err := s.otps.MarkUsed(ctx, tx, rec.ID, now); err != nil {
			return err
		}
		return s.users.UpdateVerified(ctx, tx, u.ID, now)
	})

	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, httperr.Unprocessable("Kode verifikasi tidak valid atau sudah kedaluwarsa.")
		}
		if he := httperr.As(err); he != nil {
			return nil, he // TooManyRequests passes through unchanged
		}
		return nil, httperr.Internal(err)
	}
	if invalidOTP {
		return nil, httperr.Unprocessable("Kode verifikasi tidak valid.")
	}

	u.EmailVerifiedAt = &now

	raw, hash, err := s.tokenGen.Generate()
	if err != nil {
		return nil, httperr.Internal(err)
	}
	tNow := time.Now().UTC()
	expiresAt := tNow.Add(model.DefaultTokenLifetime).UTC()
	token := &model.PersonalAccessToken{
		TokenableType: model.TokenableType,
		TokenableID:   u.ID,
		Name:          "mobile-app",
		Token:         hash,
		ExpiresAt:     &expiresAt,
		CreatedAt:     &tNow,
		UpdatedAt:     &tNow,
	}
	if err := s.tokens.Create(ctx, token); err != nil {
		return nil, httperr.Internal(err)
	}
	return &LoginResult{User: u, RawToken: raw}, nil
}

// newOtp generates a 6-digit code and its bcrypt hash — Laravel stores
// Hash::make(otp), never the plaintext, and so do we.
func (s *OtpService) newOtp() (*model.EmailVerificationOtp, string, error) {
	raw, err := s.otpGen.Generate()
	if err != nil {
		return nil, "", err
	}
	hash, err := s.hasher.Hash(raw)
	if err != nil {
		return nil, "", err
	}
	return &model.EmailVerificationOtp{OtpHash: hash}, raw, nil
}
