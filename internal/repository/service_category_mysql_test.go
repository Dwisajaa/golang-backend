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

const catDDL = `CREATE TABLE IF NOT EXISTS service_categories (
	id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
	name VARCHAR(255) NOT NULL,
	slug VARCHAR(255) NOT NULL,
	description TEXT NULL,
	icon VARCHAR(255) NULL,
	is_active TINYINT(1) NOT NULL DEFAULT 1,
	created_at TIMESTAMP NULL,
	updated_at TIMESTAMP NULL,
	PRIMARY KEY (id),
	UNIQUE KEY service_categories_slug_unique (slug)
) ENGINE=InnoDB`

const serviceDDL = `CREATE TABLE IF NOT EXISTS services (
	id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
	service_category_id BIGINT UNSIGNED NOT NULL,
	name VARCHAR(255) NOT NULL,
	slug VARCHAR(255) NOT NULL,
	description TEXT NULL,
	price DECIMAL(12,2) NOT NULL DEFAULT 0.00,
	unit VARCHAR(255) NOT NULL DEFAULT 'per_service',
	estimated_duration INT NULL,
	is_active TINYINT(1) NOT NULL DEFAULT 1,
	created_at TIMESTAMP NULL,
	updated_at TIMESTAMP NULL,
	PRIMARY KEY (id),
	UNIQUE KEY services_slug_unique (slug)
) ENGINE=InnoDB`

func ensureCatTables(t *testing.T, db *sql.DB, ctx context.Context) {
	t.Helper()
	for _, ddl := range []string{catDDL, serviceDDL} {
		if _, err := db.ExecContext(ctx, ddl); err != nil {
			t.Fatalf("ensure: %v", err)
		}
	}
}

// TestMySQLServiceCategory_Coverage verifies CRUD, unique classification, the
// delete guard, and the active-only list (with nested active services) against
// real MySQL. Gated by TEST_DATABASE_URL.
func TestMySQLServiceCategory_Coverage(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; repository integration requires a disposable MySQL test database")
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	ctx := context.Background()
	ensureCatTables(t, db, ctx)

	store := NewMySQLServiceCategoryStore()
	now := time.Now().UTC()

	// create + find
	c := &model.ServiceCategory{Name: "AC", Slug: "ac", IsActive: true, CreatedAt: &now, UpdatedAt: &now}
	if err := store.Create(ctx, db, c); err != nil {
		t.Fatalf("create: %v", err)
	}
	if c.ID == 0 {
		t.Fatal("create must backfill id")
	}
	got, err := store.FindByID(ctx, db, c.ID)
	if err != nil || got.Name != "AC" {
		t.Fatalf("find: %+v err=%v", got, err)
	}

	// unique slug classification
	dup := &model.ServiceCategory{Name: "Other", Slug: "ac", IsActive: true, CreatedAt: &now, UpdatedAt: &now}
	if err := store.Create(ctx, db, dup); !errors.Is(err, ErrDuplicateSlug) {
		t.Fatalf("expected ErrDuplicateSlug, got %v", err)
	}

	// duplicate name classification
	dupn := &model.ServiceCategory{Name: "AC", Slug: "ac-2", IsActive: true, CreatedAt: &now, UpdatedAt: &now}
	if err := store.Create(ctx, db, dupn); !errors.Is(err, ErrDuplicateName) {
		t.Fatalf("expected ErrDuplicateName, got %v", err)
	}

	// name/slug taken helpers (ignore-self)
	if v, _ := store.NameTaken(ctx, db, "AC", 0); !v {
		t.Fatal("NameTaken should find AC")
	}
	if v, _ := store.NameTaken(ctx, db, "AC", c.ID); v {
		t.Fatal("NameTaken must ignore self")
	}

	// has-services guard
	if has, _ := store.HasServices(ctx, db, c.ID); has {
		t.Fatal("no services yet")
	}
	// seed one active service
	if _, err := db.ExecContext(ctx,
		"INSERT INTO services (service_category_id, name, slug, price, unit, is_active, created_at, updated_at) VALUES (?,?,?,150.00,'per_service',1,?,?)",
		c.ID, "AC Service", "ac-service", now, now); err != nil {
		t.Fatalf("seed service: %v", err)
	}
	if has, _ := store.HasServices(ctx, db, c.ID); !has {
		t.Fatal("HasServices should be true after seeding")
	}

	// active-only list with nested service projection
	total, err := store.CountActive(ctx, db)
	if err != nil || total != 1 {
		t.Fatalf("CountActive: %d err=%v", total, err)
	}
	items, err := store.ListActive(ctx, db, 15, 0)
	if err != nil || len(items) != 1 || len(items[0].Services) != 1 {
		t.Fatalf("ListActive: %+v err=%v", items, err)
	}
	svc0 := items[0].Services[0]
	if svc0.Name != "AC Service" || svc0.PriceCents != 15000 || svc0.Unit != "per_service" {
		t.Fatalf("service projection wrong: %+v", svc0)
	}

	// update + delete
	up := &model.ServiceCategory{ID: c.ID, Name: "AC2", Slug: "ac-2", IsActive: false, UpdatedAt: &now}
	if err := store.Update(ctx, db, up); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ = store.FindByID(ctx, db, c.ID)
	if got.Name != "AC2" || got.IsActive {
		t.Fatalf("update not applied: %+v", got)
	}
	if err := store.Update(ctx, db, &model.ServiceCategory{ID: c.ID, Name: "AC2", Slug: "ac-2", IsActive: true, UpdatedAt: &now}); err != nil {
		t.Fatalf("update-preserve: %v", err)
	}

	// delete then not-found
	if err := store.Delete(ctx, db, c.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := store.FindByID(ctx, db, c.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
	if err := store.Delete(ctx, db, c.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound on double delete, got %v", err)
	}
}
