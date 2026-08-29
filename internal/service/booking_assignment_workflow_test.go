package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Dwisajaa/golang-backend/internal/httperr"
	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/repository"
)

func seedWork(fs *fakeAssignStore, assignmentID uint64, bookingStatus string, invoiceStatus string) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.bookings[1] = &model.Booking{
		ID: 1, BookingCode: "BJA-1", CustomerID: 7, Status: bookingStatus,
		Invoice: &model.Invoice{ID: 1, BookingID: 1, InvoiceNumber: "INV-1", Status: invoiceStatus},
	}
	a := &model.BookingAssignment{ID: assignmentID, BookingID: 1, TechnicianID: 9, Status: model.AssignmentStatusPending}
	fs.assigns[assignmentID] = a
}

func TestWorkAcceptSuccess(t *testing.T) {
	fs := newFakeAssign()
	seedWork(fs, 5, model.BookingStatusTechnicianAssigned, model.InvoiceStatusPaid)
	svc := NewAssignmentService(fs, fakeTx{})

	a, err := svc.Accept(context.Background(), 9, 5)
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	if a.Status != model.AssignmentStatusAccepted {
		t.Fatalf("status: %s", a.Status)
	}
}

func TestWorkAcceptNotOwned(t *testing.T) {
	fs := newFakeAssign()
	seedWork(fs, 5, model.BookingStatusTechnicianAssigned, model.InvoiceStatusPaid)
	svc := NewAssignmentService(fs, fakeTx{})
	_, err := svc.Accept(context.Background(), 99, 5)
	if he := httperr.As(err); he == nil || he.Kind != httperr.KindForbidden {
		t.Fatalf("expected 403, got %v", err)
	}
}

func TestWorkAcceptWrongState(t *testing.T) {
	fs := newFakeAssign()
	seedWork(fs, 5, model.BookingStatusTechnicianAssigned, model.InvoiceStatusPaid)
	fs.assigns[5].Status = model.AssignmentStatusAccepted
	svc := NewAssignmentService(fs, fakeTx{})
	_, err := svc.Accept(context.Background(), 9, 5)
	he := httperr.As(err)
	if he == nil || len(he.Errors["assignment"]) == 0 || he.Errors["assignment"][0] != "Assignment is not in the required state." {
		t.Fatalf("expected required-state 422, got %v", err)
	}
}

func TestWorkAcceptBookingNotReady(t *testing.T) {
	fs := newFakeAssign()
	seedWork(fs, 5, model.BookingStatusPendingPayment, model.InvoiceStatusUnpaid)
	svc := NewAssignmentService(fs, fakeTx{})
	_, err := svc.Accept(context.Background(), 9, 5)
	he := httperr.As(err)
	if he == nil || he.Errors["assignment"][0] != "Booking is not ready for acceptance." {
		t.Fatalf("expected not-ready 422, got %v", err)
	}
}

func TestWorkRejectSuccessRevertsBooking(t *testing.T) {
	fs := newFakeAssign()
	seedWork(fs, 5, model.BookingStatusTechnicianAssigned, model.InvoiceStatusPaid)
	svc := NewAssignmentService(fs, fakeTx{})

	a, err := svc.Reject(context.Background(), 9, 5, "Jadwal bentrok", "besok pagi")
	if err != nil {
		t.Fatalf("reject: %v", err)
	}
	if a.Status != model.AssignmentStatusRejected {
		t.Fatalf("status: %s", a.Status)
	}
	if a.RejectionReason == nil || *a.RejectionReason != "Jadwal bentrok - besok pagi" {
		t.Fatalf("reason merge wrong: %v", a.RejectionReason)
	}
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if fs.bookings[1].Status != model.BookingStatusConfirmed {
		t.Fatalf("booking should revert to confirmed: %s", fs.bookings[1].Status)
	}
}

