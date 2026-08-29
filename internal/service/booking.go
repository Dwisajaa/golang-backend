package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/Dwisajaa/golang-backend/internal/httperr"
	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/repository"
)

type bookingStore interface {
	ListByCustomer(ctx context.Context, q repository.Queryer, customerID uint64, limit, offset int) ([]*model.Booking, error)
	CountByCustomer(ctx context.Context, q repository.Queryer, customerID uint64) (int, error)
	FindByID(ctx context.Context, q repository.Queryer, id uint64) (*model.Booking, error)
	FindByIDForUpdate(ctx context.Context, q repository.Queryer, id uint64) (*model.Booking, error)
	Create(ctx context.Context, q repository.Queryer, b *model.Booking) error
	CreateItem(ctx context.Context, q repository.Queryer, item *model.BookingItem) error
	CreateInvoice(ctx context.Context, q repository.Queryer, inv *model.Invoice) error
	UpdateStatus(ctx context.Context, q repository.Queryer, id uint64, status string) error
	UpdateInvoiceStatus(ctx context.Context, q repository.Queryer, bookingID uint64, status string) error
	AdminCount(ctx context.Context, q repository.Queryer, f repository.AdminBookingFilters) (int, error)
	AdminList(ctx context.Context, q repository.Queryer, f repository.AdminBookingFilters, limit, offset int) ([]*model.Booking, error)
	AttachItems(ctx context.Context, q repository.Queryer, bookings []*model.Booking) error
	AttachInvoices(ctx context.Context, q repository.Queryer, bookings []*model.Booking) error
	BookingCodeExists(ctx context.Context, q repository.Queryer, code string) (bool, error)
	InvoiceNumberExists(ctx context.Context, q repository.Queryer, number string) (bool, error)

	// Booking verify/completion (FASE 13)
	FindLatestAssignmentByStatus(ctx context.Context, q repository.Queryer, bookingID uint64, status string) (*model.BookingAssignment, error)
	LockAssignmentForUpdate(ctx context.Context, q repository.Queryer, id uint64) (*model.BookingAssignment, error)
	UpdateAssignmentVerifiedNote(ctx context.Context, q repository.Queryer, id uint64, note string) error
	RevertAssignmentCompleted(ctx context.Context, q repository.Queryer, id uint64, note string) error
	LoadBookingFull(ctx context.Context, q repository.Queryer, id uint64) (*model.Booking, error)
}

type catalogChecker interface {
	FindActiveServiceForUpdate(ctx context.Context, q repository.Queryer, id uint64) (name string, priceCents int64, err error)
	FindActivePackageForUpdate(ctx context.Context, q repository.Queryer, id uint64) (name string, priceCents int64, err error)
}

type profileChecker interface {
	IsProfileComplete(ctx context.Context, userID uint64) (bool, error)
}

// BookingService owns customer booking CRUD + admin list.
type BookingService struct {
	bookings bookingStore
	catalog  catalogChecker
	profile  profileChecker
	tx       txRunner
}

func NewBookingService(bookings bookingStore, catalog catalogChecker, profile profileChecker, tx txRunner) *BookingService {
	return &BookingService{bookings: bookings, catalog: catalog, profile: profile, tx: tx}
}

// BookingList is the paginated result.
type BookingList struct {
	Items   []*model.Booking
	Total   int
	Page    int
	PerPage int
}

// CreateBookingInput mirrors StoreBookingRequest.
type CreateBookingInput struct {
	ItemType          string // "service" or "package"
	ServiceID         uint64
	PackageID         uint64
	Quantity          int
	BookingDate       string // "2006-01-02"
	BookingTime       string
	Address           string
	AddressDetail     string
	Latitude          string
	Longitude         string
	CustomerNote      string
	AdditionalJobdesk string
}

