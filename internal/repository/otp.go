package repository

import (
	"context"
	"time"

	"github.com/Dwisajaa/golang-backend/internal/model"
)

// OtpStore is OTP persistence. Methods take a Queryer so they can run either
// on the pool or inside a service transaction; the transaction + FOR UPDATE
// for single-use is orchestrated by the OTP service.
type OtpStore interface {
	// PruneAndInvalidate deletes expired unused OTPs and marks all previously
	// active OTPs of the given type as used (Laravel User::sendOtp).
	PruneAndInvalidate(ctx context.Context, q Queryer, userID uint64, otpType string) error
	Create(ctx context.Context, q Queryer, otp *model.EmailVerificationOtp) error
	// FindActiveForUpdate selects the newest unchanged active OTP with
	// FOR UPDATE, so concurrent verify requests cannot reuse the same code.
	FindActiveForUpdate(ctx context.Context, q Queryer, userID uint64, otpType string, now time.Time) (*model.EmailVerificationOtp, error)
	IncrementAttempts(ctx context.Context, q Queryer, id uint64) error
	MarkUsed(ctx context.Context, q Queryer, id uint64, usedAt time.Time) error
}
