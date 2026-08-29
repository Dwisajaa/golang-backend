package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/big"

	"github.com/Dwisajaa/golang-backend/internal/httperr"
	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/repository"
	"github.com/Dwisajaa/golang-backend/internal/storage"
)

type paymentStore interface {
	FindInvoiceForUpdate(ctx context.Context, q repository.Queryer, invoiceID uint64) (*model.Invoice, error)
	HasPendingPayment(ctx context.Context, q repository.Queryer, invoiceID uint64) (bool, error)
	Create(ctx context.Context, q repository.Queryer, p *model.Payment) error
	UpdateInvoiceStatus(ctx context.Context, q repository.Queryer, invoiceID uint64, status string) error
	UpdateBookingStatus(ctx context.Context, q repository.Queryer, bookingID uint64, status string) error
	PaymentCodeExists(ctx context.Context, q repository.Queryer, code string) (bool, error)
	FindLatestWithProofByInvoice(ctx context.Context, q repository.Queryer, invoiceID uint64) (*model.Payment, error)
	AdminCount(ctx context.Context, q repository.Queryer, status string) (int, error)
	AdminList(ctx context.Context, q repository.Queryer, status string, limit, offset int) ([]*model.Payment, error)
	FindByIDForUpdate(ctx context.Context, q repository.Queryer, id uint64) (*model.Payment, error)
	FindByIDNoLock(ctx context.Context, q repository.Queryer, id uint64) (*model.Payment, error)
	MarkVerified(ctx context.Context, q repository.Queryer, id, verifiedBy uint64) error
	MarkRejected(ctx context.Context, q repository.Queryer, id, verifiedBy uint64, adminNote string) error
	AttachInvoices(ctx context.Context, q repository.Queryer, payments []*model.Payment) error
}

// PaymentService orchestrates the payment flows: customer upload, customer
// proof download, admin list/proof/verify/reject — with state cascades and
// private storage.
type PaymentService struct {
	payments paymentStore
	storage  storage.Storage
	tx       txRunner
}

func NewPaymentService(pay paymentStore, st storage.Storage, tx txRunner) *PaymentService {
	return &PaymentService{payments: pay, storage: st, tx: tx}
}

// PaymentList is the paginated admin payment list.
type PaymentList struct {
	Items   []*model.Payment
	Total   int
	Page    int
	PerPage int
}

// UploadProofInput mirrors UploadPaymentProofRequest validated fields.
type UploadProofInput struct {
	PaymentMethod string
	AmountCents   int64
	CustomerNote  string
	// ProofFile is the validated image bytes (max 2MB, jpg/png; validated by
	// the handler before reaching the service).
	ProofFile     io.Reader
	ProofFilename string // original filename (used for extension only)
}

// allowedProofExtensions mirrors Laravel mimes:jpg,jpeg,png.
var allowedProofExtensions = map[string]bool{"jpg": true, "jpeg": true, "png": true}

