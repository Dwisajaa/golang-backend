package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Dwisajaa/golang-backend/internal/model"
)

const paymentColumns = "id, invoice_id, payment_code, payment_method, amount, paid_at, status, proof_image, customer_note, admin_note, verified_by, verified_at, created_at, updated_at"

type MySQLPaymentStore struct{}

func NewMySQLPaymentStore() *MySQLPaymentStore { return &MySQLPaymentStore{} }

// FindInvoiceForUpdate locks the invoice row and loads its booking (upload flow).
func (r *MySQLPaymentStore) FindInvoiceForUpdate(ctx context.Context, q Queryer, invoiceID uint64) (*model.Invoice, error) {
	row := q.QueryRowContext(ctx,
		"SELECT "+invoiceColumns+" FROM invoices WHERE id = ? FOR UPDATE", invoiceID)
	inv, err := scanInvoiceRow(row)
	if err != nil {
		return nil, err
	}
	brow := q.QueryRowContext(ctx,
		"SELECT "+bookingColumns+" FROM bookings WHERE id = ? FOR UPDATE", inv.BookingID)
	b, err := scanBookingRow(brow)
	if err != nil {
		return nil, err
	}
	inv.Booking = b
	return inv, nil
}

func (r *MySQLPaymentStore) HasPendingPayment(ctx context.Context, q Queryer, invoiceID uint64) (bool, error) {
	var n int
	err := q.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM payments WHERE invoice_id = ? AND status IN (?, ?)",
		invoiceID, model.PaymentStatusWaitingVerification, model.PaymentStatusPending).Scan(&n)
	return n > 0, err
}

func (r *MySQLPaymentStore) Create(ctx context.Context, q Queryer, p *model.Payment) error {
	res, err := q.ExecContext(ctx,
		`INSERT INTO payments (invoice_id, payment_code, payment_method, amount, paid_at,
		  status, proof_image, customer_note, admin_note, verified_by, verified_at, created_at, updated_at)
		 VALUES (?,?,?,?,?,?,?,?,NULL,NULL,NULL,NOW(),NOW())`,
		p.InvoiceID, p.PaymentCode, p.PaymentMethod, fmtCents(p.AmountCents), p.PaidAt,
		p.Status, p.ProofImage, p.CustomerNote)
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
	p.ID = uint64(id)
	return nil
}

func (r *MySQLPaymentStore) UpdateInvoiceStatus(ctx context.Context, q Queryer, invoiceID uint64, status string) error {
	_, err := q.ExecContext(ctx,
		"UPDATE invoices SET status = ?, updated_at = NOW() WHERE id = ?", status, invoiceID)
	return err
}

func (r *MySQLPaymentStore) UpdateBookingStatus(ctx context.Context, q Queryer, bookingID uint64, status string) error {
	_, err := q.ExecContext(ctx,
		"UPDATE bookings SET status = ?, updated_at = NOW() WHERE id = ?", status, bookingID)
	return err
}

func (r *MySQLPaymentStore) PaymentCodeExists(ctx context.Context, q Queryer, code string) (bool, error) {
	var n int
	err := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM payments WHERE payment_code = ?", code).Scan(&n)
	return n > 0, err
}

func (r *MySQLPaymentStore) FindLatestWithProofByInvoice(ctx context.Context, q Queryer, invoiceID uint64) (*model.Payment, error) {
	row := q.QueryRowContext(ctx,
		"SELECT "+paymentColumns+` FROM payments
		 WHERE invoice_id = ? AND proof_image IS NOT NULL
		 ORDER BY id DESC LIMIT 1`, invoiceID)
	return scanPaymentRow(row)
}

func (r *MySQLPaymentStore) AdminCount(ctx context.Context, q Queryer, status string) (int, error) {
	query := "SELECT COUNT(*) FROM payments"
	args := []any{}
	if status != "" {
		query += " WHERE status = ?"
		args = append(args, status)
	}
	var n int
	err := q.QueryRowContext(ctx, query, args...).Scan(&n)
	return n, err
}

func (r *MySQLPaymentStore) AdminList(ctx context.Context, q Queryer, status string, limit, offset int) ([]*model.Payment, error) {
	query := "SELECT " + paymentColumns + " FROM payments"
	args := []any{}
	if status != "" {
		query += " WHERE status = ?"
		args = append(args, status)
	}
	query += " ORDER BY id DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out, err := scanPayments(rows)
	if err != nil {
		return nil, err
	}
	if err := r.AttachInvoices(ctx, q, out); err != nil {
		return nil, err
	}
	return out, nil
}

// FindByIDNoLock reads a payment with its invoice (+booking) attached, without
// acquiring row locks (reload for responses).
func (r *MySQLPaymentStore) FindByIDNoLock(ctx context.Context, q Queryer, id uint64) (*model.Payment, error) {
	row := q.QueryRowContext(ctx, "SELECT "+paymentColumns+" FROM payments WHERE id = ?", id)
	p, err := scanPaymentRow(row)
	if err != nil {
		return nil, err
	}
	if err := r.AttachInvoices(ctx, q, []*model.Payment{p}); err != nil {
		return nil, err
	}
	return p, nil
}