// ListByCustomer mirrors BookingController@index.
func (s *BookingService) ListByCustomer(ctx context.Context, userID uint64, page, perPage int) (*BookingList, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 15
	}
	if perPage > 50 {
		perPage = 50
	}
	var out BookingList
	out.Page, out.PerPage = page, perPage
	err := s.tx.Within(ctx, func(tx *sql.Tx) error {
		total, err := s.bookings.CountByCustomer(ctx, tx, userID)
		if err != nil {
			return err
		}
		items, err := s.bookings.ListByCustomer(ctx, tx, userID, perPage, (page-1)*perPage)
		if err != nil {
			return err
		}
		if err := s.bookings.AttachItems(ctx, tx, items); err != nil {
			return err
		}
		if err := s.bookings.AttachInvoices(ctx, tx, items); err != nil {
			return err
		}
		out.Total, out.Items = total, items
		return nil
	})
	if err != nil {
		return nil, httperr.Internal(err)
	}
	return &out, nil
}

// Show mirrors BookingController@show with ownership check.
func (s *BookingService) Show(ctx context.Context, userID, bookingID uint64) (*model.Booking, error) {
	var out *model.Booking
	err := s.tx.Within(ctx, func(tx *sql.Tx) error {
		b, err := s.bookings.FindByID(ctx, tx, bookingID)
		if err != nil {
			return err
		}
		if b.CustomerID != userID {
			return errForbidden
		}
		if err := s.bookings.AttachItems(ctx, tx, []*model.Booking{b}); err != nil {
			return err
		}
		if err := s.bookings.AttachInvoices(ctx, tx, []*model.Booking{b}); err != nil {
			return err
		}
		out = b
		return nil
	})
	if err != nil {
		return nil, mapBookingErr(err)
	}
	return out, nil
}

// Create mirrors BookingController@store: validate profile, lock catalog row,
// snapshot price, create booking+item+invoice atomically.
func (s *BookingService) Create(ctx context.Context, userID uint64, in CreateBookingInput) (*model.Booking, error) {
	complete, err := s.profile.IsProfileComplete(ctx, userID)
	if err != nil {
		return nil, httperr.Internal(err)
	}
	if !complete {
		return nil, httperr.Validation(map[string][]string{
			"profile": {"Complete your customer profile before creating a booking."},
		})
	}

	var out *model.Booking
	err = s.tx.Within(ctx, func(tx *sql.Tx) error {
		var itemName string
		var unitPriceCents int64
		if in.ItemType == "service" {
			name, price, err := s.catalog.FindActiveServiceForUpdate(ctx, tx, in.ServiceID)
			if err != nil {
				return err
			}
			itemName, unitPriceCents = name, price
		} else {
			name, price, err := s.catalog.FindActivePackageForUpdate(ctx, tx, in.PackageID)
			if err != nil {
				return err
			}
			itemName, unitPriceCents = name, price
		}
		subtotalCents := unitPriceCents * int64(in.Quantity)

		code, err := s.generateBookingCode(ctx, tx)
		if err != nil {
			return err
		}

		now := time.Now().UTC()
		booking := &model.Booking{
			BookingCode:         code,
			CustomerID:          userID,
			BookingDate:         in.BookingDate,
			BookingTime:         in.BookingTime,
			Address:             in.Address,
			AddressDetail:       nullIfEmpty(in.AddressDetail),
			Latitude:            nullIfEmpty(in.Latitude),
			Longitude:           nullIfEmpty(in.Longitude),
			CustomerNote:        nullIfEmpty(in.CustomerNote),
			AdditionalJobdesk:   nullIfEmpty(in.AdditionalJobdesk),
			SubtotalCents:       subtotalCents,
			AdditionalCostCents: 0,
			TotalPriceCents:     subtotalCents,
			Status:              model.BookingStatusPendingPayment,
			CreatedAt:           &now,
			UpdatedAt:           &now,
		}
		if err := s.bookings.Create(ctx, tx, booking); err != nil {
			return err
		}

		var svcID, pkgID *uint64
		if in.ItemType == "service" {
			svcID = &in.ServiceID
		} else {
			pkgID = &in.PackageID
		}
		item := &model.BookingItem{
			BookingID:      booking.ID,
			ServiceID:      svcID,
			PackageID:      pkgID,
			ItemType:       in.ItemType,
			ItemName:       itemName,
			Quantity:       in.Quantity,
			UnitPriceCents: unitPriceCents,
			SubtotalCents:  subtotalCents,
		}
		if err := s.bookings.CreateItem(ctx, tx, item); err != nil {
			return err
		}

		invNumber, err := s.generateInvoiceNumber(ctx, tx, code)
		if err != nil {
			return err
		}
		dueAt := now.AddDate(0, 0, 7)
		inv := &model.Invoice{
			BookingID:           booking.ID,
			InvoiceNumber:       invNumber,
			IssuedAt:            &now,
			DueAt:               &dueAt,
			SubtotalCents:       subtotalCents,
			AdditionalCostCents: 0,
			TotalAmountCents:    subtotalCents,
			Status:              model.InvoiceStatusUnpaid,
		}
		if err := s.bookings.CreateInvoice(ctx, tx, inv); err != nil {
			return err
		}

		booking.Items = []*model.BookingItem{item}
		booking.Invoice = inv
		out = booking
		return nil
	})
	if err != nil {
		return nil, mapBookingErr(err)
	}
	// Notification side-effect: DEFERRED (NotificationService not wired)
	return out, nil
}