// UploadProof mirrors PaymentController@storeProof.
func (s *PaymentService) UploadProof(ctx context.Context, customerID, invoiceID uint64, in UploadProofInput) (*model.Payment, error) {
	if in.PaymentMethod != model.PaymentMethodBankTransfer {
		return nil, httperr.Unprocessable("Only bank transfer is supported.")
	}

	var proofKey string
	var storedFile *model.Payment

	err := s.tx.Within(ctx, func(tx *sql.Tx) error {
		locked, err := s.payments.FindInvoiceForUpdate(ctx, tx, invoiceID)
		if err != nil {
			return err
		}
		if locked.Booking == nil || locked.Booking.CustomerID != customerID {
			return htForbidden
		}
		if locked.Status != model.InvoiceStatusUnpaid && locked.Status != model.InvoiceStatusPendingPayment {
			return httperr.Conflict("Invoice cannot receive a payment in its current status.")
		}
		if in.AmountCents != locked.TotalAmountCents {
			return httperr.Validation(map[string][]string{"amount": {"Amount must equal the invoice total."}})
		}
		pending, err := s.payments.HasPendingPayment(ctx, tx, invoiceID)
		if err != nil {
			return err
		}
		if pending {
			return httperr.Validation(map[string][]string{"payment": {"A payment is already awaiting verification."}})
		}
		if locked.Status == model.InvoiceStatusCancelled || locked.Status == model.InvoiceStatusExpired {
			return httperr.Validation(map[string][]string{"invoice": {"This invoice cannot be paid."}})
		}
		if locked.Booking.Status != model.BookingStatusPendingPayment {
			return httperr.Validation(map[string][]string{"booking": {"This booking is not accepting payment proof."}})
		}

		code, err := s.generatePaymentCode(ctx, tx, locked.Booking.BookingCode)
		if err != nil {
			return err
		}
		key, err := s.newProofKey(in.ProofFilename)
		if err != nil {
			return err
		}

		p := &model.Payment{
			InvoiceID:     invoiceID,
			PaymentCode:   code,
			PaymentMethod: model.PaymentMethodBankTransfer,
			AmountCents:   locked.TotalAmountCents,
			Status:        model.PaymentStatusWaitingVerification,
			ProofImage:    &key,
			CustomerNote:  nullIfEmpty(in.CustomerNote),
		}
		if err := s.payments.Create(ctx, tx, p); err != nil {
			return err
		}
		if err := s.payments.UpdateInvoiceStatus(ctx, tx, invoiceID, model.InvoiceStatusPendingPayment); err != nil {
			return err
		}
		if err := s.payments.UpdateBookingStatus(ctx, tx, locked.Booking.ID, model.BookingStatusWaitingVerification); err != nil {
			return err
		}
		proofKey = key
		storedFile = p
		return nil
	})
	if err != nil {
		if proofKey != "" {
			_ = s.storage.Delete(proofKey)
		}
		return nil, mapPaymentErr(err)
	}
	if proofKey != "" {
		// post-commit file write; failure keeps the DB row but the proof is
		// missing — matches the observable contract (proof_available=false yet)
		// and avoids holding a DB transaction during I/O.
		if err := s.storage.Save(proofKey, in.ProofFile); err != nil {
			return nil, httperr.Internal(err)
		}
	}
	// Notification to admins: DEFERRED (Notification domain not wired).
	return storedFile, nil
}

// ShowProofByInvoice mirrors PaymentController@showProof (customer download).
func (s *PaymentService) ShowProofByInvoice(ctx context.Context, customerID, invoiceID uint64) (*model.Payment, error) {
	var out *model.Payment
	err := s.tx.Within(ctx, func(tx *sql.Tx) error {
		p, err := s.payments.FindLatestWithProofByInvoice(ctx, tx, invoiceID)
		if err != nil {
			return err
		}
		inv, err := s.paymentsAttachInvoice(ctx, tx, p)
		if err != nil {
			return err
		}
		if inv.Booking == nil || inv.Booking.CustomerID != customerID {
			return htForbidden
		}
		out = p
		return nil
	})
	if err != nil {
		return nil, mapPaymentErr(err)
	}
	if !s.proofUsable(out) {
		return nil, httperr.NotFound("Payment proof not found.")
	}
	return out, nil
}

// ShowProofByID mirrors Admin PaymentController@showProof (admin download,
// policy view = admin OR owning customer).
func (s *PaymentService) ShowProofByID(ctx context.Context, userID uint64, role string, paymentID uint64) (*model.Payment, error) {
	var out *model.Payment
	err := s.tx.Within(ctx, func(tx *sql.Tx) error {
		p, err := s.payments.FindByIDNoLock(ctx, tx, paymentID)
		if err != nil {
			return err
		}
		inv, err := s.paymentsAttachInvoice(ctx, tx, p)
		if err != nil {
			return err
		}
		if role != model.RoleAdmin && role != model.RoleSuperAdmin {
			if inv.Booking == nil || inv.Booking.CustomerID != userID {
				return htForbidden
			}
		}
		out = p
		return nil
	})
	if err != nil {
		return nil, mapPaymentErr(err)
	}
	if !s.proofUsable(out) {
		return nil, httperr.NotFound("Payment proof not found.")
	}
	return out, nil
}

