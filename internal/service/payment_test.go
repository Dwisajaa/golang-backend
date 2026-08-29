package service

import (
	"bytes"
	"context"
	"io"
	"sync"
	"testing"

	"github.com/Dwisajaa/golang-backend/internal/httperr"
	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/repository"
	"github.com/Dwisajaa/golang-backend/internal/storage"
)

type memStorage struct {
	mu sync.Mutex
	m  map[string][]byte
}

func (s *memStorage) Save(key string, r io.Reader) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, _ := io.ReadAll(r)
	s.m[key] = b
	return nil
}
func (s *memStorage) Exists(key string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.m[key]
	return ok, nil
}
func (s *memStorage) Open(key string) (io.ReadCloser, error) { return nil, nil }
func (s *memStorage) Path(key string) (string, error) {
	if _, err := s.Exists(key); err != nil {
		return "", err
	}
	return key, nil
}
func (s *memStorage) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, key)
	return nil
}

var _ storage.Storage = (*memStorage)(nil)

type fakePayStore struct {
	mu            sync.Mutex
	payments      map[uint64]*model.Payment
	invoiceByID   map[uint64]*model.Invoice
	statusChanged []string
	err           error
}

func newFakePay() *fakePayStore {
	return &fakePayStore{
		payments:    map[uint64]*model.Payment{},
		invoiceByID: map[uint64]*model.Invoice{},
	}
}

func (f *fakePayStore) FindInvoiceForUpdate(ctx context.Context, q repository.Queryer, invoiceID uint64) (*model.Invoice, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	inv, ok := f.invoiceByID[invoiceID]
	if !ok {
		return nil, repository.ErrNotFound
	}
	if inv.Booking == nil {
		inv.Booking = &model.Booking{ID: 1, CustomerID: 7, Status: model.BookingStatusPendingPayment, BookingCode: "BJA-X"}
	}
	return inv, nil
}
func (f *fakePayStore) HasPendingPayment(ctx context.Context, q repository.Queryer, invoiceID uint64) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, p := range f.payments {
		if p.InvoiceID == invoiceID && model.IsPaymentPendingVerification(p.Status) {
			return true, nil
		}
	}
	return false, nil
}
func (f *fakePayStore) Create(ctx context.Context, q repository.Queryer, p *model.Payment) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	p.ID = uint64(len(f.payments) + 1)
	f.payments[p.ID] = p
	return nil
}
func (f *fakePayStore) UpdateInvoiceStatus(ctx context.Context, q repository.Queryer, invoiceID uint64, s string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.invoiceByID[invoiceID].Status = s
	return nil
}
func (f *fakePayStore) UpdateBookingStatus(ctx context.Context, q repository.Queryer, bookingID uint64, s string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.statusChanged = append(f.statusChanged, s)
	// mirror the change on the shared booking object so later reads reflect it
	for _, inv := range f.invoiceByID {
		if inv.Booking != nil && inv.Booking.ID == bookingID {
			inv.Booking.Status = s
		}
	}
	return nil
}
func (f *fakePayStore) PaymentCodeExists(ctx context.Context, q repository.Queryer, code string) (bool, error) {
	return false, nil
}
func (f *fakePayStore) FindLatestWithProofByInvoice(ctx context.Context, q repository.Queryer, invoiceID uint64) (*model.Payment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var latest *model.Payment
	for _, p := range f.payments {
		if p.InvoiceID == invoiceID && p.ProofImage != nil {
			if latest == nil || p.ID > latest.ID {
				latest = p
			}
		}
	}
	if latest == nil {
		return nil, repository.ErrNotFound
	}
	return latest, nil
}
func (f *fakePayStore) AdminCount(ctx context.Context, q repository.Queryer, status string) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, p := range f.payments {
		if status == "" || p.Status == status {
			n++
		}
	}
	return n, nil
}
func (f *fakePayStore) AdminList(ctx context.Context, q repository.Queryer, status string, l, o int) ([]*model.Payment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*model.Payment
	for _, p := range f.payments {
		if status == "" || p.Status == status {
			out = append(out, p)
		}
	}
	return out, nil
}
func (f *fakePayStore) FindByIDNoLock(ctx context.Context, q repository.Queryer, id uint64) (*model.Payment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.payments[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	cp := *p
	cp.Invoice = f.invoiceByID[p.InvoiceID]
	return &cp, nil
}
func (f *fakePayStore) FindByIDForUpdate(ctx context.Context, q repository.Queryer, id uint64) (*model.Payment, error) {
	return f.FindByIDNoLock(ctx, q, id)
}
func (f *fakePayStore) MarkVerified(ctx context.Context, q repository.Queryer, id, by uint64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.payments[id].Status = model.PaymentStatusPaid
	return nil
}
func (f *fakePayStore) MarkRejected(ctx context.Context, q repository.Queryer, id, by uint64, note string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	p := f.payments[id]
	p.Status = model.PaymentStatusRejected
	p.AdminNote = &note
	return nil
}
func (f *fakePayStore) AttachInvoices(ctx context.Context, q repository.Queryer, ps []*model.Payment) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, p := range ps {
		p.Invoice = f.invoiceByID[p.InvoiceID]
	}
	return nil
}
func (f *fakePayStore) FindByID(ctx context.Context, q repository.Queryer, id uint64) (*model.Payment, error) {
	return f.FindByIDNoLock(ctx, q, id)
}

