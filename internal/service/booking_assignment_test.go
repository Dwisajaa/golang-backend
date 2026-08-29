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

type fakeAssignStore struct {
	mu       sync.Mutex
	bookings map[uint64]*model.Booking
	techs    map[uint64]*repository.TechnicianUser
	assigns  map[uint64]*model.BookingAssignment
	created  []*model.BookingAssignment
	err      error
}

func newFakeAssign() *fakeAssignStore {
	paidInvoice := &model.Invoice{ID: 1, BookingID: 1, InvoiceNumber: "INV-1", Status: model.InvoiceStatusPaid}
	confirmed := &model.Booking{ID: 1, BookingCode: "BJA-1", CustomerID: 7, Status: model.BookingStatusConfirmed, Invoice: paidInvoice}
	active := true
	return &fakeAssignStore{
		bookings: map[uint64]*model.Booking{1: confirmed},
		techs: map[uint64]*repository.TechnicianUser{
			9: {User: &model.User{ID: 9, Name: "T", Role: model.RoleTechnician}, TechnicianProfile: &model.TechnicianProfile{UserID: 9, IsActive: active}},
		},
		assigns: map[uint64]*model.BookingAssignment{},
	}
}

func (f *fakeAssignStore) FindBookingForAssign(ctx context.Context, q repository.Queryer, bookingID uint64) (*model.Booking, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	b, ok := f.bookings[bookingID]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return b, nil
}
func (f *fakeAssignStore) FindTechnicianForAssign(ctx context.Context, q repository.Queryer, technicianID uint64) (*repository.TechnicianUser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.techs[technicianID]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return t, nil
}
func (f *fakeAssignStore) FindActiveAssignment(ctx context.Context, q repository.Queryer, bookingID uint64) (*model.BookingAssignment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, a := range f.assigns {
		if a.BookingID == bookingID && model.IsActiveAssignment(a.Status) {
			return a, nil
		}
	}
	return nil, repository.ErrNotFound
}
func (f *fakeAssignStore) ReplaceAssignment(ctx context.Context, q repository.Queryer, id uint64, rejectedAt time.Time, reason string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.assigns[id].Status = model.AssignmentStatusRejected
	f.assigns[id].RejectionReason = &reason
	return nil
}
func (f *fakeAssignStore) Create(ctx context.Context, q repository.Queryer, a *model.BookingAssignment) error {
	if f.techs[a.TechnicianID] == nil {
		return repository.ErrNotFound
	}
	f.created = append(f.created, a)
	a.ID = uint64(len(f.created))
	f.assigns[a.ID] = a
	return nil
}
func (f *fakeAssignStore) UpdateBookingStatus(ctx context.Context, q repository.Queryer, bookingID uint64, status string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.bookings[bookingID].Status = status
	return nil
}
func (f *fakeAssignStore) LoadBookingForResponse(ctx context.Context, q repository.Queryer, bookingID uint64) (*model.Booking, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.bookings[bookingID], nil
}

func TestAssignSuccess(t *testing.T) {
	fs := newFakeAssign()
	svc := NewAssignmentService(fs, fakeTx{})

	a, err := svc.Assign(context.Background(), 2, 1, 9)
	if err != nil {
		t.Fatalf("assign: %v", err)
	}
	if a.Status != model.AssignmentStatusPending || a.BookingID != 1 || a.TechnicianID != 9 {
		t.Fatalf("assignment wrong: %+v", a)
	}
	if a.AssignedBy == nil || *a.AssignedBy != 2 {
		t.Fatalf("assigned_by must be admin id, got %v", a.AssignedBy)
	}
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if fs.bookings[1].Status != model.BookingStatusTechnicianAssigned {
		t.Fatalf("booking status: %s", fs.bookings[1].Status)
	}
}

func TestAssignBookingNotConfirmed(t *testing.T) {
	fs := newFakeAssign()
	fs.bookings[1].Status = model.BookingStatusPendingPayment
	svc := NewAssignmentService(fs, fakeTx{})

	_, err := svc.Assign(context.Background(), 2, 1, 9)
	he := httperr.As(err)
	if he == nil || he.Kind != httperr.KindValidation || len(he.Errors["booking"]) == 0 {
		t.Fatalf("expected 422 booking, got %v", err)
	}
}

func TestAssignCancelledBooking(t *testing.T) {
	fs := newFakeAssign()
	fs.bookings[1].Status = model.BookingStatusCancelled
	svc := NewAssignmentService(fs, fakeTx{})

	_, err := svc.Assign(context.Background(), 2, 1, 9)
	he := httperr.As(err)
	if he == nil || len(he.Errors["booking"]) == 0 || he.Errors["booking"][0] != "Cancelled bookings cannot be assigned." {
		t.Fatalf("expected cancelled msg, got %v", err)
	}
}

