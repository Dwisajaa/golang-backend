package service

import (
	"context"
	"sync"
	"testing"

	"github.com/Dwisajaa/golang-backend/internal/httperr"
	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/repository"
)

func seedVerify(t *testing.T) *fakeBookingStore {
	t.Helper()
	fs := newFakeBooking()
	fs.byID[1] = &model.Booking{ID: 1, BookingCode: "BJA-1", CustomerID: 7, Status: model.BookingStatusAwaitingVerification}
	return fs
}

func TestVerifyCompletionApprove(t *testing.T) {
	fs := seedVerify(t)
	svc := NewBookingService(fs, &fakeCatalog{}, &fakeProfile{complete: true}, fakeTx{})

	b, err := svc.VerifyCompletion(context.Background(), 1, "approve", "Bagus")
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if fs.byID[1].Status != model.BookingStatusCompleted {
		t.Fatalf("booking should be completed: %s", fs.byID[1].Status)
	}
	if len(b.Assignments) != 1 {
		t.Fatalf("response assignments missing: %+v", b.Assignments)
	}
}

func TestVerifyCompletionReject(t *testing.T) {
	fs := seedVerify(t)
	svc := NewBookingService(fs, &fakeCatalog{}, &fakeProfile{complete: true}, fakeTx{})

	if _, err := svc.VerifyCompletion(context.Background(), 1, "reject", "Perlu perbaikan"); err != nil {
		t.Fatalf("verify reject: %v", err)
	}
	if fs.byID[1].Status != model.BookingStatusInProgress {
		t.Fatalf("booking should return to in_progress: %s", fs.byID[1].Status)
	}
}

func TestVerifyCompletionWrongState(t *testing.T) {
	fs := seedVerify(t)
	fs.byID[1].Status = model.BookingStatusConfirmed
	svc := NewBookingService(fs, &fakeCatalog{}, &fakeProfile{complete: true}, fakeTx{})

	_, err := svc.VerifyCompletion(context.Background(), 1, "approve", "")
	he := httperr.As(err)
	if he == nil || len(he.Errors["booking"]) == 0 || he.Errors["booking"][0] != "Booking must be awaiting verification." {
		t.Fatalf("expected awaiting-verification 422, got %v", err)
	}
}

func TestVerifyCompletionNoCompletedAssignment(t *testing.T) {
	fs := seedVerify(t)
	fs.latestAssignmentErr = repository.ErrNotFound // force no completed assignment
	svc := NewBookingService(fs, &fakeCatalog{}, &fakeProfile{complete: true}, fakeTx{})

	_, err := svc.VerifyCompletion(context.Background(), 1, "approve", "")
	he := httperr.As(err)
	if he == nil || len(he.Errors["booking"]) == 0 || he.Errors["booking"][0] != "No completed assignment is waiting for verification." {
		t.Fatalf("expected no-assignment 422, got %v", err)
	}
}

func TestVerifyCompletionNotPermittedByRepositoryError(t *testing.T) {
	fs := seedVerify(t)
	fs.err = repository.ErrNotFound // booking fetch fails inside tx
	svc := NewBookingService(fs, &fakeCatalog{}, &fakeProfile{complete: true}, fakeTx{})
	_, err := svc.VerifyCompletion(context.Background(), 1, "approve", "")
	if he := httperr.As(err); he == nil || he.Kind != httperr.KindNotFound {
		t.Fatalf("expected 404, got %v", err)
	}
}

func TestVerifyCompletionRepositoryError(t *testing.T) {
	fs := seedVerify(t)
	fs.err = errVerifyDown{}
	svc := NewBookingService(fs, &fakeCatalog{}, &fakeProfile{complete: true}, fakeTx{})
	_, err := svc.VerifyCompletion(context.Background(), 1, "approve", "")
	if he := httperr.As(err); he == nil || he.Kind != httperr.KindInternal {
		t.Fatalf("expected 500, got %v", err)
	}
}

type errVerifyDown struct{}

func (errVerifyDown) Error() string { return "db down" }

