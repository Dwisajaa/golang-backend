package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/Dwisajaa/golang-backend/internal/model"
)

const packageColumns = "id, name, slug, description, price, duration, is_active, is_popular, created_at, updated_at"

// MySQLPackageStore implements PackageStore.
type MySQLPackageStore struct{}

func NewMySQLPackageStore() *MySQLPackageStore { return &MySQLPackageStore{} }

func (r *MySQLPackageStore) CountActive(ctx context.Context, q Queryer, search string) (int, error) {
	query := `SELECT COUNT(*) FROM packages
		WHERE is_active = 1
		  AND EXISTS (SELECT 1 FROM package_items pi
		              JOIN services sv ON sv.id = pi.service_id AND sv.is_active = 1
		              WHERE pi.package_id = packages.id)`
	args := []any{}
	if search != "" {
		term := "%" + search + "%"
		query += " AND (name LIKE ? OR description LIKE ?)"
		args = append(args, term, term)
	}
	var n int
	err := q.QueryRowContext(ctx, query, args...).Scan(&n)
	return n, err
}

func (r *MySQLPackageStore) ListActive(ctx context.Context, q Queryer, search string, limit, offset int) ([]*model.Package, error) {
	query := `SELECT ` + packageColumns + ` FROM packages
		WHERE is_active = 1
		  AND EXISTS (SELECT 1 FROM package_items pi
		              JOIN services sv ON sv.id = pi.service_id AND sv.is_active = 1
		              WHERE pi.package_id = packages.id)`
	args := []any{}
	if search != "" {
		term := "%" + search + "%"
		query += " AND (name LIKE ? OR description LIKE ?)"
		args = append(args, term, term)
	}
	query += " ORDER BY is_popular DESC, name ASC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	pkgs, err := scanPackages(rows)
	if err != nil || len(pkgs) == 0 {
		return pkgs, err
	}
	return r.attachItems(ctx, q, pkgs, true)
}

func (r *MySQLPackageStore) FindActiveByID(ctx context.Context, q Queryer, id uint64) (*model.Package, error) {
	row := q.QueryRowContext(ctx, "SELECT "+packageColumns+" FROM packages WHERE id = ? AND is_active = 1", id)
	p, err := scanPackageRow(row)
	if err != nil {
		return nil, err
	}
	pkgs, err := r.attachItems(ctx, q, []*model.Package{p}, true)
	if err != nil {
		return nil, err
	}
	return pkgs[0], nil
}

func (r *MySQLPackageStore) FindByID(ctx context.Context, q Queryer, id uint64) (*model.Package, error) {
	row := q.QueryRowContext(ctx, "SELECT "+packageColumns+" FROM packages WHERE id = ?", id)
	p, err := scanPackageRow(row)
	if err != nil {
		return nil, err
	}
	pkgs, err := r.attachItems(ctx, q, []*model.Package{p}, false)
	if err != nil {
		return nil, err
	}
	return pkgs[0], nil
}