func (r *MySQLPaymentStore) FindByIDForUpdate(ctx context.Context, q Queryer, id uint64) (*model.Payment, error) {
	row := q.QueryRowContext(ctx, "SELECT "+paymentColumns+" FROM payments WHERE id = ? FOR UPDATE", id)
	p, err := scanPaymentRow(row)
	if err != nil {
		return nil, err
	}
	// lock the invoice + booking too (consistent order: payment → invoice → booking)
	irow := q.QueryRowContext(ctx, "SELECT "+invoiceColumns+" FROM invoices WHERE id = ? FOR UPDATE", p.InvoiceID)
	inv, err := scanInvoiceRow(irow)
	if err != nil {
		return nil, err
	}
	brow := q.QueryRowContext(ctx, "SELECT "+bookingColumns+" FROM bookings WHERE id = ? FOR UPDATE", inv.BookingID)
	b, err := scanBookingRow(brow)
	if err != nil {
		return nil, err
	}
	inv.Booking = b
	p.Invoice = inv
	return p, nil
}

func (r *MySQLPaymentStore) MarkVerified(ctx context.Context, q Queryer, id, verifiedBy uint64) error {
	_, err := q.ExecContext(ctx,
		`UPDATE payments SET status = ?, paid_at = NOW(), verified_by = ?, verified_at = NOW(), updated_at = NOW()
		 WHERE id = ?`, model.PaymentStatusPaid, verifiedBy, id)
	return err
}

func (r *MySQLPaymentStore) MarkRejected(ctx context.Context, q Queryer, id, verifiedBy uint64, adminNote string) error {
	_, err := q.ExecContext(ctx,
		`UPDATE payments SET status = ?, paid_at = NULL, verified_by = ?, verified_at = NOW(),
		  admin_note = ?, updated_at = NOW() WHERE id = ?`,
		model.PaymentStatusRejected, verifiedBy, adminNote, id)
	return err
}

// AttachInvoices batch-loads invoices (+bookings for ownership) for payments.
func (r *MySQLPaymentStore) AttachInvoices(ctx context.Context, q Queryer, payments []*model.Payment) error {
	if len(payments) == 0 {
		return nil
	}
	ids := make([]uint64, 0, len(payments))
	seen := map[uint64]bool{}
	for _, p := range payments {
		if !seen[p.InvoiceID] {
			seen[p.InvoiceID] = true
			ids = append(ids, p.InvoiceID)
		}
	}
	rows, err := q.QueryContext(ctx,
		"SELECT "+invoiceColumns+" FROM invoices WHERE id IN ("+placeholdersFor(ids)+")", argsOf(ids)...)
	if err != nil {
		return err
	}
	defer rows.Close()
	byID := map[uint64]*model.Invoice{}
	bookingIDs := []uint64{}
	for rows.Next() {
		inv, err := scanInvoiceCols(rows)
		if err != nil {
			return err
		}
		byID[inv.ID] = inv
		bookingIDs = append(bookingIDs, inv.BookingID)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	// bookings for ownership checks
	if len(bookingIDs) > 0 {
		brows, err := q.QueryContext(ctx,
			"SELECT "+bookingColumns+" FROM bookings WHERE id IN ("+placeholdersFor(bookingIDs)+")", argsOf(bookingIDs)...)
		if err != nil {
			return err
		}
		defer brows.Close()
		bByID := map[uint64]*model.Booking{}
		for brows.Next() {
			b, err := scanBookingCols(brows)
			if err != nil {
				return err
			}
			bByID[b.ID] = b
		}
		if err := brows.Err(); err != nil {
			return err
		}
		for _, inv := range byID {
			inv.Booking = bByID[inv.BookingID]
		}
	}
	for _, p := range payments {
		p.Invoice = byID[p.InvoiceID]
	}
	return nil
}

func scanPayments(rows *sql.Rows) ([]*model.Payment, error) {
	var out []*model.Payment
	for rows.Next() {
		p, err := scanPaymentCols(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func scanPaymentRow(s rowScanner) (*model.Payment, error) {
	p, err := scanPaymentCols(s)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return p, nil
}

func scanPaymentCols(s rowScanner) (*model.Payment, error) {
	p := &model.Payment{}
	var amount string
	var paidAt, verifiedAt, ca, ua sql.NullTime
	var proof, custNote, adminNote sql.NullString
	var verifiedBy sql.NullInt64
	if err := s.Scan(&p.ID, &p.InvoiceID, &p.PaymentCode, &p.PaymentMethod, &amount,
		&paidAt, &p.Status, &proof, &custNote, &adminNote, &verifiedBy, &verifiedAt, &ca, &ua); err != nil {
		return nil, err
	}
	cents, err := parsePriceString(amount)
	if err != nil {
		return nil, err
	}
	p.AmountCents = cents
	p.PaidAt = nullTimePtr(paidAt)
	p.ProofImage = nullStringPtr(proof)
	p.CustomerNote = nullStringPtr(custNote)
	p.AdminNote = nullStringPtr(adminNote)
	p.VerifiedBy = uint64Ptr(verifiedBy)
	p.VerifiedAt = nullTimePtr(verifiedAt)
	p.CreatedAt = nullTimePtr(ca)
	p.UpdatedAt = nullTimePtr(ua)
	return p, nil
}