func TestWorkRejectInvalidReason(t *testing.T) {
	// service-level: the handler validates allowed reasons; service merges
	fs := newFakeAssign()
	seedWork(fs, 5, model.BookingStatusTechnicianAssigned, model.InvoiceStatusPaid)
	svc := NewAssignmentService(fs, fakeTx{})
	if _, err := svc.Reject(context.Background(), 9, 5, "Bukan alasan", ""); err != nil {
		t.Fatalf("service should accept any reason string; handler gates the enum: %v", err)
	}
}

func TestWorkStartSuccess(t *testing.T) {
	fs := newFakeAssign()
	seedWork(fs, 5, model.BookingStatusTechnicianAssigned, model.InvoiceStatusPaid)
	fs.assigns[5].Status = model.AssignmentStatusAccepted
	svc := NewAssignmentService(fs, fakeTx{})

	a, err := svc.Start(context.Background(), 9, 5)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if a.StartedAt == nil {
		t.Fatal("started_at must be set")
	}
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if fs.bookings[1].Status != model.BookingStatusInProgress {
		t.Fatalf("booking should be in_progress: %s", fs.bookings[1].Status)
	}
}

func TestWorkCompleteSuccess(t *testing.T) {
	fs := newFakeAssign()
	seedWork(fs, 5, model.BookingStatusInProgress, model.InvoiceStatusPaid)
	fs.assigns[5].Status = model.AssignmentStatusAccepted
	at := mustTime()
	fs.assigns[5].StartedAt = &at
	svc := NewAssignmentService(fs, fakeTx{})

	a, err := svc.Complete(context.Background(), 9, 5, "Selesai, AC bersih")
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if a.Status != model.AssignmentStatusCompleted {
		t.Fatalf("status: %s", a.Status)
	}
	if a.TechnicianNote == nil || *a.TechnicianNote != "Selesai, AC bersih" {
		t.Fatalf("note wrong: %v", a.TechnicianNote)
	}
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if fs.bookings[1].Status != model.BookingStatusAwaitingVerification {
		t.Fatalf("booking should be awaiting_verification: %s", fs.bookings[1].Status)
	}
}

func TestWorkCompleteNotStarted(t *testing.T) {
	fs := newFakeAssign()
	seedWork(fs, 5, model.BookingStatusInProgress, model.InvoiceStatusPaid)
	fs.assigns[5].Status = model.AssignmentStatusAccepted
	svc := NewAssignmentService(fs, fakeTx{})
	_, err := svc.Complete(context.Background(), 9, 5, "note")
	he := httperr.As(err)
	if he == nil || he.Errors["assignment"][0] != "Job must be started before completion." {
		t.Fatalf("expected not-started 422, got %v", err)
	}
}

func TestWorkIdempotentAccept(t *testing.T) {
	fs := newFakeAssign()
	seedWork(fs, 5, model.BookingStatusTechnicianAssigned, model.InvoiceStatusPaid)
	svc := NewAssignmentService(fs, fakeTx{})
	if _, err := svc.Accept(context.Background(), 9, 5); err != nil {
		t.Fatalf("first accept: %v", err)
	}
	// second accept must fail (state no longer pending) â€” Laravel parity
	_, err := svc.Accept(context.Background(), 9, 5)
	he := httperr.As(err)
	if he == nil || len(he.Errors["assignment"]) == 0 {
		t.Fatalf("expected 422 on repeat accept, got %v", err)
	}
}

func TestWorkConcurrentAcceptSingleWinner(t *testing.T) {
	inner := newFakeAssign()
	seedWork(inner, 5, model.BookingStatusTechnicianAssigned, model.InvoiceStatusPaid)
	// serializer: claim assignment on first locked read (FOR UPDATE emulation)
	ser := &serAcceptStore{inner: inner, claimed: map[uint64]bool{}}
	svc := NewAssignmentService(ser, fakeTx{})

	var wg sync.WaitGroup
	wins := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.Accept(context.Background(), 9, 5)
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
		t.Fatalf("expected exactly 1 accept success, got %d", succ)
	}
}

