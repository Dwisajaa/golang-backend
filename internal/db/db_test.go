package db

import (
	"strings"
	"testing"

	"github.com/Dwisajaa/golang-backend/internal/config"
)

func testDatabaseConfig() config.DatabaseConfig {
	return config.DatabaseConfig{
		Host:     "127.0.0.1",
		Port:     3306,
		Name:     "apidw",
		User:     "apidw",
		Password: "sekret1",
	}
}

func TestBuildDSNContainsExpectedPieces(t *testing.T) {
	dsn, err := buildDSN(testDatabaseConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"tcp(127.0.0.1:3306)", "apidw", "parseTime=true", "charset=utf8mb4"} {
		if !strings.Contains(dsn, want) {
			t.Errorf("DSN missing %q, got %q", want, dsn)
		}
	}
	if !strings.Contains(dsn, "sekret1") {
		t.Errorf("DSN should contain the password for the driver, got %q", dsn)
	}
}

func TestBuildDSNNeverPanicsOnEmpty(t *testing.T) {
	// Empty config must still produce a valid DSN string (connection later
	// fails), never a panic.
	dsn, err := buildDSN(config.DatabaseConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dsn == "" {
		t.Fatal("expected non-empty DSN")
	}
}

// TestOpenSkippedWithoutInfrastructure is a documentation-only test: opening a
// real pool needs a MySQL instance. The requirement is stated here and real
// integration coverage lands with the repository tests (FASE 10) against a
// disposable MySQL test database.
func TestOpenRequiresRunningMySQL(t *testing.T) {
	t.Skip("integration requires a MySQL test instance; unit coverage = config + DSN")
}
