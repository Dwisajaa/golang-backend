package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Dwisajaa/golang-backend/internal/model"
)

const assignmentColumns = "id, booking_id, technician_id, assigned_by, assigned_at, accepted_at, rejected_at, started_at, completed_at, status, rejection_reason, technician_note, admin_verification_note, created_at, updated_at"

type MySQLAssignmentStore struct{}

func NewMySQLAssignmentStore() *MySQLAssignmentStore { return &MySQLAssignmentStore{} }

func (r *MySQLAssignmentStore) FindBookingForAssign(ctx context.Context, q Queryer, bookingID uint64) (*model.Booking, error) {
	row := q.QueryRowContext(ctx,
		"SELECT "+bookingColumns+" FROM bookings WHERE id = ? FOR UPDATE", bookingID)
	b, err := scanBookingRow(row)
	if err != nil {
		return nil, err
	}
	irow := q.QueryRowContext(ctx,
		"SELECT "+invoiceColumns+" FROM invoices WHERE booking_id = ?", bookingID)
	inv, err := scanInvoiceRow(irow)
	if err == nil {
		b.Invoice = inv
	} else if !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	return b, nil
}

func (r *MySQLAssignmentStore) FindTechnicianForAssign(ctx context.Context, q Queryer, technicianID uint64) (*TechnicianUser, error) {
	row := q.QueryRowContext(ctx,
		"SELECT "+userColumns+" FROM users WHERE id = ? FOR UPDATE", technicianID)
	u, err := scanUser(row)
	if err != nil {
		return nil, err
	}
	p, err := r.technicianProfile(ctx, q, technicianID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	return &TechnicianUser{User: u, TechnicianProfile: p}, nil
}

func (r *MySQLAssignmentStore) technicianProfile(ctx context.Context, q Queryer, userID uint64) (*model.TechnicianProfile, error) {
	row := q.QueryRowContext(ctx,
		"SELECT "+technicianProfileColumns+" FROM technician_profiles WHERE user_id = ?", userID)
	p := &model.TechnicianProfile{}
	var phone, specialization, address, bio sql.NullString
	var ca, ua sql.NullTime
	if err := row.Scan(&p.ID, &p.UserID, &p.TechnicianCode, &phone, &specialization, &address, &bio, &p.IsActive, &ca, &ua); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	p.Phone = nullStringPtr(phone)
	p.Specialization = nullStringPtr(specialization)
	p.Address = nullStringPtr(address)
	p.Bio = nullStringPtr(bio)
	p.CreatedAt = nullTimePtr(ca)
	p.UpdatedAt = nullTimePtr(ua)
	return p, nil
}

func (r *MySQLAssignmentStore) FindActiveAssignment(ctx context.Context, q Queryer, bookingID uint64) (*model.BookingAssignment, error) {
	row := q.QueryRowContext(ctx,
		"SELECT "+assignmentColumns+` FROM booking_assignments
		 WHERE booking_id = ? AND status IN (?, ?)
		 ORDER BY id DESC LIMIT 1`,
		bookingID, model.AssignmentStatusPending, model.AssignmentStatusAccepted)
	return scanAssignmentRow(row)
}

func (r *MySQLAssignmentStore) ReplaceAssignment(ctx context.Context, q Queryer, id uint64, rejectedAt time.Time, reason string) error {
	_, err := q.ExecContext(ctx,
		"UPDATE booking_assignments SET status = ?, rejected_at = ?, rejection_reason = ?, updated_at = NOW() WHERE id = ?",
		model.AssignmentStatusRejected, rejectedAt, reason, id)
	return err
}

func (r *MySQLAssignmentStore) Create(ctx context.Context, q Queryer, a *model.BookingAssignment) error {
	res, err := q.ExecContext(ctx,
		`INSERT INTO booking_assignments
		   (booking_id, technician_id, assigned_by, assigned_at, accepted_at, rejected_at,
		    started_at, completed_at, status, rejection_reason, technician_note,
		    admin_verification_note, created_at, updated_at)
		 VALUES (?,?,?,?,NULL,NULL,NULL,NULL,?,NULL,NULL,NULL,NOW(),NOW())`,
		a.BookingID, a.TechnicianID, a.AssignedBy, a.AssignedAt, a.Status)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	a.ID = uint64(id)
	return nil
}

func (r *MySQLAssignmentStore) UpdateBookingStatus(ctx context.Context, q Queryer, bookingID uint64, status string) error {
	_, err := q.ExecContext(ctx,
		"UPDATE bookings SET status = ?, updated_at = NOW() WHERE id = ?", status, bookingID)
	return err
}

// LoadBookingForResponse loads booking + items + customer (batch, no N+1).
func (r *MySQLAssignmentStore) LoadBookingForResponse(ctx context.Context, q Queryer, bookingID uint64) (*model.Booking, error) {
	row := q.QueryRowContext(ctx, "SELECT "+bookingColumns+" FROM bookings WHERE id = ?", bookingID)
	b, err := scanBookingRow(row)
	if err != nil {
		return nil, err
	}
	// items
	items, err := q.QueryContext(ctx,
		`SELECT id, booking_id, service_id, package_id, item_type, item_name,
		        quantity, unit_price, subtotal, created_at, updated_at
		 FROM booking_items WHERE booking_id = ? ORDER BY id`, bookingID)
	if err != nil {
		return nil, err
	}
	defer items.Close()
	for items.Next() {
		bi := &model.BookingItem{}
		var svcID, pkgID sql.NullInt64
		var up, st string
		var ca, ua sql.NullTime
		if err := items.Scan(&bi.ID, &bi.BookingID, &svcID, &pkgID, &bi.ItemType, &bi.ItemName,
			&bi.Quantity, &up, &st, &ca, &ua); err != nil {
			return nil, err
		}
		bi.ServiceID = uint64Ptr(svcID)
		bi.PackageID = uint64Ptr(pkgID)
		bi.UnitPriceCents, _ = parsePriceString(up)
		bi.SubtotalCents, _ = parsePriceString(st)
		bi.CreatedAt = nullTimePtr(ca)
		bi.UpdatedAt = nullTimePtr(ua)
		b.Items = append(b.Items, bi)
	}
	if err := items.Err(); err != nil {
		return nil, err
	}
	// customer
	crow := q.QueryRowContext(ctx, "SELECT id, name, email FROM users WHERE id = ?", b.CustomerID)
	var usr model.User
	if err := crow.Scan(&usr.ID, &usr.Name, &usr.Email); err == nil {
		b.Customer = &usr
	}
	return b, nil
}

func scanAssignmentRow(s rowScanner) (*model.BookingAssignment, error) {
	a := &model.BookingAssignment{}
	var assignedBy sql.NullInt64
	var assignedAt, acceptedAt, rejectedAt, startedAt, completedAt, ca, ua sql.NullTime
	var rejectionReason, techNote, adminNote sql.NullString
	if err := s.Scan(
		&a.ID, &a.BookingID, &a.TechnicianID, &assignedBy,
		&assignedAt, &acceptedAt, &rejectedAt, &startedAt, &completedAt,
		&a.Status, &rejectionReason, &techNote, &adminNote, &ca, &ua,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	a.AssignedBy = uint64Ptr(assignedBy)
	a.AssignedAt = nullTimePtr(assignedAt)
	a.AcceptedAt = nullTimePtr(acceptedAt)
	a.RejectedAt = nullTimePtr(rejectedAt)
	a.StartedAt = nullTimePtr(startedAt)
	a.CompletedAt = nullTimePtr(completedAt)
	a.RejectionReason = nullStringPtr(rejectionReason)
	a.TechnicianNote = nullStringPtr(techNote)
	a.AdminVerificationNote = nullStringPtr(adminNote)
	a.CreatedAt = nullTimePtr(ca)
	a.UpdatedAt = nullTimePtr(ua)
	return a, nil
}
