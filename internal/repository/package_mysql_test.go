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

func TestMySQLPackage_Coverage(t *testing.T) {
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
		`CREATE TABLE IF NOT EXISTS packages (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
			name VARCHAR(255) NOT NULL, slug VARCHAR(255) NOT NULL,
			description TEXT NULL, price DECIMAL(12,2) NOT NULL DEFAULT 0.00,
			duration INT NULL, is_active TINYINT(1) NOT NULL DEFAULT 1,
			is_popular TINYINT(1) NOT NULL DEFAULT 0,
			created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL,
			PRIMARY KEY (id), UNIQUE KEY packages_slug_unique (slug)
		) ENGINE=InnoDB`); err != nil {
		t.Fatalf("ensure packages: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`CREATE TABLE IF NOT EXISTS package_items (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
			package_id BIGINT UNSIGNED NOT NULL, service_id BIGINT UNSIGNED NOT NULL,
			quantity INT NOT NULL DEFAULT 1,
			created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL,
			PRIMARY KEY (id)
		) ENGINE=InnoDB`); err != nil {
		t.Fatalf("ensure package_items: %v", err)
	}

	store := NewMySQLPackageStore()
	catStore := NewMySQLServiceCategoryStore()
	svcStore := NewMySQLServiceStore()
	now := time.Now().UTC()

	cat := &model.ServiceCategory{Name: "PkgCat", Slug: "pkgcat", IsActive: true, CreatedAt: &now, UpdatedAt: &now}
	if err := catStore.Create(ctx, db, cat); err != nil {
		t.Fatalf("seed cat: %v", err)
	}
	svc := &model.Service{ServiceCategoryID: cat.ID, Name: "PkgSvc", Slug: "pkgsvc", PriceCents: 10000, Unit: "per_service", IsActive: true, CreatedAt: &now, UpdatedAt: &now}
	if err := svcStore.Create(ctx, db, svc); err != nil {
		t.Fatalf("seed svc: %v", err)
	}

	// create package + items
	pkg := &model.Package{Name: "Pkg1", Slug: "pkg1", PriceCents: 20000, IsActive: true, IsPopular: true, CreatedAt: &now, UpdatedAt: &now}
	if err := store.Create(ctx, db, pkg); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := store.InsertItems(ctx, db, pkg.ID, []model.PackageItemInput{{ServiceID: svc.ID, Quantity: 3}}); err != nil {
		t.Fatalf("insert items: %v", err)
	}

	// find with items
	got, err := store.FindByID(ctx, db, pkg.ID)
	if err != nil || got.Name != "Pkg1" || got.PriceCents != 20000 {
		t.Fatalf("find: %+v err=%v", got, err)
	}
	if len(got.Items) != 1 || got.Items[0].Quantity != 3 || got.Items[0].Service == nil {
		t.Fatalf("items wrong: %+v", got.Items)
	}
	if got.Items[0].Service.PriceCents != 10000 {
		t.Fatalf("nested service price wrong: %d", got.Items[0].Service.PriceCents)
	}

	// active list
	total, _ := store.CountActive(ctx, db, "")
	if total != 1 {
		t.Fatalf("CountActive: %d", total)
	}
	items, _ := store.ListActive(ctx, db, "", 15, 0)
	if len(items) != 1 || len(items[0].Items) != 1 {
		t.Fatalf("list: %+v", items)
	}

	// unique slug
	dup := &model.Package{Name: "Pkg2", Slug: "pkg1", PriceCents: 1, CreatedAt: &now, UpdatedAt: &now}
	if err := store.Create(ctx, db, dup); !errors.Is(err, ErrDuplicateSlug) {
		t.Fatalf("expected ErrDuplicateSlug, got %v", err)
	}

	// update + replace items
	if err := store.Update(ctx, db, &model.Package{ID: pkg.ID, Name: "Pkg1Up", Slug: "pkg1up", PriceCents: 25000, IsActive: true, UpdatedAt: &now}); err != nil {
		t.Fatalf("update: %v", err)
	}
	if err := store.DeleteItems(ctx, db, pkg.ID); err != nil {
		t.Fatalf("delete items: %v", err)
	}
	if err := store.InsertItems(ctx, db, pkg.ID, []model.PackageItemInput{{ServiceID: svc.ID, Quantity: 5}}); err != nil {
		t.Fatalf("re-insert items: %v", err)
	}
	got, _ = store.FindByID(ctx, db, pkg.ID)
	if got.Name != "Pkg1Up" || len(got.Items) != 1 || got.Items[0].Quantity != 5 {
		t.Fatalf("after update: %+v items=%+v", got, got.Items)
	}

	// delete (hard, no guard)
	if err := store.Delete(ctx, db, pkg.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := store.Delete(ctx, db, pkg.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