func scanPackages(rows *sql.Rows) ([]*model.Package, error) {
	var out []*model.Package
	for rows.Next() {
		p, err := scanPkgCols(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanPackageRow(row scanner) (*model.Package, error) {
	p, err := scanPkgCols(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return p, nil
}

func scanPkgCols(s scanner) (*model.Package, error) {
	p := &model.Package{}
	var desc sql.NullString
	var dur sql.NullInt64
	var price string
	var createdAt, updatedAt sql.NullTime
	if err := s.Scan(&p.ID, &p.Name, &p.Slug, &desc, &price, &dur,
		&p.IsActive, &p.IsPopular, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	cents, err := parsePriceString(price)
	if err != nil {
		return nil, err
	}
	p.Description = nullStringPtr(desc)
	p.Duration = int64Ptr(dur)
	p.PriceCents = cents
	p.CreatedAt = nullTimePtr(createdAt)
	p.UpdatedAt = nullTimePtr(updatedAt)
	return p, nil
}

// attachItems loads package_items (with their services) for the given packages.
// activeOnly filters to items whose service is active (public endpoint parity).
func (r *MySQLPackageStore) attachItems(ctx context.Context, q Queryer, pkgs []*model.Package, activeOnly bool) ([]*model.Package, error) {
	ids := make([]uint64, 0, len(pkgs))
	for _, p := range pkgs {
		ids = append(ids, p.ID)
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}

	activeJoin := ""
	if activeOnly {
		activeJoin = " AND sv.is_active = 1"
	}
	rows, err := q.QueryContext(ctx,
		`SELECT pi.id, pi.package_id, pi.service_id, pi.quantity, pi.created_at, pi.updated_at,
		        sv.id, sv.name, sv.slug, sv.description, sv.price, sv.unit, sv.estimated_duration, sv.is_active
		 FROM package_items pi
		 JOIN services sv ON sv.id = pi.service_id`+activeJoin+`
		 WHERE pi.package_id IN (`+placeholders+`)
		 ORDER BY pi.package_id, pi.id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byPkg := map[uint64][]*model.PackageItem{}
	for rows.Next() {
		pi := &model.PackageItem{}
		svc := &model.Service{}
		var piCreated, piUpdated sql.NullTime
		var svcDesc sql.NullString
		var svcPrice string
		var svcEst sql.NullInt64
		if err := rows.Scan(
			&pi.ID, &pi.PackageID, &pi.ServiceID, &pi.Quantity, &piCreated, &piUpdated,
			&svc.ID, &svc.Name, &svc.Slug, &svcDesc, &svcPrice, &svc.Unit, &svcEst, &svc.IsActive,
		); err != nil {
			return nil, err
		}
		cents, err := parsePriceString(svcPrice)
		if err != nil {
			return nil, err
		}
		svc.Description = nullStringPtr(svcDesc)
		svc.PriceCents = cents
		svc.EstimatedDuration = int64Ptr(svcEst)
		pi.CreatedAt = nullTimePtr(piCreated)
		pi.UpdatedAt = nullTimePtr(piUpdated)
		pi.Service = svc
		byPkg[pi.PackageID] = append(byPkg[pi.PackageID], pi)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, p := range pkgs {
		p.Items = byPkg[p.ID]
	}
	return pkgs, nil
}

func (r *MySQLPackageStore) NameTaken(ctx context.Context, q Queryer, name string, ignoreID uint64) (bool, error) {
	return exists(ctx, q, "packages", "name", name, ignoreID)
}

func (r *MySQLPackageStore) SlugTaken(ctx context.Context, q Queryer, slug string, ignoreID uint64) (bool, error) {
	return exists(ctx, q, "packages", "slug", slug, ignoreID)
}

func (r *MySQLPackageStore) Create(ctx context.Context, q Queryer, p *model.Package) error {
	res, err := q.ExecContext(ctx,
		`INSERT INTO packages (name, slug, description, price, duration, is_active, is_popular, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.Name, p.Slug, p.Description, centsToDecimal(p.PriceCents), p.Duration,
		p.IsActive, p.IsPopular, p.CreatedAt, p.UpdatedAt)
	if err != nil {
		if _, dup := duplicateTarget(err); dup {
			return classifyPackageDuplicate(err)
		}
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	p.ID = uint64(id)
	return nil
}

func (r *MySQLPackageStore) InsertItems(ctx context.Context, q Queryer, packageID uint64, items []model.PackageItemInput) error {
	for _, item := range items {
		if _, err := q.ExecContext(ctx,
			`INSERT INTO package_items (package_id, service_id, quantity, created_at, updated_at)
			 VALUES (?, ?, ?, NOW(), NOW())`,
			packageID, item.ServiceID, item.Quantity); err != nil {
			return err
		}
	}
	return nil
}

func (r *MySQLPackageStore) DeleteItems(ctx context.Context, q Queryer, packageID uint64) error {
	_, err := q.ExecContext(ctx, "DELETE FROM package_items WHERE package_id = ?", packageID)
	return err
}

func (r *MySQLPackageStore) Update(ctx context.Context, q Queryer, p *model.Package) error {
	res, err := q.ExecContext(ctx,
		`UPDATE packages
		 SET name = ?, slug = ?, description = ?, price = ?, duration = ?,
		     is_active = ?, is_popular = ?, updated_at = ?
		 WHERE id = ?`,
		p.Name, p.Slug, p.Description, centsToDecimal(p.PriceCents), p.Duration,
		p.IsActive, p.IsPopular, p.UpdatedAt, p.ID)
	if err != nil {
		if _, dup := duplicateTarget(err); dup {
			return classifyPackageDuplicate(err)
		}
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *MySQLPackageStore) Delete(ctx context.Context, q Queryer, id uint64) error {
	res, err := q.ExecContext(ctx, "DELETE FROM packages WHERE id = ?", id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func classifyPackageDuplicate(err error) error {
	key, ok := duplicateTarget(err)
	if !ok {
		return err
	}
	if key == "packages_slug_unique" {
		return ErrDuplicateSlug
	}
	return ErrDuplicateName // packages_name_unique or default
}