// Cancel mirrors BookingController@cancel: ownership + state check + tx.
func (s *BookingService) Cancel(ctx context.Context, userID, bookingID uint64) (*model.Booking, error) {
	var out *model.Booking
	err := s.tx.Within(ctx, func(tx *sql.Tx) error {
		b, err := s.bookings.FindByIDForUpdate(ctx, tx, bookingID)
		if err != nil {
			return err
		}
		if b.CustomerID != userID {
			return errForbidden
		}
		if !model.CanTransition(b.Status, model.BookingStatusCancelled) {
			return httperr.Conflict("Booking cannot be cancelled in its current status.")
		}
		if err := s.bookings.UpdateStatus(ctx, tx, bookingID, model.BookingStatusCancelled); err != nil {
			return err
		}
		if err := s.bookings.UpdateInvoiceStatus(ctx, tx, bookingID, model.InvoiceStatusCancelled); err != nil {
			return err
		}
		b.Status = model.BookingStatusCancelled
		if err := s.bookings.AttachItems(ctx, tx, []*model.Booking{b}); err != nil {
			return err
		}
		if err := s.bookings.AttachInvoices(ctx, tx, []*model.Booking{b}); err != nil {
			return err
		}
		out = b
		return nil
	})
	if err != nil {
		return nil, mapBookingErr(err)
	}
	return out, nil
}

// AdminList mirrors Admin BookingController@index.
func (s *BookingService) AdminList(ctx context.Context, filters repository.AdminBookingFilters, page, perPage int) (*BookingList, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 15
	}
	if perPage > 50 {
		perPage = 50
	}
	var out BookingList
	out.Page, out.PerPage = page, perPage
	err := s.tx.Within(ctx, func(tx *sql.Tx) error {
		total, err := s.bookings.AdminCount(ctx, tx, filters)
		if err != nil {
			return err
		}
		items, err := s.bookings.AdminList(ctx, tx, filters, perPage, (page-1)*perPage)
		if err != nil {
			return err
		}
		if err := s.bookings.AttachItems(ctx, tx, items); err != nil {
			return err
		}
		if err := s.bookings.AttachInvoices(ctx, tx, items); err != nil {
			return err
		}
		out.Total, out.Items = total, items
		return nil
	})
	if err != nil {
		return nil, httperr.Internal(err)
	}
	return &out, nil
}