func (s *PaymentService) proofUsable(p *model.Payment) bool {
	if p.ProofImage == nil {
		return false
	}
	ok, err := s.storage.Exists(*p.ProofImage)
	return err == nil && ok
}

// AdminList mirrors Admin PaymentController@index (status filter + latest).
func (s *PaymentService) AdminList(ctx context.Context, status string, page, perPage int) (*PaymentList, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 15
	}
	if perPage > 50 {
		perPage = 50
	}
	var out PaymentList
	out.Page, out.PerPage = page, perPage
	err := s.tx.Within(ctx, func(tx *sql.Tx) error {
		total, err := s.payments.AdminCount(ctx, tx, status)
		if err != nil {
			return err
		}
		items, err := s.payments.AdminList(ctx, tx, status, perPage, (page-1)*perPage)
		if err != nil {
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

// Verify mirrors Admin PaymentController@verify: lock, validate, then cascade
// payment→invoice→booking (paid→confirmed).
func (s *PaymentService) Verify(ctx context.Context, adminID, paymentID uint64) (*model.Payment, error) {
	var out *model.Payment
	err := s.tx.Within(ctx, func(tx *sql.Tx) error {
		locked, err := s.payments.FindByIDForUpdate(ctx, tx, paymentID)
		if err != nil {
			return err
		}
		if !model.IsPaymentPendingVerification(locked.Status) {
			return httperr.Validation(map[string][]string{"payment": {"Payment has already been processed."}})
		}
		if !s.proofExists(locked) {
			return httperr.Validation(map[string][]string{"payment": {"Payment proof is required."}})
		}
		if locked.Invoice == nil || locked.Invoice.Status == model.InvoiceStatusCancelled || locked.Invoice.Status == model.InvoiceStatusExpired {
			return httperr.Validation(map[string][]string{"invoice": {"Invoice cannot be verified."}})
		}
		if err := s.payments.MarkVerified(ctx, tx, paymentID, adminID); err != nil {
			return err
		}
		if err := s.payments.UpdateInvoiceStatus(ctx, tx, locked.InvoiceID, model.InvoiceStatusPaid); err != nil {
			return err
		}
		booking := locked.Invoice.Booking
		if booking != nil {
			switch booking.Status {
			case model.BookingStatusWaitingVerification:
				if err := s.payments.UpdateBookingStatus(ctx, tx, booking.ID, model.BookingStatusPaid); err != nil {
					return err
				}
				fallthrough
			case model.BookingStatusPaid:
				if err := s.payments.UpdateBookingStatus(ctx, tx, booking.ID, model.BookingStatusConfirmed); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, mapPaymentErr(err)
	}
	// reload for response
	_ = s.tx.Within(ctx, func(tx *sql.Tx) error {
		p, ferr := s.payments.FindByIDNoLock(ctx, tx, paymentID)
		if ferr != nil {
			return ferr
		}
		_, ferr = s.paymentsAttachInvoice(ctx, tx, p)
		if ferr != nil {
			return ferr
		}
		out = p
		return nil
	})
	// Notification to customer: DEFERRED.
	return out, nil
}

// Reject mirrors Admin PaymentController@reject.
func (s *PaymentService) Reject(ctx context.Context, adminID, paymentID uint64, adminNote string) (*model.Payment, error) {
	var out *model.Payment
	err := s.tx.Within(ctx, func(tx *sql.Tx) error {
		locked, err := s.payments.FindByIDForUpdate(ctx, tx, paymentID)
		if err != nil {
			return err
		}
		if !model.IsPaymentPendingVerification(locked.Status) {
			return httperr.Validation(map[string][]string{"payment": {"Payment has already been processed."}})
		}
		if locked.Invoice == nil || locked.Invoice.Status == model.InvoiceStatusCancelled || locked.Invoice.Status == model.InvoiceStatusExpired {
			return httperr.Validation(map[string][]string{"invoice": {"Invoice cannot be rejected in its current status."}})
		}
		if err := s.payments.MarkRejected(ctx, tx, paymentID, adminID, adminNote); err != nil {
			return err
		}
		if err := s.payments.UpdateInvoiceStatus(ctx, tx, locked.InvoiceID, model.InvoiceStatusUnpaid); err != nil {
			return err
		}
		if b := locked.Invoice.Booking; b != nil && b.Status == model.BookingStatusWaitingVerification {
			if err := s.payments.UpdateBookingStatus(ctx, tx, b.ID, model.BookingStatusPendingPayment); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, mapPaymentErr(err)
	}
	_ = s.tx.Within(ctx, func(tx *sql.Tx) error {
		p, ferr := s.payments.FindByIDNoLock(ctx, tx, paymentID)
		if ferr != nil {
			return ferr
		}
		_, ferr = s.paymentsAttachInvoice(ctx, tx, p)
		if ferr != nil {
			return ferr
		}
		out = p
		return nil
	})
	// Notification to customer: DEFERRED.
	return out, nil
}

// -- helpers --

// paymentsAttachInvoice loads the invoice (+booking) for a single payment.
func (s *PaymentService) paymentsAttachInvoice(ctx context.Context, q repository.Queryer, p *model.Payment) (*model.Invoice, error) {
	if err := s.payments.AttachInvoices(ctx, q, []*model.Payment{p}); err != nil {
		return nil, err
	}
	return p.Invoice, nil
}

func (s *PaymentService) proofExists(p *model.Payment) bool {
	if p.ProofImage == nil {
		return false
	}
	ok, err := s.storage.Exists(*p.ProofImage)
	return err == nil && ok
}

// newProofKey mirrors Laravel: "payment-proof-{uuid}.{ext}" — uuid replaced by
// crypto/rand hex (no uuid dependency needed).
func (s *PaymentService) newProofKey(filename string) (string, error) {
	ext := ""
	if filename != "" {
		for i := len(filename) - 1; i >= 0; i-- {
			if filename[i] == '.' {
				ext = filename[i+1:]
				break
			}
		}
	}
	if !allowedProofExtensions[ext] {
		return "", httperr.Validation(map[string][]string{"proof_image": {"The proof image must be a file of type: jpg, jpeg, png."}})
	}
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("payment-proof-%s.%s", hex.EncodeToString(b[:]), ext), nil
}

func (s *PaymentService) generatePaymentCode(ctx context.Context, q repository.Queryer, bookingCode string) (string, error) {
	for attempt := 0; attempt < 5; attempt++ {
		n, _ := rand.Int(rand.Reader, big.NewInt(9999))
		code := fmt.Sprintf("%s%s-%04d", model.PaymentCodePrefix, bookingCode, n.Int64()+1)
		exists, err := s.payments.PaymentCodeExists(ctx, q, code)
		if err != nil {
			return "", err
		}
		if !exists {
			return code, nil
		}
	}
	return "", errors.New("payment code generation: too many collisions")
}

// FindByIDNoLock is a read helper on the paymentStore interface (admin detail
// isn't an endpoint; used for reloads). Implemented by the MySQL store.
type findByIDNoLock interface {
	FindByIDNoLock(ctx context.Context, q repository.Queryer, id uint64) (*model.Payment, error)
}

func mapPaymentErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, repository.ErrNotFound) {
		return httperr.NotFound("Resource not found.")
	}
	if errors.Is(err, repository.ErrDuplicate) {
		return httperr.Conflict("Resource conflict.")
	}
	if he := httperr.As(err); he != nil {
		return he
	}
	return httperr.Internal(err)
}
