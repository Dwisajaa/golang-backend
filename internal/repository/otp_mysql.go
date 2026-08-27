package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Dwisajaa/golang-backend/internal/model"
)

const otpColumns = "id, user_id, type, otp_hash, expires_at, used_at, attempts, created_at, updated_at"

// MySQLOtpStore implements OtpStore. All methods accept a Queryer so the
// caller controls transaction membership.
type MySQLOtpStore struct{}

func NewMySQLOtpStore() *MySQLOtpStore { return &MySQLOtpStore{} }

func (s *MySQLOtpStore) PruneAndInvalidate(ctx context.Context, q Queryer, userID uint64, otpType string) error {
	if _, err := q.ExecContext(ctx,
		`DELETE FROM email_verification_otps
		 WHERE user_id = ? AND used_at IS NULL AND expires_at <= ?`,
		userID, time.Now().UTC()); err != nil {
		return err
	}
	_, err := q.ExecContext(ctx,
		`UPDATE email_verification_otps SET used_at = ?
		 WHERE user_id = ? AND type = ? AND used_at IS NULL`,
		time.Now().UTC(), userID, otpType)
	return err
}

func (s *MySQLOtpStore) Create(ctx context.Context, q Queryer, otp *model.EmailVerificationOtp) error {
	res, err := q.ExecContext(ctx,
		`INSERT INTO email_verification_otps
		   (user_id, type, otp_hash, expires_at, used_at, attempts, created_at, updated_at)
		 VALUES (?, ?, ?, ?, NULL, 0, ?, ?)`,
		otp.UserID, otp.Type, otp.OtpHash, otp.ExpiresAt,
		otp.CreatedAt, otp.UpdatedAt)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	otp.ID = uint64(id)
	otp.Attempts = 0
	return nil
}

func (s *MySQLOtpStore) FindActiveForUpdate(ctx context.Context, q Queryer, userID uint64, otpType string, now time.Time) (*model.EmailVerificationOtp, error) {
	row := q.QueryRowContext(ctx,
		"SELECT "+otpColumns+` FROM email_verification_otps
		 WHERE user_id = ? AND type = ? AND used_at IS NULL AND expires_at > ?
		 ORDER BY id DESC LIMIT 1 FOR UPDATE`,
		userID, otpType, now)

	otp := &model.EmailVerificationOtp{}
	var expiresAt, usedAt, createdAt, updatedAt sql.NullTime
	if err := row.Scan(
		&otp.ID, &otp.UserID, &otp.Type, &otp.OtpHash,
		&expiresAt, &usedAt, &otp.Attempts, &createdAt, &updatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	otp.ExpiresAt = nullTimePtr(expiresAt)
	otp.UsedAt = nullTimePtr(usedAt)
	otp.CreatedAt = nullTimePtr(createdAt)
	otp.UpdatedAt = nullTimePtr(updatedAt)
	return otp, nil
}

func (s *MySQLOtpStore) IncrementAttempts(ctx context.Context, q Queryer, id uint64) error {
	_, err := q.ExecContext(ctx,
		"UPDATE email_verification_otps SET attempts = attempts + 1 WHERE id = ?", id)
	return err
}

func (s *MySQLOtpStore) MarkUsed(ctx context.Context, q Queryer, id uint64, usedAt time.Time) error {
	_, err := q.ExecContext(ctx,
		"UPDATE email_verification_otps SET used_at = ? WHERE id = ?", usedAt, id)
	return err
}
