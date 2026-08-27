package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Migrator applies forward-only SQL files from a directory in filename order.
// Each applied file is recorded in schema_migrations so it never runs twice.
//
// MySQL DDL causes implicit commits, so migration files are NOT transactional:
// a failure mid-file leaves that file partially applied. The runner stops and
// reports the file + statement index; a human resolves before re-running.
type Migrator struct {
	DB  *sql.DB
	Dir string
}

func (m *Migrator) Migrate(ctx context.Context) ([]string, error) {
	if err := m.ensureTable(ctx); err != nil {
		return nil, err
	}

	files, err := os.ReadDir(m.Dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}

	applied := make(map[string]bool)
	rows, err := m.DB.QueryContext(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return nil, fmt.Errorf("query applied migrations: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		applied[v] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var ran []string
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".sql") {
			continue
		}
		if applied[f.Name()] {
			continue
		}
		if err := m.applyFile(ctx, f.Name()); err != nil {
			return ran, err
		}
		ran = append(ran, f.Name())
	}
	return ran, nil
}

func (m *Migrator) ensureTable(ctx context.Context) error {
	_, err := m.DB.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version VARCHAR(255) NOT NULL PRIMARY KEY,
		applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`)
	return err
}

func (m *Migrator) applyFile(ctx context.Context, name string) error {
	path := filepath.Join(m.Dir, name)
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", name, err)
	}

	stmts := splitStatements(string(content))
	for i, stmt := range stmts {
		if _, err := m.DB.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("apply %s statement %d: %w", name, i+1, err)
		}
	}

	_, err = m.DB.ExecContext(ctx,
		"INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)",
		name, time.Now().UTC().Truncate(time.Second))
	if err != nil {
		return fmt.Errorf("record %s: %w", name, err)
	}
	return nil
}

// splitStatements splits a migration file on statement-terminating semicolons.
// Migration SQL in this repo contains no semicolons inside literals, so a plain
// split is sufficient and keeps the driver's multiStatements disabled.
func splitStatements(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ";") {
		stmt := strings.TrimSpace(part)
		if stmt != "" {
			out = append(out, stmt)
		}
	}
	return out
}