func TestAssignUnpaidInvoice(t *testing.T) {
	fs := newFakeAssign()
	fs.bookings[1].Invoice.Status = model.InvoiceStatusUnpaid
	svc := NewAssignmentService(fs, fakeTx{})

	_, err := svc.Assign(context.Background(), 2, 1, 9)
	he := httperr.As(err)
	if he == nil || len(he.Errors["booking"]) == 0 {
		t.Fatalf("expected 422 (must be paid), got %v", err)
	}
}

func TestAssignInvalidTechnician(t *testing.T) {
	fs := newFakeAssign()
	fs.techs[9].User.Role = model.RoleCustomer
	svc := NewAssignmentService(fs, fakeTx{})

	_, err := svc.Assign(context.Background(), 2, 1, 9)
	he := httperr.As(err)
	if he == nil || len(he.Errors["technician_id"]) == 0 {
		t.Fatalf("expected 422 technician, got %v", err)
	}
}

func TestAssignInactiveTechnician(t *testing.T) {
	fs := newFakeAssign()
	fs.techs[9].TechnicianProfile.IsActive = false
	svc := NewAssignmentService(fs, fakeTx{})

	_, err := svc.Assign(context.Background(), 2, 1, 9)
	he := httperr.As(err)
	if he == nil || len(he.Errors["technician_id"]) == 0 {
		t.Fatalf("expected 422 technician inactive, got %v", err)
	}
}

func TestAssignReassignmentReplacesActive(t *testing.T) {
	fs := newFakeAssign()
	// booking still confirmed but holds a stale active assignment (e.g. a
	// previous assign that never left confirmed) -> admin reassign replaces it.
	old := &model.BookingAssignment{ID: 100, BookingID: 1, TechnicianID: 9, Status: model.AssignmentStatusPending}
	fs.assigns[100] = old
	fs.techs[10] = &repository.TechnicianUser{
		User:              &model.User{ID: 10, Name: "T2", Role: model.RoleTechnician},
		TechnicianProfile: &model.TechnicianProfile{UserID: 10, IsActive: true},
	}
	svc := NewAssignmentService(fs, fakeTx{})

	a2, err := svc.Assign(context.Background(), 2, 1, 10)
	if err != nil {
		t.Fatalf("reassign: %v", err)
	}
	if a2.TechnicianID != 10 {
		t.Fatalf("new assignment should target tech 10: %+v", a2)
	}
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if fs.assigns[100].Status != model.AssignmentStatusRejected {
		t.Fatalf("old active assignment must be rejected: %+v", fs.assigns[100])
	}
	if fs.assigns[100].RejectionReason == nil || *fs.assigns[100].RejectionReason != model.ReplacementReason {
		t.Fatalf("replacement reason mismatch: %v", fs.assigns[100].RejectionReason)
	}
	if fs.bookings[1].Status != model.BookingStatusTechnicianAssigned {
		t.Fatalf("booking status after reassign: %s", fs.bookings[1].Status)
	}
}

func TestAssignBookingNotFound(t *testing.T) {
	fs := newFakeAssign()
	svc := NewAssignmentService(fs, fakeTx{})
	_, err := svc.Assign(context.Background(), 2, 999, 9)
	if he := httperr.As(err); he == nil || he.Kind != httperr.KindNotFound {
		t.Fatalf("expected 404, got %v", err)
	}
}

func TestAssignConcurrentSingleWinner(t *testing.T) {
	// The real FOR UPDATE serialization is proven by the integration test;
	// this emulates the lock via a claim so only the first assign proceeds
	// while the booking is still confirmed.
	inner := newFakeAssign()
	ser := &serAssignStore{inner: inner, claimed: map[uint64]bool{}}
	svc := NewAssignmentService(ser, fakeTx{})

	var wg sync.WaitGroup
	wins := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.Assign(context.Background(), 2, 1, 9)
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
		t.Fatalf("expected exactly 1 successful assign, got %d", succ)
	}
}

// serAssignStore claims a booking on first read (FOR UPDATE emulation): later
// reads see the booking already technician_assigned.
type serAssignStore struct {
	mu      sync.Mutex
	inner   *fakeAssignStore
	claimed map[uint64]bool
}

func (s *serAssignStore) FindBookingForAssign(ctx context.Context, q repository.Queryer, bookingID uint64) (*model.Booking, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := s.inner.FindBookingForAssign(ctx, q, bookingID)
	if err != nil {
		return nil, err
	}
	if s.claimed[bookingID] {
		cp := *b
		cp.Status = model.BookingStatusTechnicianAssigned
		return &cp, nil
	}
	s.claimed[bookingID] = true
	return b, nil
}
func (s *serAssignStore) FindTechnicianForAssign(ctx context.Context, q repository.Queryer, technicianID uint64) (*repository.TechnicianUser, error) {
	return s.inner.FindTechnicianForAssign(ctx, q, technicianID)
}
func (s *serAssignStore) FindActiveAssignment(ctx context.Context, q repository.Queryer, bookingID uint64) (*model.BookingAssignment, error) {
	return s.inner.FindActiveAssignment(ctx, q, bookingID)
}
func (s *serAssignStore) ReplaceAssignment(ctx context.Context, q repository.Queryer, id uint64, rejectedAt time.Time, reason string) error {
	return s.inner.ReplaceAssignment(ctx, q, id, rejectedAt, reason)
}
func (s *serAssignStore) Create(ctx context.Context, q repository.Queryer, a *model.BookingAssignment) error {
	return s.inner.Create(ctx, q, a)
}
func (s *serAssignStore) UpdateBookingStatus(ctx context.Context, q repository.Queryer, bookingID uint64, status string) error {
	return s.inner.UpdateBookingStatus(ctx, q, bookingID, status)
}
func (s *serAssignStore) LoadBookingForResponse(ctx context.Context, q repository.Queryer, bookingID uint64) (*model.Booking, error) {
	return s.inner.LoadBookingForResponse(ctx, q, bookingID)
}