// serVerifyStore delegation of the full bookingStore interface.
func (s *serVerifyStore) CountByCustomer(ctx context.Context, q repository.Queryer, cid uint64) (int, error) {
	return s.inner.CountByCustomer(ctx, q, cid)
}
func (s *serVerifyStore) ListByCustomer(ctx context.Context, q repository.Queryer, cid uint64, l, o int) ([]*model.Booking, error) {
	return s.inner.ListByCustomer(ctx, q, cid, l, o)
}
func (s *serVerifyStore) Create(ctx context.Context, q repository.Queryer, b *model.Booking) error {
	return s.inner.Create(ctx, q, b)
}
func (s *serVerifyStore) CreateItem(ctx context.Context, q repository.Queryer, it *model.BookingItem) error {
	return s.inner.CreateItem(ctx, q, it)
}
func (s *serVerifyStore) CreateInvoice(ctx context.Context, q repository.Queryer, inv *model.Invoice) error {
	return s.inner.CreateInvoice(ctx, q, inv)
}
func (s *serVerifyStore) UpdateStatus(ctx context.Context, q repository.Queryer, id uint64, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inner.UpdateStatus(ctx, q, id, status)
}
func (s *serVerifyStore) UpdateInvoiceStatus(ctx context.Context, q repository.Queryer, bid uint64, status string) error {
	return s.inner.UpdateInvoiceStatus(ctx, q, bid, status)
}
func (s *serVerifyStore) AdminCount(ctx context.Context, q repository.Queryer, f repository.AdminBookingFilters) (int, error) {
	return s.inner.AdminCount(ctx, q, f)
}
func (s *serVerifyStore) AdminList(ctx context.Context, q repository.Queryer, f repository.AdminBookingFilters, l, o int) ([]*model.Booking, error) {
	return s.inner.AdminList(ctx, q, f, l, o)
}
func (s *serVerifyStore) AttachItems(ctx context.Context, q repository.Queryer, bs []*model.Booking) error {
	return s.inner.AttachItems(ctx, q, bs)
}
func (s *serVerifyStore) AttachInvoices(ctx context.Context, q repository.Queryer, bs []*model.Booking) error {
	return s.inner.AttachInvoices(ctx, q, bs)
}
func (s *serVerifyStore) BookingCodeExists(ctx context.Context, q repository.Queryer, c string) (bool, error) {
	return s.inner.BookingCodeExists(ctx, q, c)
}
func (s *serVerifyStore) InvoiceNumberExists(ctx context.Context, q repository.Queryer, n string) (bool, error) {
	return s.inner.InvoiceNumberExists(ctx, q, n)
}
func (s *serVerifyStore) FindLatestAssignmentByStatus(ctx context.Context, q repository.Queryer, bid uint64, status string) (*model.BookingAssignment, error) {
	return s.inner.FindLatestAssignmentByStatus(ctx, q, bid, status)
}
func (s *serVerifyStore) LockAssignmentForUpdate(ctx context.Context, q repository.Queryer, id uint64) (*model.BookingAssignment, error) {
	return s.inner.LockAssignmentForUpdate(ctx, q, id)
}
func (s *serVerifyStore) UpdateAssignmentVerifiedNote(ctx context.Context, q repository.Queryer, id uint64, note string) error {
	return s.inner.UpdateAssignmentVerifiedNote(ctx, q, id, note)
}
func (s *serVerifyStore) RevertAssignmentCompleted(ctx context.Context, q repository.Queryer, id uint64, note string) error {
	return s.inner.RevertAssignmentCompleted(ctx, q, id, note)
}
func (s *serVerifyStore) LoadBookingFull(ctx context.Context, q repository.Queryer, id uint64) (*model.Booking, error) {
	return s.inner.LoadBookingFull(ctx, q, id)
}

func TestVerifyCompletionConcurrentSingleWinner(t *testing.T) {
	inner := seedVerify(t)
	ser := &serVerifyStore{inner: inner, claimed: map[uint64]bool{}}
	svc := NewBookingService(ser, &fakeCatalog{}, &fakeProfile{complete: true}, fakeTx{})

	var wg sync.WaitGroup
	wins := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.VerifyCompletion(context.Background(), 1, "approve", "")
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
		t.Fatalf("expected exactly 1 approve success, got %d", succ)
	}
}

// serVerifyStore emulates the booking FOR UPDATE: the first read gets the
// awaiting_verification state; later readers observe it already completed
// (the state guard then rejects them).
type serVerifyStore struct {
	mu      sync.Mutex
	inner   *fakeBookingStore
	claimed map[uint64]bool
}

func (s *serVerifyStore) FindByIDForUpdate(ctx context.Context, q repository.Queryer, id uint64) (*model.Booking, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := s.inner.FindByID(ctx, q, id)
	if err != nil {
		return nil, err
	}
	if s.claimed[id] {
		cp := *b
		cp.Status = model.BookingStatusCompleted
		return &cp, nil
	}
	s.claimed[id] = true
	return b, nil
}
func (s *serVerifyStore) FindByID(ctx context.Context, q repository.Queryer, id uint64) (*model.Booking, error) {
	return s.inner.FindByID(ctx, q, id)
}
