package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Dwisajaa/golang-backend/internal/model"
)

const invoiceColumns = "id, booking_id, invoice_number, issued_at, due_at, subtotal, additional_cost, total_amount, status, notes, created_at, updated_at"

type MySQLInvoiceStore struct{}

func NewMySQLInvoiceStore() *MySQLInvoiceStore { return &MySQLInvoiceStore{} }

func (r *MySQLInvoiceStore) CountByCustomer(ctx context.Context, q Queryer, customerID uint64) (int, error) {
	var n int
	err := q.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM invoices i
		 JOIN bookings b ON b.id = i.booking_id
		 WHERE b.customer_id = ?`, customerID).Scan(&n)
	return n, err
}

func (r *MySQLInvoiceStore) ListByCustomer(ctx context.Context, q Queryer, customerID uint64, limit, offset int) ([]*model.Invoice, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT i.`+invoiceColumns+` FROM invoices i
		 JOIN bookings b ON b.id = i.booking_id
		 WHERE b.customer_id = ?
		 ORDER BY i.id DESC LIMIT ? OFFSET ?`, customerID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out, err := scanInvoices(rows)
	if err != nil {
		return nil, err
	}
	return r.attachBookings(ctx, q, out)
}

func (r *MySQLInvoiceStore) FindByID(ctx context.Context, q Queryer, id uint64) (*model.Invoice, error) {
	row := q.QueryRowContext(ctx, "SELECT "+invoiceColumns+" FROM invoices WHERE id = ?", id)
	inv, err := scanInvoiceRow(row)
	if err != nil {
		return nil, err
	}
	invs, err := r.attachBookings(ctx, q, []*model.Invoice{inv})
	if err != nil {
		return nil, err
	}
	return invs[0], nil
}

func scanInvoices(rows *sql.Rows) ([]*model.Invoice, error) {
	var out []*model.Invoice
	for rows.Next() {
		inv, err := scanInvoiceCols(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, inv)
	}
	return out, rows.Err()
}

func scanInvoiceRow(s rowScanner) (*model.Invoice, error) {
	inv, err := scanInvoiceCols(s)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return inv, nil
}

func scanInvoiceCols(s rowScanner) (*model.Invoice, error) {
	inv := &model.Invoice{}
	var issuedAt, dueAt, ca, ua sql.NullTime
	var sub, add, tot string
	var notes sql.NullString
	if err := s.Scan(&inv.ID, &inv.BookingID, &inv.InvoiceNumber,
		&issuedAt, &dueAt, &sub, &add, &tot, &inv.Status, &notes, &ca, &ua); err != nil {
		return nil, err
	}
	c1, _ := parsePriceString(sub)
	c2, _ := parsePriceString(add)
	c3, _ := parsePriceString(tot)
	inv.IssuedAt = nullTimePtr(issuedAt)
	inv.DueAt = nullTimePtr(dueAt)
	inv.SubtotalCents = c1
	inv.AdditionalCostCents = c2
	inv.TotalAmountCents = c3
	inv.Notes = nullStringPtr(notes)
	inv.CreatedAt = nullTimePtr(ca)
	inv.UpdatedAt = nullTimePtr(ua)
	return inv, nil
}

// attachBookings loads the booking row for each invoice (needed by the
// ownership policy). Batch, no N+1.
func (r *MySQLInvoiceStore) attachBookings(ctx context.Context, q Queryer, invoices []*model.Invoice) ([]*model.Invoice, error) {
	if len(invoices) == 0 {
		return invoices, nil
	}
	ids := make([]uint64, len(invoices))
	for i, inv := range invoices {
		ids[i] = inv.BookingID
	}
	ph := placeholdersFor(ids)
	rows, err := q.QueryContext(ctx,
		"SELECT "+bookingColumns+" FROM bookings WHERE id IN ("+ph+")", argsOf(ids)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byID := map[uint64]*model.Booking{}
	for rows.Next() {
		b, err := scanBookingCols(rows)
		if err != nil {
			return nil, err
		}
		byID[b.ID] = b
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, inv := range invoices {
		inv.Booking = byID[inv.BookingID]
	}
	return invoices, nil
}

func placeholdersFor(ids []uint64) string {
	var ph string
	for i := range ids {
		if i > 0 {
			ph += ","
		}
		ph += "?"
	}
	return ph
}

func argsOf(ids []uint64) []any {
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	return args
}
