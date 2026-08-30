package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Dwisajaa/golang-backend/internal/model"
)

const reviewColumns = "id, booking_id, customer_id, technician_id, rating, comment, status, created_at"

type MySQLReviewStore struct{}

func NewMySQLReviewStore() *MySQLReviewStore { return &MySQLReviewStore{} }

func (r *MySQLReviewStore) Create(ctx context.Context, q Queryer, rv *model.Review) error {
	res, err := q.ExecContext(ctx,
		"INSERT INTO reviews (booking_id, customer_id, technician_id, rating, comment, status) VALUES (?,?,?,?,?,?)",
		rv.BookingID, rv.CustomerID, rv.TechnicianID, rv.Rating, rv.Comment, rv.Status)
	if err != nil {
		if _, dup := duplicateTarget(err); dup {
			return ErrDuplicate
		}
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	rv.ID = uint64(id)
	return nil
}

func (r *MySQLReviewStore) FindByBooking(ctx context.Context, q Queryer, bookingID uint64) (*model.Review, error) {
	row := q.QueryRowContext(ctx,
		"SELECT "+reviewColumns+" FROM reviews WHERE booking_id = ?", bookingID)
	rv, err := scanReviewRow(row)
	if err != nil {
		return nil, err
	}
	if err := r.attachUsers(ctx, q, []*model.Review{rv}); err != nil {
		return nil, err
	}
	return rv, nil
}

func (r *MySQLReviewStore) FindByID(ctx context.Context, q Queryer, id uint64) (*model.Review, error) {
	row := q.QueryRowContext(ctx, "SELECT "+reviewColumns+" FROM reviews WHERE id = ?", id)
	return scanReviewRow(row)
}

func (r *MySQLReviewStore) ReviewExists(ctx context.Context, q Queryer, bookingID uint64) (bool, error) {
	var n int
	err := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM reviews WHERE booking_id = ?", bookingID).Scan(&n)
	return n > 0, err
}

func (r *MySQLReviewStore) LatestAssignmentTechnicianID(ctx context.Context, q Queryer, bookingID uint64) (uint64, error) {
	var id uint64
	err := q.QueryRowContext(ctx,
		"SELECT technician_id FROM booking_assignments WHERE booking_id = ? ORDER BY id DESC LIMIT 1",
		bookingID).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrNotFound
		}
		return 0, err
	}
	return id, nil
}

func (r *MySQLReviewStore) AdminCount(ctx context.Context, q Queryer, status string) (int, error) {
	query := "SELECT COUNT(*) FROM reviews"
	args := []any{}
	if status != "" {
		query += " WHERE status = ?"
		args = append(args, status)
	}
	var n int
	err := q.QueryRowContext(ctx, query, args...).Scan(&n)
	return n, err
}

func (r *MySQLReviewStore) AdminList(ctx context.Context, q Queryer, status string, limit, offset int) ([]*model.Review, error) {
	query := "SELECT " + reviewColumns + " FROM reviews"
	args := []any{}
	if status != "" {
		query += " WHERE status = ?"
		args = append(args, status)
	}
	query += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out, err := scanReviews(rows)
	if err != nil {
		return nil, err
	}
	if err := r.attachUsers(ctx, q, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *MySQLReviewStore) UpdateStatus(ctx context.Context, q Queryer, id uint64, status string) error {
	_, err := q.ExecContext(ctx,
		"UPDATE reviews SET status = ?, updated_at = NOW() WHERE id = ?", status, id)
	return err
}

func scanReviews(rows *sql.Rows) ([]*model.Review, error) {
	var out []*model.Review
	for rows.Next() {
		rv, err := scanReviewCols(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rv)
	}
	return out, rows.Err()
}

func scanReviewRow(s rowScanner) (*model.Review, error) {
	rv, err := scanReviewCols(s)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return rv, nil
}

func scanReviewCols(s rowScanner) (*model.Review, error) {
	rv := &model.Review{}
	var comment sql.NullString
	var ca sql.NullTime
	if err := s.Scan(&rv.ID, &rv.BookingID, &rv.CustomerID, &rv.TechnicianID,
		&rv.Rating, &comment, &rv.Status, &ca); err != nil {
		return nil, err
	}
	rv.Comment = nullStringPtr(comment)
	rv.CreatedAt = nullTimePtr(ca)
	return rv, nil
}

// attachUsers batch-loads customer + technician name/id for the resource.
func (r *MySQLReviewStore) attachUsers(ctx context.Context, q Queryer, reviews []*model.Review) error {
	ids := []uint64{}
	seen := map[uint64]bool{}
	for _, rv := range reviews {
		for _, id := range []uint64{rv.CustomerID, rv.TechnicianID} {
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	if len(ids) == 0 {
		return nil
	}
	rows, err := q.QueryContext(ctx,
		"SELECT id, name FROM users WHERE id IN ("+placeholdersFor(ids)+")", argsOf(ids)...)
	if err != nil {
		return err
	}
	defer rows.Close()
	byID := map[uint64]*model.User{}
	for rows.Next() {
		u := &model.User{}
		if err := rows.Scan(&u.ID, &u.Name); err != nil {
			return err
		}
		byID[u.ID] = u
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, rv := range reviews {
		rv.Customer = byID[rv.CustomerID]
		rv.Technician = byID[rv.TechnicianID]
	}
	return nil
}