func seedInvoice(invoiceID uint64) *model.Invoice {
	return &model.Invoice{
		ID: invoiceID, BookingID: 1, InvoiceNumber: "INV-X-1",
		TotalAmountCents: 30000, Status: model.InvoiceStatusUnpaid,
		Booking: &model.Booking{ID: 1, CustomerID: 7, Status: model.BookingStatusPendingPayment, BookingCode: "BJA-X"},
	}
}

func newPaySvc() (*PaymentService, *fakePayStore, *memStorage) {
	st := &memStorage{m: map[string][]byte{}}
	fs := newFakePay()
	fs.invoiceByID[1] = seedInvoice(1)
	return NewPaymentService(fs, st, fakeTx{}), fs, st
}

func uploadIn() UploadProofInput {
	return UploadProofInput{
		PaymentMethod: model.PaymentMethodBankTransfer, AmountCents: 30000,
		ProofFile: bytes.NewReader([]byte("jpeg-bytes")), ProofFilename: "proof.jpg",
	}
}

func TestUploadSuccess(t *testing.T) {
	svc, _, st := newPaySvc()
	p, err := svc.UploadProof(context.Background(), 7, 1, uploadIn())
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if p.Status != model.PaymentStatusWaitingVerification {
		t.Fatalf("status: %s", p.Status)
	}
	if p.ProofImage == nil {
		t.Fatal("proof key missing")
	}
	if ok, _ := st.Exists(*p.ProofImage); !ok {
		t.Fatalf("proof not stored")
	}
}

func TestUploadWrongOwner(t *testing.T) {
	svc, _, _ := newPaySvc()
	_, err := svc.UploadProof(context.Background(), 99, 1, uploadIn())
	he := httperr.As(err)
	if he == nil || he.Kind != httperr.KindForbidden {
		t.Fatalf("expected 403, got %v", err)
	}
}

func TestUploadWrongAmount(t *testing.T) {
	svc, _, _ := newPaySvc()
	in := uploadIn()
	in.AmountCents = 1
	_, err := svc.UploadProof(context.Background(), 7, 1, in)
	he := httperr.As(err)
	if he == nil || he.Kind != httperr.KindValidation {
		t.Fatalf("expected 422, got %v", err)
	}
}

func TestUploadInvoiceNotPayable(t *testing.T) {
	svc, fs, _ := newPaySvc()
	fs.invoiceByID[1].Status = model.InvoiceStatusPaid
	_, err := svc.UploadProof(context.Background(), 7, 1, uploadIn())
	if he := httperr.As(err); he == nil || he.Kind != httperr.KindConflict {
		t.Fatalf("expected 409, got %v", err)
	}
}

