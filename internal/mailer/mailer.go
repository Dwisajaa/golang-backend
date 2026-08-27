// Package mailer abstracts outbound email so services never touch SMTP.
// Handlers must not know about it; services depend on the Mailer interface.
package mailer

import (
	"context"
	"time"
)

// Message is a fully-formatted outbound email. Body may contain the OTP —
// callers must never log Message.Body (see Worker/LogMailer notes).
type Message struct {
	ToEmail string
	ToName  string
	Subject string
	Body    string
}

// Mailer is the seam every async/email path goes through.
type Mailer interface {
	Send(ctx context.Context, m Message) error
}

// VerificationEmail builds the subject/body for the email-verification OTP,
// mirroring Laravel EmailVerificationOtpMail's content ("Kode Verifikasi Email
// - Dwidev", expiry shown in Asia/Jakarta, "d F Y, H:i").
func VerificationEmail(name, otp string, expiresAt time.Time) Message {
	const layout = "02 January 2006, 15:04"
	jakarta := time.FixedZone("WIB", 7*3600) // Asia/Jakarta, no DST
	return Message{
		Subject: "Kode Verifikasi Email - Dwidev",
		Body:    "Halo " + name + ",\n\nKode verifikasi email Anda: " + otp + "\n\nKode berlaku hingga " + expiresAt.In(jakarta).Format(layout) + " WIB.\n\nDwidev",
	}
}
