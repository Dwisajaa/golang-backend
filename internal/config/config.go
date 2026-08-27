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

const (
	defaultAppPort = 8080
	defaultDBPort  = 3306
	defaultAppEnv  = "development"
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
