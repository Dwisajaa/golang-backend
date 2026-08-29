package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds every value the application needs at startup. It is a plain
// value struct built once in the composition root and passed down; there is no
// global configuration object.
type Config struct {
	App      AppConfig
	Database DatabaseConfig
	Mail     MailConfig
	Otp      OtpConfig
	Storage  StorageConfig
}

type AppConfig struct {
	Env  string // development | production | testing
	Port int
}

type DatabaseConfig struct {
	Host     string
	Port     int
	Name     string
	User     string
	Password string
}

// MailConfig drives the SMTP mailer. When SMTPHost is empty the app falls back
// to a log mailer (Laravel MAIL_MAILER=log parity for local dev).
type MailConfig struct {
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	FromAddress  string
	FromName     string
}

// OtpConfig mirrors Laravel config('auth.otp_expiration')/('otp_max_attempts').
type OtpConfig struct {
	ExpirationMinutes int
	MaxAttempts       int
}

// StorageConfig locates private file storage (payment proofs).
type StorageConfig struct {
	PaymentProofsPath string
}

const (
	defaultAppPort     = 8080
	defaultDBPort      = 3306
	defaultAppEnv      = "development"
	defaultSMTPPort    = 587
	defaultOtpExpiry   = 10
	defaultOtpAttempts = 5
	defaultProofPath   = "storage/app/private/payment-proofs"
)

// Load reads environment variables and returns a validated Config. Required
// values that are missing or invalid produce an error that names the missing
// setting but never contains the database password.
func Load(getenv func(string) string) (*Config, error) {
	appPort, err := envInt(getenv("APP_PORT"), defaultAppPort, "APP_PORT")
	if err != nil {
		return nil, err
	}
	dbPort, err := envInt(getenv("DATABASE_PORT"), defaultDBPort, "DATABASE_PORT")
	if err != nil {
		return nil, err
	}

	smtpPort, err := envInt(getenv("SMTP_PORT"), defaultSMTPPort, "SMTP_PORT")
	if err != nil {
		return nil, err
	}
	otpExpiry, err := envInt(getenv("OTP_EXPIRATION_MINUTES"), defaultOtpExpiry, "OTP_EXPIRATION_MINUTES")
	if err != nil {
		return nil, err
	}
	otpAttempts, err := envInt(getenv("OTP_MAX_ATTEMPTS"), defaultOtpAttempts, "OTP_MAX_ATTEMPTS")
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		App: AppConfig{
			Env:  firstNonEmpty(getenv("APP_ENV"), defaultAppEnv),
			Port: appPort,
		},
		Database: DatabaseConfig{
			Host:     getenv("DATABASE_HOST"),
			Port:     dbPort,
			Name:     getenv("DATABASE_NAME"),
			User:     getenv("DATABASE_USER"),
			Password: getenv("DATABASE_PASSWORD"),
		},
		Mail: MailConfig{
			SMTPHost:     getenv("SMTP_HOST"),
			SMTPPort:     smtpPort,
			SMTPUsername: getenv("SMTP_USERNAME"),
			SMTPPassword: getenv("SMTP_PASSWORD"),
			FromAddress:  getenv("SMTP_FROM_ADDRESS"),
			FromName:     firstNonEmpty(getenv("SMTP_FROM_NAME"), "api-dwidev"),
		},
		Otp: OtpConfig{
			ExpirationMinutes: otpExpiry,
			MaxAttempts:       otpAttempts,
		},
		Storage: StorageConfig{
			PaymentProofsPath: firstNonEmpty(getenv("STORAGE_PAYMENT_PROOFS_PATH"), defaultProofPath),
		},
	}

	missing := []string{}
	if cfg.Database.Host == "" {
		missing = append(missing, "DATABASE_HOST")
	}
	if cfg.Database.Name == "" {
		missing = append(missing, "DATABASE_NAME")
	}
	if cfg.Database.User == "" {
		missing = append(missing, "DATABASE_USER")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}
	return cfg, nil
}

// envInt parses an env value as a port. An empty value falls back to def.
func envInt(value string, def int, name string) (int, error) {
	if value == "" {
		return def, nil
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 1 || n > 65535 {
		return 0, fmt.Errorf("%s must be a valid port (1-65535), got %q", name, value)
	}
	return n, nil
}

func firstNonEmpty(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

// osGetenv is the production getenv function.
func osGetenv(key string) string { return os.Getenv(key) }

// LoadFromOS loads configuration from the real process environment.
func LoadFromOS() (*Config, error) {
	return Load(osGetenv)
}