// -- workflow interface methods --
func (f *fakeAssignStore) FindWorkForUpdate(ctx context.Context, q repository.Queryer, id uint64) (*model.BookingAssignment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	a, ok := f.assigns[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	cp := *a
	if b, ok := f.bookings[cp.BookingID]; ok {
		bc := *b
		cp.Booking = &bc
	}
	return &cp, nil
}
func (f *fakeAssignStore) FetchByID(ctx context.Context, q repository.Queryer, id uint64) (*model.BookingAssignment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	a, ok := f.assigns[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return a, nil
}
func (f *fakeAssignStore) CountByTechnician(ctx context.Context, q repository.Queryer, techID uint64) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, a := range f.assigns {
		if a.TechnicianID == techID {
			n++
		}
	}
	return n, nil
}
func (f *fakeAssignStore) ListByTechnician(ctx context.Context, q repository.Queryer, techID uint64, limit, offset int) ([]*model.BookingAssignment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*model.BookingAssignment
	for _, a := range f.assigns {
		if a.TechnicianID == techID {
			out = append(out, a)
		}
	}
	return out, nil
}
func (f *fakeAssignStore) AttachJobRelations(ctx context.Context, q repository.Queryer, asg []*model.BookingAssignment) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, a := range asg {
		if b, ok := f.bookings[a.BookingID]; ok {
			a.Booking = b
		}
	}
	return nil
}
func (f *fakeAssignStore) MarkAccepted(ctx context.Context, q repository.Queryer, id uint64, at time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.assigns[id].Status = model.AssignmentStatusAccepted
	f.assigns[id].AcceptedAt = &at
	return nil
}
func (f *fakeAssignStore) MarkRejected(ctx context.Context, q repository.Queryer, id uint64, at time.Time, reason string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.assigns[id].Status = model.AssignmentStatusRejected
	f.assigns[id].RejectedAt = &at
	f.assigns[id].RejectionReason = &reason
	return nil
}
func (f *fakeAssignStore) MarkStarted(ctx context.Context, q repository.Queryer, id uint64, at time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.assigns[id].StartedAt = &at
	return nil
}
func (f *fakeAssignStore) MarkCompleted(ctx context.Context, q repository.Queryer, id uint64, at time.Time, note string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.assigns[id].Status = model.AssignmentStatusCompleted
	f.assigns[id].CompletedAt = &at
	f.assigns[id].TechnicianNote = &note
	return nil
}

// serAssignStore workflow delegates
func (s *serAssignStore) FindWorkForUpdate(ctx context.Context, q repository.Queryer, id uint64) (*model.BookingAssignment, error) {
	return s.inner.FindWorkForUpdate(ctx, q, id)
}
func (s *serAssignStore) FetchByID(ctx context.Context, q repository.Queryer, id uint64) (*model.BookingAssignment, error) {
	return s.inner.FetchByID(ctx, q, id)
}
func (s *serAssignStore) CountByTechnician(ctx context.Context, q repository.Queryer, techID uint64) (int, error) {
	return s.inner.CountByTechnician(ctx, q, techID)
}
func (s *serAssignStore) ListByTechnician(ctx context.Context, q repository.Queryer, techID uint64, l, o int) ([]*model.BookingAssignment, error) {
	return s.inner.ListByTechnician(ctx, q, techID, l, o)
}
func (s *serAssignStore) AttachJobRelations(ctx context.Context, q repository.Queryer, asg []*model.BookingAssignment) error {
	return s.inner.AttachJobRelations(ctx, q, asg)
}
func (s *serAssignStore) MarkAccepted(ctx context.Context, q repository.Queryer, id uint64, at time.Time) error {
	return s.inner.MarkAccepted(ctx, q, id, at)
}
func (s *serAssignStore) MarkRejected(ctx context.Context, q repository.Queryer, id uint64, at time.Time, reason string) error {
	return s.inner.MarkRejected(ctx, q, id, at, reason)
}
func (s *serAssignStore) MarkStarted(ctx context.Context, q repository.Queryer, id uint64, at time.Time) error {
	return s.inner.MarkStarted(ctx, q, id, at)
}
func (s *serAssignStore) MarkCompleted(ctx context.Context, q repository.Queryer, id uint64, at time.Time, note string) error {
	return s.inner.MarkCompleted(ctx, q, id, at, note)
}
