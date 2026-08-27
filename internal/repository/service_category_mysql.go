package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/Dwisajaa/golang-backend/internal/model"
)

const categoryColumns = "id, name, slug, description, icon, is_active, created_at, updated_at"

// MySQLServiceCategoryStore implements ServiceCategoryStore.
type MySQLServiceCategoryStore struct{}

func NewMySQLServiceCategoryStore() *MySQLServiceCategoryStore { return &MySQLServiceCategoryStore{} }

func (s *MySQLServiceCategoryStore) CountActive(ctx context.Context, q Queryer) (int, error) {
	var n int
	err := q.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM service_categories c
		 WHERE c.is_active = 1
		   AND EXISTS (SELECT 1 FROM services sv WHERE sv.service_category_id = c.id AND sv.is_active = 1)`).Scan(&n)
	return n, err
}

func (s *MySQLServiceCategoryStore) ListActive(ctx context.Context, q Queryer, limit, offset int) ([]*model.ServiceCategory, error) {
	rows, err := q.QueryContext(ctx,
		"SELECT "+categoryColumns+` FROM service_categories c
		 WHERE c.is_active = 1
		   AND EXISTS (SELECT 1 FROM services sv WHERE sv.service_category_id = c.id AND sv.is_active = 1)
		 ORDER BY c.name
		 LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*model.ServiceCategory
	ids := []uint64{}
	for rows.Next() {
		c := &model.ServiceCategory{}
		var desc, icon sql.NullString
		var createdAt, updatedAt sql.NullTime
		if err := rows.Scan(&c.ID, &c.Name, &c.Slug, &desc, &icon, &c.IsActive, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		c.Description = nullStringPtr(desc)
		c.Icon = nullStringPtr(icon)
		c.CreatedAt = nullTimePtr(createdAt)
		c.UpdatedAt = nullTimePtr(updatedAt)
		out = append(out, c)
		ids = append(ids, c.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return out, nil
	}
	if err := s.attachActiveServices(ctx, q, out, ids); err != nil {
		return nil, err
	}
	return out, nil
}

// attachActiveServices loads active services (ordered by name) for the given
// categories into each category's Services slice — mirrors Laravel's
// ->with(['services' => active, orderBy name]).
func (s *MySQLServiceCategoryStore) attachActiveServices(ctx context.Context, q Queryer, cats []*model.ServiceCategory, ids []uint64) error {
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := q.QueryContext(ctx,
		`SELECT service_category_id, id, name, slug, description,
		        CAST(price * 100 AS SIGNED) AS price_cents, unit, estimated_duration, is_active
		 FROM services
		 WHERE is_active = 1 AND service_category_id IN (`+placeholders+`)
		 ORDER BY service_category_id, name`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	byCat := map[uint64][]*model.ServiceLite{}
	for rows.Next() {
		svc := &model.ServiceLite{}
		var catID uint64
		var desc sql.NullString
		var est sql.NullInt64
		if err := rows.Scan(&catID, &svc.ID, &svc.Name, &svc.Slug, &desc,
			&svc.PriceCents, &svc.Unit, &est, &svc.IsActive); err != nil {
			return err
		}
		svc.Description = nullStringPtr(desc)
		svc.EstimatedDuration = int64Ptr(est)
		byCat[catID] = append(byCat[catID], svc)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, c := range cats {
		c.Services = byCat[c.ID]
	}
	return nil
}

func int64Ptr(n sql.NullInt64) *int64 {
	if !n.Valid {
		return nil
	}
	v := n.Int64
	return &v
}

func (s *MySQLServiceCategoryStore) FindByID(ctx context.Context, q Queryer, id uint64) (*model.ServiceCategory, error) {
	row := q.QueryRowContext(ctx,
		"SELECT "+categoryColumns+" FROM service_categories WHERE id = ?", id)
	c := &model.ServiceCategory{}
	var desc, icon sql.NullString
	var createdAt, updatedAt sql.NullTime
	if err := row.Scan(&c.ID, &c.Name, &c.Slug, &desc, &icon, &c.IsActive, &createdAt, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	c.Description = nullStringPtr(desc)
	c.Icon = nullStringPtr(icon)
	c.CreatedAt = nullTimePtr(createdAt)
	c.UpdatedAt = nullTimePtr(updatedAt)
	return c, nil
}

func (s *MySQLServiceCategoryStore) HasServices(ctx context.Context, q Queryer, id uint64) (bool, error) {
	var n int
	err := q.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM services WHERE service_category_id = ?", id).Scan(&n)
	return n > 0, err
}

func (s *MySQLServiceCategoryStore) NameTaken(ctx context.Context, q Queryer, name string, ignoreID uint64) (bool, error) {
	return exists(ctx, q, "service_categories", "name", name, ignoreID)
}

func (s *MySQLServiceCategoryStore) SlugTaken(ctx context.Context, q Queryer, slug string, ignoreID uint64) (bool, error) {
	return exists(ctx, q, "service_categories", "slug", slug, ignoreID)
}

func exists(ctx context.Context, q Queryer, table, column, value string, ignoreID uint64) (bool, error) {
	query := "SELECT COUNT(*) FROM " + table + " WHERE " + column + " = ?"
	var args []any = []any{value}
	if ignoreID != 0 {
		query += " AND id <> ?"
		args = append(args, ignoreID)
	}
	var n int
	err := q.QueryRowContext(ctx, query, args...).Scan(&n)
	return n > 0, err
}

func (s *MySQLServiceCategoryStore) Create(ctx context.Context, q Queryer, c *model.ServiceCategory) error {
	res, err := q.ExecContext(ctx,
		`INSERT INTO service_categories (name, slug, description, icon, is_active, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		c.Name, c.Slug, c.Description, c.Icon, c.IsActive, c.CreatedAt, c.UpdatedAt)
	if err != nil {
		if _, dup := duplicateTarget(err); dup {
			return classifyServiceCategoryDuplicate(err)
		}
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	c.ID = uint64(id)
	return nil
}

func (s *MySQLServiceCategoryStore) Update(ctx context.Context, q Queryer, c *model.ServiceCategory) error {
	res, err := q.ExecContext(ctx,
		`UPDATE service_categories
		 SET name = ?, slug = ?, description = ?, icon = ?, is_active = ?, updated_at = ?
		 WHERE id = ?`,
		c.Name, c.Slug, c.Description, c.Icon, c.IsActive, c.UpdatedAt, c.ID)
	if err != nil {
		if _, dup := duplicateTarget(err); dup {
			return classifyServiceCategoryDuplicate(err)
		}
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *MySQLServiceCategoryStore) Delete(ctx context.Context, q Queryer, id uint64) error {
	res, err := q.ExecContext(ctx, "DELETE FROM service_categories WHERE id = ?", id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