func TestUploadStorageError(t *testing.T) {
	svc, fs, _ := newPaySvc()
	fs.invoiceByID[1] = seedInvoice(1)
	svc = NewPaymentService(fs, &failStorage{}, fakeTx{})
	_, err := svc.UploadProof(context.Background(), 7, 1, uploadIn())
	if httperr.As(err) == nil || httperr.As(err).Kind != httperr.KindInternal {
		t.Fatalf("expected 500 on storage failure, got %v", err)
	}
}

type failStorage struct{ memStorage }

func (f *failStorage) Save(key string, r io.Reader) error { return errStorageDown }
func (f *failStorage) Exists(key string) (bool, error) {
	f.memStorage.mu.Lock()
	defer f.memStorage.mu.Unlock()
	_, ok := f.memStorage.m[key]
	return ok, nil
}

type errStorageType struct{}

func (e errStorageType) Error() string { return "storage down" }

var errStorageDown = errStorageType{}

func TestVerifySuccessCascades(t *testing.T) {
	svc, fs, st := newPaySvc()
	p, _ := svc.UploadProof(context.Background(), 7, 1, uploadIn())

	verified, err := svc.Verify(context.Background(), 2, p.ID)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if verified.Status != model.PaymentStatusPaid {
		t.Fatalf("payment status: %s", verified.Status)
	}
	if fs.invoiceByID[1].Status != model.InvoiceStatusPaid {
		t.Fatalf("invoice not paid")
	}
	// booking cascade running: waiting_verification → paid → confirmed
	changed := fs.statusChanged
	if len(changed) == 0 || changed[len(changed)-1] != model.BookingStatusConfirmed {
		t.Fatalf("booking cascade wrong: %v", changed)
	}
	_ = st
}

func TestVerifyInactivePaymentRejected(t *testing.T) {
	svc, fs, st := newPaySvc()
	p, _ := svc.UploadProof(context.Background(), 7, 1, uploadIn())
	// simulate already-paid
	fs.mu.Lock()
	fs.payments[p.ID].Status = model.PaymentStatusPaid
	fs.mu.Unlock()
	_, err := svc.Verify(context.Background(), 2, p.ID)
	he := httperr.As(err)
	if he == nil || he.Kind != httperr.KindValidation || len(he.Errors["payment"]) == 0 {
		t.Fatalf("expected 422 payment processed, got %v", err)
	}
	_ = st
}

func TestRejectSuccess(t *testing.T) {
	svc, fs, _ := newPaySvc()
	p, _ := svc.UploadProof(context.Background(), 7, 1, uploadIn())
	rejected, err := svc.Reject(context.Background(), 2, p.ID, "Blurred proof")
	if err != nil {
		t.Fatalf("reject: %v", err)
	}
	if rejected.Status != model.PaymentStatusRejected {
		t.Fatalf("status: %s", rejected.Status)
	}
	if rejected.AdminNote == nil || *rejected.AdminNote != "Blurred proof" {
		t.Fatalf("admin note missing")
	}
	if fs.invoiceByID[1].Status != model.InvoiceStatusUnpaid {
		t.Fatalf("invoice should revert to unpaid")
	}
}

func TestConcurrentVerifyOnlyOneWins(t *testing.T) {
	inner := newFakePay()
	inner.invoiceByID[1] = seedInvoice(1)
	st := &memStorage{m: map[string][]byte{}}
	pre := NewPaymentService(inner, st, fakeTx{})
	p, _ := pre.UploadProof(context.Background(), 7, 1, uploadIn())

	// serializer store emulates SELECT ... FOR UPDATE: the first FindByIDForUpdate
	// claims the row, later callers observe the committed outcome (real
	// serialization is proven by the MySQL integration test).
	ser := &serialPayStore{inner: inner, claimed: map[uint64]bool{}}
	svc := NewPaymentService(ser, st, fakeTx{})

	var wg sync.WaitGroup
	wins := make(chan error, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.Verify(context.Background(), 2, p.ID)
			wins <- err
		}()
	}
	wg.Wait()
	close(wins)
	succ := 0
	for err := range wins {
		if err == nil {
			succ++
		}
	}
	if succ != 1 {
		t.Fatalf("expected exactly 1 successful verify, got %d", succ)
	}
}

