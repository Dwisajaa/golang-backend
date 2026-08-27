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

// TestMySQLService_Coverage verifies service CRUD, active-only flows, money
// round-trip, unique classification, and the package delete guard against real
// MySQL. Gated by TEST_DATABASE_URL.
func TestMySQLService_Coverage(t *testing.T) {
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

	if _, err := db.ExecContext(ctx,
		`CREATE TABLE IF NOT EXISTS package_items (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
			package_id BIGINT UNSIGNED NOT NULL,
			service_id BIGINT UNSIGNED NOT NULL,
			quantity INT NOT NULL DEFAULT 1,
			created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL,
			PRIMARY KEY (id)
		) ENGINE=InnoDB`); err != nil {
		t.Fatalf("ensure package_items: %v", err)
	}

	store := NewMySQLServiceStore()
	now := time.Now().UTC()

	seed := func(name, slug string, catID uint64, active bool) *model.Service {
		c := &model.Service{
			ServiceCategoryID: catID, Name: name, Slug: slug,
			PriceCents: 15000, Unit: "per_service", IsActive: active,
			CreatedAt: &now, UpdatedAt: &now,
		}
		if err := store.Create(ctx, db, c); err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		return c
	}

	cat := &model.ServiceCategory{Name: "Cat", Slug: "cat", IsActive: true, CreatedAt: &now, UpdatedAt: &now}
	if err := NewMySQLServiceCategoryStore().Create(ctx, db, cat); err != nil {
		t.Fatalf("seed cat: %v", err)
	}

	active := seed("AC", "ac", cat.ID, true)
	seed("Wiring", "wiring", cat.ID, false) // inactive excluded from public list

	// money round-trip: cents -> DECIMAL -> cents
	got, err := store.FindByID(ctx, db, active.ID)
	if err != nil || got.PriceCents != 15000 {
		t.Fatalf("price round-trip: %+v err=%v", got, err)
	}
	if got.Unit != "per_service" || !got.IsActive {
		t.Fatalf("scan wrong: %+v", got)
	}

	// public active-only: detail + list exclude inactive
	if _, err := store.FindActiveByID(ctx, db, active.ID); err != nil {
		t.Fatalf("active detail: %v", err)
	}
	if _, err := store.FindByID(ctx, db, seed("Wiring2", "wiring2", cat.ID, false).ID); err != nil {
		t.Fatalf("admin detail (inactive) must be found: %v", err)
	}
	total, _ := store.Count(ctx, db, nil, "")
	if total != 1 {
		t.Fatalf("CountActive expected 1, got %d", total)
	}
	// search matches the active row by name
	total, _ = store.Count(ctx, db, nil, "AC")
	if total != 1 {
		t.Fatalf("search count wrong: %d", total)
	}
	items, err := store.List(ctx, db, nil, "", 15, 0)
	if err != nil || len(items) != 1 {
		t.Fatalf("list: %+v err=%v", items, err)
	}
	if items[0].Category == nil || items[0].Category.ID != cat.ID {
		t.Fatalf("category not attached: %+v", items[0])
	}

	// unique classification
	dup := &model.Service{ServiceCategoryID: cat.ID, Name: "AC", Slug: "ac-2", PriceCents: 1, Unit: "custom", CreatedAt: &now, UpdatedAt: &now}
	if err := store.Create(ctx, db, dup); !errors.Is(err, ErrDuplicateName) {
		t.Fatalf("expected ErrDuplicateName, got %v", err)
	}

	// update + delete guard via package_items
	if err := store.Update(ctx, db, &model.Service{
		ID: active.ID, ServiceCategoryID: cat.ID, Name: "AC Plus", Slug: "ac-plus",
		PriceCents: 20000, Unit: "per_unit", IsActive: true, UpdatedAt: &now,
	}); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ = store.FindByID(ctx, db, active.ID)
	if got.Name != "AC Plus" || got.PriceCents != 20000 {
		t.Fatalf("update wrong: %+v", got)
	}

	if used, _ := store.HasPackages(ctx, db, active.ID); used {
		t.Fatal("no package_items yet")
	}
	if _, err := db.ExecContext(ctx,
		"INSERT INTO package_items (package_id, service_id, quantity, created_at, updated_at) VALUES (?,?,1,?,?)",
		1, active.ID, now, now); err != nil {
		t.Fatalf("seed package_item: %v", err)
	}
	if used, _ := store.HasPackages(ctx, db, active.ID); !used {
		t.Fatal("HasPackages should be true after seeding")
	}

	if err := store.Delete(ctx, db, active.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := store.Delete(ctx, db, active.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound on double delete, got %v", err)
	}
}