// serAcceptStore claims an assignment on the first locked read so concurrent
// accepts serialize (the second reader sees accepted â†’ required-state 422).
type serAcceptStore struct {
	mu      sync.Mutex
	inner   *fakeAssignStore
	claimed map[uint64]bool
}

func (s *serAcceptStore) FindWorkForUpdate(ctx context.Context, q repository.Queryer, id uint64) (*model.BookingAssignment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp, err := s.inner.FindWorkForUpdate(ctx, q, id)
	if err != nil {
		return nil, err
	}
	if s.claimed[id] {
		cp.Status = model.AssignmentStatusAccepted
		return cp, nil
	}
	s.claimed[id] = true
	return cp, nil
}
func (s *serAcceptStore) FindBookingForAssign(ctx context.Context, q repository.Queryer, bookingID uint64) (*model.Booking, error) {
	return s.inner.FindBookingForAssign(ctx, q, bookingID)
}
func (s *serAcceptStore) FindTechnicianForAssign(ctx context.Context, q repository.Queryer, technicianID uint64) (*repository.TechnicianUser, error) {
	return s.inner.FindTechnicianForAssign(ctx, q, technicianID)
}
func (s *serAcceptStore) FindActiveAssignment(ctx context.Context, q repository.Queryer, bookingID uint64) (*model.BookingAssignment, error) {
	return s.inner.FindActiveAssignment(ctx, q, bookingID)
}
func (s *serAcceptStore) ReplaceAssignment(ctx context.Context, q repository.Queryer, id uint64, rejectedAt time.Time, reason string) error {
	return s.inner.ReplaceAssignment(ctx, q, id, rejectedAt, reason)
}
func (s *serAcceptStore) Create(ctx context.Context, q repository.Queryer, a *model.BookingAssignment) error {
	return s.inner.Create(ctx, q, a)
}
func (s *serAcceptStore) UpdateBookingStatus(ctx context.Context, q repository.Queryer, bookingID uint64, status string) error {
	return s.inner.UpdateBookingStatus(ctx, q, bookingID, status)
}
func (s *serAcceptStore) LoadBookingForResponse(ctx context.Context, q repository.Queryer, bookingID uint64) (*model.Booking, error) {
	return s.inner.LoadBookingForResponse(ctx, q, bookingID)
}
func (s *serAcceptStore) FetchByID(ctx context.Context, q repository.Queryer, id uint64) (*model.BookingAssignment, error) {
	return s.inner.FetchByID(ctx, q, id)
}
func (s *serAcceptStore) CountByTechnician(ctx context.Context, q repository.Queryer, techID uint64) (int, error) {
	return s.inner.CountByTechnician(ctx, q, techID)
}
func (s *serAcceptStore) ListByTechnician(ctx context.Context, q repository.Queryer, techID uint64, l, o int) ([]*model.BookingAssignment, error) {
	return s.inner.ListByTechnician(ctx, q, techID, l, o)
}
func (s *serAcceptStore) AttachJobRelations(ctx context.Context, q repository.Queryer, asg []*model.BookingAssignment) error {
	return s.inner.AttachJobRelations(ctx, q, asg)
}
func (s *serAcceptStore) MarkAccepted(ctx context.Context, q repository.Queryer, id uint64, at time.Time) error {
	return s.inner.MarkAccepted(ctx, q, id, at)
}
func (s *serAcceptStore) MarkRejected(ctx context.Context, q repository.Queryer, id uint64, at time.Time, reason string) error {
	return s.inner.MarkRejected(ctx, q, id, at, reason)
}
func (s *serAcceptStore) MarkStarted(ctx context.Context, q repository.Queryer, id uint64, at time.Time) error {
	return s.inner.MarkStarted(ctx, q, id, at)
}
func (s *serAcceptStore) MarkCompleted(ctx context.Context, q repository.Queryer, id uint64, at time.Time, note string) error {
	return s.inner.MarkCompleted(ctx, q, id, at, note)
}

func mustTime() time.Time { return time.Now().UTC() }