// serialPayStore makes FindByIDForUpdate an atomic claim, emulating the row
// lock: once a payment has been claimed, later reads see it as paid.
type serialPayStore struct {
	mu      sync.Mutex
	inner   *fakePayStore
	claimed map[uint64]bool
}

func (s *serialPayStore) FindByIDForUpdate(ctx context.Context, q repository.Queryer, id uint64) (*model.Payment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp, err := s.inner.FindByIDNoLock(ctx, q, id)
	if err != nil {
		return nil, err
	}
	if s.claimed[id] {
		cp.Status = model.PaymentStatusPaid // committed winner outcome
		return cp, nil
	}
	s.claimed[id] = true
	return cp, nil
}

func (s *serialPayStore) FindInvoiceForUpdate(ctx context.Context, q repository.Queryer, invoiceID uint64) (*model.Invoice, error) {
	return s.inner.FindInvoiceForUpdate(ctx, q, invoiceID)
}
func (s *serialPayStore) HasPendingPayment(ctx context.Context, q repository.Queryer, invoiceID uint64) (bool, error) {
	return s.inner.HasPendingPayment(ctx, q, invoiceID)
}
func (s *serialPayStore) Create(ctx context.Context, q repository.Queryer, p *model.Payment) error {
	return s.inner.Create(ctx, q, p)
}
func (s *serialPayStore) UpdateInvoiceStatus(ctx context.Context, q repository.Queryer, invoiceID uint64, status string) error {
	return s.inner.UpdateInvoiceStatus(ctx, q, invoiceID, status)
}
func (s *serialPayStore) UpdateBookingStatus(ctx context.Context, q repository.Queryer, bookingID uint64, status string) error {
	return s.inner.UpdateBookingStatus(ctx, q, bookingID, status)
}
func (s *serialPayStore) PaymentCodeExists(ctx context.Context, q repository.Queryer, code string) (bool, error) {
	return s.inner.PaymentCodeExists(ctx, q, code)
}
func (s *serialPayStore) FindLatestWithProofByInvoice(ctx context.Context, q repository.Queryer, invoiceID uint64) (*model.Payment, error) {
	return s.inner.FindLatestWithProofByInvoice(ctx, q, invoiceID)
}
func (s *serialPayStore) AdminCount(ctx context.Context, q repository.Queryer, status string) (int, error) {
	return s.inner.AdminCount(ctx, q, status)
}
func (s *serialPayStore) AdminList(ctx context.Context, q repository.Queryer, status string, limit, offset int) ([]*model.Payment, error) {
	return s.inner.AdminList(ctx, q, status, limit, offset)
}
func (s *serialPayStore) FindByIDNoLock(ctx context.Context, q repository.Queryer, id uint64) (*model.Payment, error) {
	return s.inner.FindByIDNoLock(ctx, q, id)
}
func (s *serialPayStore) MarkVerified(ctx context.Context, q repository.Queryer, id, by uint64) error {
	return s.inner.MarkVerified(ctx, q, id, by)
}
func (s *serialPayStore) MarkRejected(ctx context.Context, q repository.Queryer, id, by uint64, note string) error {
	return s.inner.MarkRejected(ctx, q, id, by, note)
}
func (s *serialPayStore) AttachInvoices(ctx context.Context, q repository.Queryer, ps []*model.Payment) error {
	return s.inner.AttachInvoices(ctx, q, ps)
}
func (s *serialPayStore) FindByID(ctx context.Context, q repository.Queryer, id uint64) (*model.Payment, error) {
	return s.inner.FindByID(ctx, q, id)
}
