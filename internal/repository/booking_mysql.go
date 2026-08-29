package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/Dwisajaa/golang-backend/internal/model"
)

type MySQLBookingStore struct{}

func NewMySQLBookingStore() *MySQLBookingStore { return &MySQLBookingStore{} }

const bookingColumns = "id, booking_code, customer_id, booking_date, booking_time, address, address_detail, latitude, longitude, customer_note, additional_jobdesk, subtotal, additional_cost, total_price, status, created_at, updated_at"

func (r *MySQLBookingStore) CountByCustomer(ctx context.Context, q Queryer, customerID uint64) (int, error) {
	var n int
	err := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM bookings WHERE customer_id = ?", customerID).Scan(&n)
	return n, err
}

func (r *MySQLBookingStore) ListByCustomer(ctx context.Context, q Queryer, customerID uint64, limit, offset int) ([]*model.Booking, error) {
	rows, err := q.QueryContext(ctx,
		"SELECT "+bookingColumns+" FROM bookings WHERE customer_id = ? ORDER BY id DESC LIMIT ? OFFSET ?",
		customerID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanBookings(rows)
}

func (r *MySQLBookingStore) FindByID(ctx context.Context, q Queryer, id uint64) (*model.Booking, error) {
	row := q.QueryRowContext(ctx, "SELECT "+bookingColumns+" FROM bookings WHERE id = ?", id)
	return scanBookingRow(row)
}

func (r *MySQLBookingStore) FindByIDForUpdate(ctx context.Context, q Queryer, id uint64) (*model.Booking, error) {
	row := q.QueryRowContext(ctx, "SELECT "+bookingColumns+" FROM bookings WHERE id = ? FOR UPDATE", id)
	return scanBookingRow(row)
}

func scanBookings(rows *sql.Rows) ([]*model.Booking, error) {
	var out []*model.Booking
	for rows.Next() {
		b, err := scanBookingCols(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func scanBookingRow(s scanner) (*model.Booking, error) {
	b, err := scanBookingCols(s)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return b, nil
}

func scanBookingCols(s scanner) (*model.Booking, error) {
	b := &model.Booking{}
	var addrDetail, lat, lng, note, jobdesk sql.NullString
	var subtotal, addCost, total string
	var createdAt, updatedAt sql.NullTime
	var bookingDate string
	if err := s.Scan(
		&b.ID, &b.BookingCode, &b.CustomerID, &bookingDate, &b.BookingTime,
		&b.Address, &addrDetail, &lat, &lng, &note, &jobdesk,
		&subtotal, &addCost, &total, &b.Status, &createdAt, &updatedAt,
	); err != nil {
		return nil, err
	}
	b.BookingDate = bookingDate
	b.AddressDetail = nullStringPtr(addrDetail)
	b.Latitude = nullStringPtr(lat)
	b.Longitude = nullStringPtr(lng)
	b.CustomerNote = nullStringPtr(note)
	b.AdditionalJobdesk = nullStringPtr(jobdesk)
	c1, _ := parsePriceString(subtotal)
	c2, _ := parsePriceString(addCost)
	c3, _ := parsePriceString(total)
	b.SubtotalCents = c1
	b.AdditionalCostCents = c2
	b.TotalPriceCents = c3
	b.CreatedAt = nullTimePtr(createdAt)
	b.UpdatedAt = nullTimePtr(updatedAt)
	return b, nil
}

func (r *MySQLBookingStore) Create(ctx context.Context, q Queryer, b *model.Booking) error {
	res, err := q.ExecContext(ctx,
		`INSERT INTO bookings (booking_code, customer_id, booking_date, booking_time,
		  address, address_detail, latitude, longitude, customer_note, additional_jobdesk,
		  subtotal, additional_cost, total_price, status, created_at, updated_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		b.BookingCode, b.CustomerID, b.BookingDate, b.BookingTime,
		b.Address, b.AddressDetail, b.Latitude, b.Longitude, b.CustomerNote, b.AdditionalJobdesk,
		fmtCents(b.SubtotalCents), fmtCents(b.AdditionalCostCents), fmtCents(b.TotalPriceCents),
		b.Status, b.CreatedAt, b.UpdatedAt)
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
	b.ID = uint64(id)
	return nil
}

func (r *MySQLBookingStore) CreateItem(ctx context.Context, q Queryer, item *model.BookingItem) error {
	res, err := q.ExecContext(ctx,
		`INSERT INTO booking_items (booking_id, service_id, package_id, item_type, item_name,
		  quantity, unit_price, subtotal, created_at, updated_at)
		 VALUES (?,?,?,?,?,?,?,?,NOW(),NOW())`,
		item.BookingID, item.ServiceID, item.PackageID, item.ItemType, item.ItemName,
		item.Quantity, fmtCents(item.UnitPriceCents), fmtCents(item.SubtotalCents))
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	item.ID = uint64(id)
	return nil
}

func (r *MySQLBookingStore) CreateInvoice(ctx context.Context, q Queryer, inv *model.Invoice) error {
	res, err := q.ExecContext(ctx,
		`INSERT INTO invoices (booking_id, invoice_number, issued_at, due_at,
		  subtotal, additional_cost, total_amount, status, notes, created_at, updated_at)
		 VALUES (?,?,?,?,?,?,?,?,?,NOW(),NOW())`,
		inv.BookingID, inv.InvoiceNumber, inv.IssuedAt, inv.DueAt,
		fmtCents(inv.SubtotalCents), fmtCents(inv.AdditionalCostCents), fmtCents(inv.TotalAmountCents),
		inv.Status, inv.Notes)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	inv.ID = uint64(id)
	return nil
}

func (r *MySQLBookingStore) UpdateStatus(ctx context.Context, q Queryer, id uint64, status string) error {
	_, err := q.ExecContext(ctx, "UPDATE bookings SET status = ?, updated_at = NOW() WHERE id = ?", status, id)
	return err
}

func (r *MySQLBookingStore) UpdateInvoiceStatus(ctx context.Context, q Queryer, bookingID uint64, status string) error {
	_, err := q.ExecContext(ctx, "UPDATE invoices SET status = ?, updated_at = NOW() WHERE booking_id = ?", status, bookingID)
	return err
}

func (r *MySQLBookingStore) BookingCodeExists(ctx context.Context, q Queryer, code string) (bool, error) {
	var n int
	err := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM bookings WHERE booking_code = ?", code).Scan(&n)
	return n > 0, err
}

func (r *MySQLBookingStore) InvoiceNumberExists(ctx context.Context, q Queryer, number string) (bool, error) {
	var n int
	err := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM invoices WHERE invoice_number = ?", number).Scan(&n)
	return n > 0, err
}

// AttachItems batch-loads booking items for the given bookings.
func (r *MySQLBookingStore) AttachItems(ctx context.Context, q Queryer, bookings []*model.Booking) error {
	if len(bookings) == 0 {
		return nil
	}
	ids := make([]uint64, len(bookings))
	for i, b := range bookings {
		ids[i] = b.ID
	}
	ph := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := q.QueryContext(ctx,
		`SELECT id, booking_id, service_id, package_id, item_type, item_name,
		        quantity, unit_price, subtotal, created_at, updated_at
		 FROM booking_items WHERE booking_id IN (`+ph+`) ORDER BY booking_id, id`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	byBooking := map[uint64][]*model.BookingItem{}
	for rows.Next() {
		item := &model.BookingItem{}
		var svcID, pkgID sql.NullInt64
		var unitPrice, subtotal string
		var ca, ua sql.NullTime
		if err := rows.Scan(&item.ID, &item.BookingID, &svcID, &pkgID, &item.ItemType,
			&item.ItemName, &item.Quantity, &unitPrice, &subtotal, &ca, &ua); err != nil {
			return err
		}
		item.ServiceID = uint64Ptr(svcID)
		item.PackageID = uint64Ptr(pkgID)
		c1, _ := parsePriceString(unitPrice)
		c2, _ := parsePriceString(subtotal)
		item.UnitPriceCents = c1
		item.SubtotalCents = c2
		item.CreatedAt = nullTimePtr(ca)
		item.UpdatedAt = nullTimePtr(ua)
		byBooking[item.BookingID] = append(byBooking[item.BookingID], item)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, b := range bookings {
		b.Items = byBooking[b.ID]
	}
	return nil
}

// AttachInvoices batch-loads invoices (one per booking).
func (r *MySQLBookingStore) AttachInvoices(ctx context.Context, q Queryer, bookings []*model.Booking) error {
	if len(bookings) == 0 {
		return nil
	}
	ids := make([]uint64, len(bookings))
	for i, b := range bookings {
		ids[i] = b.ID
	}
	ph := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := q.QueryContext(ctx,
		`SELECT id, booking_id, invoice_number, issued_at, due_at,
		        subtotal, additional_cost, total_amount, status, notes, created_at, updated_at
		 FROM invoices WHERE booking_id IN (`+ph+`)`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	byBooking := map[uint64]*model.Invoice{}
	for rows.Next() {
		inv := &model.Invoice{}
		var issuedAt, dueAt, ca, ua sql.NullTime
		var sub, add, tot string
		var notes sql.NullString
		if err := rows.Scan(&inv.ID, &inv.BookingID, &inv.InvoiceNumber, &issuedAt, &dueAt,
			&sub, &add, &tot, &inv.Status, &notes, &ca, &ua); err != nil {
			return err
		}
		inv.IssuedAt = nullTimePtr(issuedAt)
		inv.DueAt = nullTimePtr(dueAt)
		c1, _ := parsePriceString(sub)
		c2, _ := parsePriceString(add)
		c3, _ := parsePriceString(tot)
		inv.SubtotalCents = c1
		inv.AdditionalCostCents = c2
		inv.TotalAmountCents = c3
		inv.Notes = nullStringPtr(notes)
		inv.CreatedAt = nullTimePtr(ca)
		inv.UpdatedAt = nullTimePtr(ua)
		byBooking[inv.BookingID] = inv
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, b := range bookings {
		b.Invoice = byBooking[b.ID]
	}
	return nil
}

func uint64Ptr(n sql.NullInt64) *uint64 {
	if !n.Valid {
		return nil
	}
	v := uint64(n.Int64)
	return &v
}

// Admin list
func (r *MySQLBookingStore) AdminCount(ctx context.Context, q Queryer, f AdminBookingFilters) (int, error) {
	query := "SELECT COUNT(*) FROM bookings"
	args := []any{}
	where := adminWhere(f, &args)
	if where != "" {
		query += " WHERE " + where
	}
	var n int
	err := q.QueryRowContext(ctx, query, args...).Scan(&n)
	return n, err
}

func (r *MySQLBookingStore) AdminList(ctx context.Context, q Queryer, f AdminBookingFilters, limit, offset int) ([]*model.Booking, error) {
	query := "SELECT " + bookingColumns + " FROM bookings"
	args := []any{}
	where := adminWhere(f, &args)
	if where != "" {
		query += " WHERE " + where
	}
	query += " ORDER BY id DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanBookings(rows)
}

func adminWhere(f AdminBookingFilters, args *[]any) string {
	var clauses []string
	if f.Search != "" {
		clauses = append(clauses, "booking_code LIKE ?")
		*args = append(*args, "%"+f.Search+"%")
	}
	if f.Status != "" {
		clauses = append(clauses, "status = ?")
		*args = append(*args, f.Status)
	}
	return strings.Join(clauses, " AND ")
}

// ---- Booking verify/completion (FASE 13) ----

func (r *MySQLBookingStore) FindLatestAssignmentByStatus(ctx context.Context, q Queryer, bookingID uint64, status string) (*model.BookingAssignment, error) {
	row := q.QueryRowContext(ctx,
		"SELECT "+assignmentColumns+" FROM booking_assignments WHERE booking_id = ? AND status = ? ORDER BY id DESC LIMIT 1",
		bookingID, status)
	return scanAssignmentRow(row)
}

func (r *MySQLBookingStore) LockAssignmentForUpdate(ctx context.Context, q Queryer, id uint64) (*model.BookingAssignment, error) {
	row := q.QueryRowContext(ctx, "SELECT "+assignmentColumns+" FROM booking_assignments WHERE id = ? FOR UPDATE", id)
	return scanAssignmentRow(row)
}

func (r *MySQLBookingStore) UpdateAssignmentVerifiedNote(ctx context.Context, q Queryer, id uint64, note string) error {
	_, err := q.ExecContext(ctx,
		"UPDATE booking_assignments SET admin_verification_note = ?, updated_at = NOW() WHERE id = ?", note, id)
	return err
}

func (r *MySQLBookingStore) RevertAssignmentCompleted(ctx context.Context, q Queryer, id uint64, note string) error {
	_, err := q.ExecContext(ctx,
		"UPDATE booking_assignments SET status = '"+model.AssignmentStatusAccepted+"', completed_at = NULL, admin_verification_note = ?, updated_at = NOW() WHERE id = ?",
		note, id)
	return err
}

// LoadBookingFull loads booking + customer + items + invoice + assignments
// (with technician summaries) — the verify response envelope. Batch, no N+1.
func (r *MySQLBookingStore) LoadBookingFull(ctx context.Context, q Queryer, id uint64) (*model.Booking, error) {
	row := q.QueryRowContext(ctx, "SELECT "+bookingColumns+" FROM bookings WHERE id = ?", id)
	b, err := scanBookingRow(row)
	if err != nil {
		return nil, err
	}
	// customer
	crow := q.QueryRowContext(ctx, "SELECT id, name, email FROM users WHERE id = ?", b.CustomerID)
	c := &model.User{}
	if crow.Scan(&c.ID, &c.Name, &c.Email) == nil {
		b.Customer = c
	}
	// items
	if err := r.AttachItems(ctx, q, []*model.Booking{b}); err != nil {
		return nil, err
	}
	// invoice
	if err := r.AttachInvoices(ctx, q, []*model.Booking{b}); err != nil {
		return nil, err
	}
	// assignments with technician (id,name)
	rows, err := q.QueryContext(ctx,
		"SELECT "+assignmentColumns+" FROM booking_assignments WHERE booking_id = ? ORDER BY id DESC", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		a, err := scanAssignmentRow(rows)
		if err != nil {
			return nil, err
		}
		trow := q.QueryRowContext(ctx, "SELECT id, name FROM users WHERE id = ?", a.TechnicianID)
		tm := &model.User{}
		if trow.Scan(&tm.ID, &tm.Name) == nil {
			a.Technician = tm
		}
		b.Assignments = append(b.Assignments, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return b, nil
}