// VerifyCompletion mirrors Admin BookingController@verify: admin approves or
// rejects a completed technician job, cascading booking + assignment states.
func (s *BookingService) VerifyCompletion(ctx context.Context, bookingID uint64, action, note string) (*model.Booking, error) {
	var full *model.Booking
	err := s.tx.Within(ctx, func(tx *sql.Tx) error {
		// Lock order: booking FOR UPDATE → assignment FOR UPDATE (canonical,
		// no cycle with workflow/assign which lock assignment/booking alone).
		b, err := s.bookings.FindByIDForUpdate(ctx, tx, bookingID)
		if err != nil {
			return err
		}
		if b.Status != model.BookingStatusAwaitingVerification {
			return httperr.Validation(map[string][]string{"booking": {"Booking must be awaiting verification."}})
		}
		asg, err := s.bookings.FindLatestAssignmentByStatus(ctx, tx, bookingID, model.AssignmentStatusCompleted)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return httperr.Validation(map[string][]string{"booking": {"No completed assignment is waiting for verification."}})
			}
			return err
		}
		if _, err := s.bookings.LockAssignmentForUpdate(ctx, tx, asg.ID); err != nil {
			return err
		}

		if action == "approve" {
			if !model.CanTransition(model.BookingStatusAwaitingVerification, model.BookingStatusCompleted) {
				return httperr.Validation(map[string][]string{"booking": {"Invalid booking status transition."}})
			}
			if err := s.bookings.UpdateStatus(ctx, tx, bookingID, model.BookingStatusCompleted); err != nil {
				return err
			}
			if err := s.bookings.UpdateAssignmentVerifiedNote(ctx, tx, asg.ID, note); err != nil {
				return err
			}
		} else {
			if !model.CanTransition(model.BookingStatusAwaitingVerification, model.BookingStatusInProgress) {
				return httperr.Validation(map[string][]string{"booking": {"Invalid booking status transition."}})
			}
			if err := s.bookings.UpdateStatus(ctx, tx, bookingID, model.BookingStatusInProgress); err != nil {
				return err
			}
			if err := s.bookings.RevertAssignmentCompleted(ctx, tx, asg.ID, note); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, mapBookingErr(err)
	}
	// Post-commit response envelope (customer + items + invoice + assignments).
	_ = s.tx.Within(ctx, func(tx *sql.Tx) error {
		full, err = s.bookings.LoadBookingFull(ctx, tx, bookingID)
		return err
	})
	// Notifications (verified / rejected / review reminder): DEFERRED.
	return full, nil
}

func (s *BookingService) generateBookingCode(ctx context.Context, tx *sql.Tx) (string, error) {
	for attempt := 0; attempt < 5; attempt++ {
		n, _ := rand.Int(rand.Reader, big.NewInt(9999))
		code := fmt.Sprintf("%s%s-%04d", model.BookingCodePrefix, time.Now().UTC().Format("20060102"), n.Int64()+1)
		exists, err := s.bookings.BookingCodeExists(ctx, tx, code)
		if err != nil {
			return "", err
		}
		if !exists {
			return code, nil
		}
	}
	return "", errors.New("booking code generation: too many collisions")
}

func (s *BookingService) generateInvoiceNumber(ctx context.Context, tx *sql.Tx, bookingCode string) (string, error) {
	for attempt := 0; attempt < 5; attempt++ {
		n, _ := rand.Int(rand.Reader, big.NewInt(9999))
		num := fmt.Sprintf("%s%s-%04d", model.InvoiceCodePrefix, bookingCode, n.Int64()+1)
		exists, err := s.bookings.InvoiceNumberExists(ctx, tx, num)
		if err != nil {
			return "", err
		}
		if !exists {
			return num, nil
		}
	}
	return "", errors.New("invoice number generation: too many collisions")
}

var errForbidden = httperr.Forbidden("Forbidden.")

func mapBookingErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, repository.ErrNotFound) {
		return httperr.NotFound("Resource not found.")
	}
	if he := httperr.As(err); he != nil {
		return he
	}
	return httperr.Internal(err)
}
