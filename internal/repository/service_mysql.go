package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/Dwisajaa/golang-backend/internal/model"
)

const serviceColumns = "id, service_category_id, name, slug, description, price, unit, estimated_duration, is_active, created_at, updated_at"

// serviceActiveFilter is the shared active-service + active-category predicate.
const serviceActiveFilter = `sv.category_id = s.id AND sv.is_active = 1 AND s.is_active = 1`

// MySQLServiceStore implements ServiceStore.
type MySQLServiceStore struct{}

func NewMySQLServiceStore() *MySQLServiceStore { return &MySQLServiceStore{} }

func (r *MySQLServiceStore) Count(ctx context.Context, q Queryer, categoryID *uint64, search string) (int, error) {
	query := `SELECT COUNT(*) FROM services
		WHERE is_active = 1
		  AND EXISTS (SELECT 1 FROM service_categories s WHERE ` + serviceActiveFilter + `)`
	args := []any{}
	if categoryID != nil {
		query += " AND service_category_id = ?"
		args = append(args, *categoryID)
	}
	if search != "" {
		term := "%" + search + "%"
		query += " AND (name LIKE ? OR description LIKE ?)"
		args = append(args, term, term)
	}
	var n int
	err := q.QueryRowContext(ctx, query, args...).Scan(&n)
	return n, err
}

func (r *MySQLServiceStore) List(ctx context.Context, q Queryer, categoryID *uint64, search string, limit, offset int) ([]*model.Service, error) {
	query := `SELECT ` + serviceColumns + `
		FROM services
		WHERE is_active = 1
		  AND EXISTS (SELECT 1 FROM service_categories s WHERE ` + serviceActiveFilter + `)`
	args := []any{}
	if categoryID != nil {
		query += " AND service_category_id = ?"
		args = append(args, *categoryID)
	}
	if search != "" {
		term := "%" + search + "%"
		query += " AND (name LIKE ? OR description LIKE ?)"
		args = append(args, term, term)
	}
	query += " ORDER BY name LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	services, err := scanServices(rows)
	if err != nil {
		return nil, err
	}
	if len(services) == 0 {
		return services, nil
	}
	if err := r.attachCategories(ctx, q, services); err != nil {
		return nil, err
	}
	return services, nil
}

func scanServices(rows *sql.Rows) ([]*model.Service, error) {
	var out []*model.Service
	for rows.Next() {
		s := &model.Service{}
		var desc sql.NullString
		var est sql.NullInt64
		var createdAt, updatedAt sql.NullTime
		var price string
		if err := rows.Scan(
			&s.ID, &s.ServiceCategoryID, &s.Name, &s.Slug, &desc, &price, &s.Unit, &est,
			&s.IsActive, &createdAt, &updatedAt,
		); err != nil {
			return nil, err
		}
		cents, err := parsePriceString(price)
		if err != nil {
			return nil, err
		}
		s.Description = nullStringPtr(desc)
		s.EstimatedDuration = int64Ptr(est)
		s.PriceCents = cents
		s.CreatedAt = nullTimePtr(createdAt)
		s.UpdatedAt = nullTimePtr(updatedAt)
		out = append(out, s)
	}
	return out, rows.Err()
}

// parsePriceString converts a DECIMAL(12,2) string snapshot (e.g. "150.00")
// into integer cents without float arithmetic.
func parsePriceString(s string) (int64, error) {
	neg := false
	if strings.HasPrefix(s, "-") {
		neg = true
		s = s[1:]
	}
	dot := strings.IndexByte(s, '.')
	var whole, frac string
	if dot >= 0 {
		whole, frac = s[:dot], s[dot+1:]
	} else {
		whole = s
	}
	if len(frac) > 2 {
		frac = frac[:2]
	}
	for len(frac) < 2 {
		frac += "0"
	}
	var total int64
	for _, ch := range whole + frac {
		if ch < '0' || ch > '9' {
			return 0, errors.New("invalid money string")
		}
		total = total*10 + int64(ch-'0')
	}
	if neg {
		total = -total
	}
	return total, nil
}

