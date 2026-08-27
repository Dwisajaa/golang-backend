package model

import "time"

// OTP types mirror Laravel EmailVerificationOtp constants.
const (
	OtpTypeEmailVerification = "email_verification"
	OtpTypePasswordReset     = "password_reset"
)

// EmailVerificationOtp mirrors the email_verification_otps table. OtpHash is a
// bcrypt hash (Laravel stores Hash::make(otp)); the plaintext code never
// persists and must never leave the service layer.
type EmailVerificationOtp struct {
	ID        uint64
	UserID    uint64
	Type      string
	OtpHash   string
	ExpiresAt *time.Time
	UsedAt    *time.Time
	Attempts  uint8
	CreatedAt *time.Time
	UpdatedAt *time.Time
}
