package repository

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/Dwisajaa/golang-backend/internal/model"
)

// TestMySQLOtpStore_VerifyFlow exercises the OTP lifecycle against a real
// MySQL: create -> find FOR UPDATE inside a tx -> mark used + verify user ->
// the used OTP is then not selectable again (single-use).
// Gated by TEST_DATABASE_URL; never touches production data.
func TestMySQLOtpStore_VerifyFlow(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; OTP repository integration requires a disposable MySQL test database")
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	ctx := context.Background()

	ensureTables(t, db, ctx)
	otpStore := NewMySQLOtpStore()
	userStore := NewMySQLUserRepository(db)

	user := &model.User{Name: "Otp Test", Email: "otp-test@example.test", Password: "h", Role: model.RoleCustomer}
	if err := userStore.Create(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}

	now := time.Now().UTC()
	exp := now.Add(10 * time.Minute).UTC()
	otp := &model.EmailVerificationOtp{
		UserID: user.ID, Type: model.OtpTypeEmailVerification,
		OtpHash: "some-hash", ExpiresAt: &exp, CreatedAt: &now, UpdatedAt: &now,
	}
	if err := otpStore.Create(ctx, db, otp); err != nil {
		t.Fatalf("create otp: %v", err)
	}

	// Real transactional + FOR UPDATE path identical to OtpService.VerifyEmail.
	err = within(ctx, db, func(tx *sql.Tx) error {
		rec, err := otpStore.FindActiveForUpdate(ctx, tx, user.ID, model.OtpTypeEmailVerification, now)
		if err != nil {
			return err
		}
		if rec.ID != otp.ID {
			t.Fatalf("expected otp %d, got %d", otp.ID, rec.ID)
		}
		if _, err := db.ExecContext(ctx, `UPDATE email_verification_otps SET used_at = ? WHERE id = ?`, now, rec.ID); err != nil {
			return err
		}
		return userStore.UpdateVerified(ctx, tx, user.ID, now)
	})
	if err != nil {
		t.Fatalf("tx flow: %v", err)
	}

	// The used OTP must no longer be selectable as active (single-use parity).
	if _, err := otpStore.FindActiveForUpdate(ctx, db, user.ID, model.OtpTypeEmailVerification, now); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after use, got %v", err)
	}

	// Attempts increment works.
	otp2 := &model.EmailVerificationOtp{
		UserID: user.ID, Type: model.OtpTypeEmailVerification,
		OtpHash: "h2", ExpiresAt: &exp, CreatedAt: &now, UpdatedAt: &now,
	}
	if err := otpStore.Create(ctx, db, otp2); err != nil {
		t.Fatalf("create otp2: %v", err)
	}
	if err := otpStore.IncrementAttempts(ctx, db, otp2.ID); err != nil {
		t.Fatalf("increment: %v", err)
	}
	rec2, err := otpStore.FindActiveForUpdate(ctx, db, user.ID, model.OtpTypeEmailVerification, now)
	if err != nil {
		t.Fatalf("find otp2: %v", err)
	}
	if rec2.Attempts != 1 {
		t.Fatalf("expected attempts 1, got %d", rec2.Attempts)
	}

	// PruneAndInvalidate marks active of the type as used (resend parity).
	if err := otpStore.PruneAndInvalidate(ctx, db, user.ID, model.OtpTypeEmailVerification); err != nil {
		t.Fatalf("prune: %v", err)
	}
	if _, err := otpStore.FindActiveForUpdate(ctx, db, user.ID, model.OtpTypeEmailVerification, now); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after prune, got %v", err)
	}
}

func within(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