// attachCategories batch-loads categories (no services) for list/detail
// responses — CategoryResource then renders services: [].
func (r *MySQLServiceStore) attachCategories(ctx context.Context, q Queryer, services []*model.Service) error {
	ids := make([]uint64, 0, len(services))
	seen := map[uint64]bool{}
	for _, s := range services {
		if !seen[s.ServiceCategoryID] {
			seen[s.ServiceCategoryID] = true
			ids = append(ids, s.ServiceCategoryID)
		}
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := q.QueryContext(ctx,
		"SELECT id, name, slug, description, icon, is_active FROM service_categories WHERE id IN ("+placeholders+")", args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	byID := map[uint64]*model.ServiceCategory{}
	for rows.Next() {
		c := &model.ServiceCategory{}
		var desc, icon sql.NullString
		if err := rows.Scan(&c.ID, &c.Name, &c.Slug, &desc, &icon, &c.IsActive); err != nil {
			return err
		}
		c.Description = nullStringPtr(desc)
		c.Icon = nullStringPtr(icon)
		byID[c.ID] = c
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, s := range services {
		s.Category = byID[s.ServiceCategoryID]
	}
	return nil
}

func (r *MySQLServiceStore) FindActiveByID(ctx context.Context, q Queryer, id uint64) (*model.Service, error) {
	row := q.QueryRowContext(ctx,
		"SELECT "+serviceColumns+` FROM services
		 WHERE id = ? AND is_active = 1
		   AND EXISTS (SELECT 1 FROM service_categories c WHERE c.id = services.service_category_id AND c.is_active = 1)`, id)
	s, err := scanServiceRow(row)
	if err != nil {
		return nil, err
	}
	if err := r.attachCategories(ctx, q, []*model.Service{s}); err != nil {
		return nil, err
	}
	return s, nil
}

func (r *MySQLServiceStore) FindByID(ctx context.Context, q Queryer, id uint64) (*model.Service, error) {
	row := q.QueryRowContext(ctx, "SELECT "+serviceColumns+" FROM services WHERE id = ?", id)
	return scanServiceRow(row)
}

func scanServiceRow(row rowScanner) (*model.Service, error) {
	s := &model.Service{}
	var desc sql.NullString
	var est sql.NullInt64
	var createdAt, updatedAt sql.NullTime
	var price string
	if err := row.Scan(
		&s.ID, &s.ServiceCategoryID, &s.Name, &s.Slug, &desc, &price, &s.Unit, &est,
		&s.IsActive, &createdAt, &updatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	cents, err := parsePriceString(price)
	if err != nil {
		return nil, err
	}
	s.Description = nullStringPtr(desc)
	s.EstimatedDuration = int64Ptr(est)
	s.PriceCents = cents
	s.CreatedAt = nullTimePtr(createdAt)
	s.UpdatedAt = nullTimePtr(updatedAt)
	return s, nil
}

func (r *MySQLServiceStore) CategoryExists(ctx context.Context, q Queryer, id uint64) (bool, error) {
	var n int
	err := q.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM service_categories WHERE id = ?", id).Scan(&n)
	return n > 0, err
}

func (r *MySQLServiceStore) NameTaken(ctx context.Context, q Queryer, name string, ignoreID uint64) (bool, error) {
	return exists(ctx, q, "services", "name", name, ignoreID)
}

func (r *MySQLServiceStore) SlugTaken(ctx context.Context, q Queryer, slug string, ignoreID uint64) (bool, error) {
	return exists(ctx, q, "services", "slug", slug, ignoreID)
}

func (r *MySQLServiceStore) Create(ctx context.Context, q Queryer, s *model.Service) error {
	res, err := q.ExecContext(ctx,
		`INSERT INTO services (service_category_id, name, slug, description, price, unit, estimated_duration, is_active, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ServiceCategoryID, s.Name, s.Slug, s.Description, centsToDecimal(s.PriceCents),
		s.Unit, s.EstimatedDuration, s.IsActive, s.CreatedAt, s.UpdatedAt)
	if err != nil {
		if _, dup := duplicateTarget(err); dup {
			return classifyServiceDuplicate(err)
		}
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	s.ID = uint64(id)
	return nil
}

func (r *MySQLServiceStore) Update(ctx context.Context, q Queryer, s *model.Service) error {
	res, err := q.ExecContext(ctx,
		`UPDATE services
		 SET service_category_id = ?, name = ?, slug = ?, description = ?, price = ?, unit = ?,
		     estimated_duration = ?, is_active = ?, updated_at = ?
		 WHERE id = ?`,
		s.ServiceCategoryID, s.Name, s.Slug, s.Description, centsToDecimal(s.PriceCents),
		s.Unit, s.EstimatedDuration, s.IsActive, s.UpdatedAt, s.ID)
	if err != nil {
		if _, dup := duplicateTarget(err); dup {
			return classifyServiceDuplicate(err)
		}
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// centsToDecimal serializes integer cents to a DECIMAL-friendly string.
func centsToDecimal(cents int64) string {
	return fmtCents(cents)
}

func (r *MySQLServiceStore) HasPackages(ctx context.Context, q Queryer, id uint64) (bool, error) {
	var n int
	err := q.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM package_items WHERE service_id = ?", id).Scan(&n)
	return n > 0, err
}

func (r *MySQLServiceStore) Delete(ctx context.Context, q Queryer, id uint64) error {
	res, err := q.ExecContext(ctx, "DELETE FROM services WHERE id = ?", id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ServiceIDsExist verifies all given service IDs exist in the services table;
// used by PackageService to validate items.*.service_id before creating items.
func (r *MySQLServiceStore) ServiceIDsExist(ctx context.Context, q Queryer, ids []uint64) (bool, error) {
	if len(ids) == 0 {
		return true, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	var n int
	err := q.QueryRowContext(ctx,
		"SELECT COUNT(DISTINCT id) FROM services WHERE id IN ("+placeholders+")", args...).Scan(&n)
	if err != nil {
		return false, err
	}
	return n == len(ids), nil
}

func fmtCents(cents int64) string {
	neg := ""
	if cents < 0 {
		neg = "-"
		cents = -cents
	}
	whole := cents / 100
	frac := cents % 100
	b := []byte(neg)
	b = append(b, digits(whole)...)
	b = append(b, '.')
	if frac < 10 {
		b = append(b, '0')
	}
	b = append(b, digits(frac)...)
	return string(b)
}

func digits(n int64) []byte {
	if n == 0 {
		return []byte{'0'}
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return buf[i:]
}
