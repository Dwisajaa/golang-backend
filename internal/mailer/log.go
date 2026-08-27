package mailer

import (
	"context"
	"log/slog"
)

// LogMailer writes the email to the log instead of sending — the Laravel
// MAIL_MAILER=log parity for local development when SMTP is unconfigured.
// It intentionally logs the full body (which contains the OTP) because that is
// exactly what Laravel's log mailer does locally; it is never wired in tests.
type LogMailer struct {
	logger *slog.Logger
}

func NewLogMailer(logger *slog.Logger) *LogMailer { return &LogMailer{logger: logger} }

func (m *LogMailer) Send(ctx context.Context, msg Message) error {
	m.logger.Info("mail_sent",
		"to", msg.ToEmail,
		"subject", msg.Subject,
		"body", msg.Body,
	)
	return nil
}
