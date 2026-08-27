package config

import (
	"strings"
	"testing"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func validEnv() map[string]string {
	return map[string]string{
		"APP_ENV":           "development",
		"APP_PORT":          "8080",
		"DATABASE_HOST":     "127.0.0.1",
		"DATABASE_PORT":     "3306",
		"DATABASE_NAME":     "apidw",
		"DATABASE_USER":     "apidw",
		"DATABASE_PASSWORD": "sekret1",
	}
}

func TestLoadValid(t *testing.T) {
	cfg, err := Load(env(validEnv()))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.App.Port != 8080 || cfg.Database.Host != "127.0.0.1" || cfg.Database.Password != "sekret1" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestLoadDefaults(t *testing.T) {
	m := validEnv()
	delete(m, "APP_PORT")
	delete(m, "DATABASE_PORT")
	delete(m, "APP_ENV")
	cfg, err := Load(env(m))
	if err != nil {
		t.Fatalf("expected no error with defaults, got %v", err)
	}
	if cfg.App.Port != defaultAppPort || cfg.App.Env != "development" || cfg.Database.Port != defaultDBPort {
		t.Fatalf("defaults not applied: %+v", cfg)
	}
}

func TestLoadMissingRequired(t *testing.T) {
	m := validEnv()
	delete(m, "DATABASE_NAME")
	_, err := Load(env(m))
	if err == nil {
		t.Fatal("expected error for missing DATABASE_NAME")
	}
	if !strings.Contains(err.Error(), "DATABASE_NAME") {
		t.Fatalf("error should name the missing variable, got %q", err.Error())
	}
	if strings.Contains(err.Error(), "sekret1") {
		t.Fatal("error message leaked the database password")
	}
}

func TestLoadInvalidPorts(t *testing.T) {
	for _, name := range []string{"APP_PORT", "DATABASE_PORT"} {
		m := validEnv()
		m[name] = "abc"
		if _, err := Load(env(m)); err == nil {
			t.Fatalf("expected error for invalid %s", name)
		}
		m[name] = "0"
		if _, err := Load(env(m)); err == nil {
			t.Fatalf("expected error for %s=0", name)
		}
		m[name] = "70000"
		if _, err := Load(env(m)); err == nil {
			t.Fatalf("expected error for %s=70000", name)
		}
	}
}

func TestErrorDoesNotLeakPassword(t *testing.T) {
	m := validEnv()
	delete(m, "DATABASE_HOST")
	_, err := Load(env(m))
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "sekret1") {
		t.Fatal("config error leaked the database password")
	}
}
