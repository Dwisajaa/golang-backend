package service

import (
	"context"
	"sync"
	"testing"

	"github.com/Dwisajaa/golang-backend/internal/httperr"
	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/repository"
)

type fakeReviewStore struct {
	mu         sync.Mutex
	reviews    map[uint64]*model.Review
	exists     map[uint64]bool
	created    []*model.Review
	latestTech map[uint64]uint64
	statuses   []string
	err        error
}

func newFakeReview() *fakeReviewStore {
	return &fakeReviewStore{reviews: map[uint64]*model.Review{}, exists: map[uint64]bool{}, latestTech: map[uint64]uint64{}}
}

func (f *fakeReviewStore) Create(ctx context.Context, q repository.Queryer, r *model.Review) error {
	if f.err != nil {
		return f.err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.exists[r.BookingID] {
		return repository.ErrDuplicate
	}
	r.ID = uint64(len(f.reviews) + 1)
	f.reviews[r.ID] = r
	f.exists[r.BookingID] = true
	f.created = append(f.created, r)
	return nil
}
func (f *fakeReviewStore) FindByBooking(ctx context.Context, q repository.Queryer, bookingID uint64) (*model.Review, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.reviews {
		if r.BookingID == bookingID {
			return r, nil
		}
	}
	return nil, repository.ErrNotFound
}
func (f *fakeReviewStore) FindByID(ctx context.Context, q repository.Queryer, id uint64) (*model.Review, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r, ok := f.reviews[id]; ok {
		return r, nil
	}
	return nil, repository.ErrNotFound
}
func (f *fakeReviewStore) ReviewExists(ctx context.Context, q repository.Queryer, bookingID uint64) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.exists[bookingID], nil
}
func (f *fakeReviewStore) LatestAssignmentTechnicianID(ctx context.Context, q repository.Queryer, bookingID uint64) (uint64, error) {
	id, ok := f.latestTech[bookingID]
	if !ok {
		return 0, repository.ErrNotFound
	}
	return id, nil
}
func (f *fakeReviewStore) AdminCount(ctx context.Context, q repository.Queryer, status string) (int, error) {
	return len(f.reviews), f.err
}
func (f *fakeReviewStore) AdminList(ctx context.Context, q repository.Queryer, status string, l, o int) ([]*model.Review, error) {
	var out []*model.Review
	for _, r := range f.reviews {
		out = append(out, r)
	}
	return out, f.err
}
func (f *fakeReviewStore) UpdateStatus(ctx context.Context, q repository.Queryer, id uint64, status string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reviews[id].Status = status
	return nil
}

func reviewSvc(t *testing.T) (*ReviewService, *fakeReviewStore, *fakeBookingStore) {
	t.Helper()
	fs := newFakeReview()
	bs := newFakeBooking()
	bs.byID[1] = &model.Booking{ID: 1, BookingCode: "BJA-1", CustomerID: 7, Status: model.BookingStatusCompleted}
	fs.latestTech[1] = 9
	return NewReviewService(fs, bs, fakeTx{}), fs, bs
}

func TestReviewCreateSuccess(t *testing.T) {
	svc, _, _ := reviewSvc(t)
	rv, err := svc.Create(context.Background(), 7, 1, 5, "Bagus")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if rv.Rating != 5 || rv.TechnicianID != 9 || rv.Status != model.ReviewStatusPublished {
		t.Fatalf("review wrong: %+v", rv)
	}
}

func TestReviewCreateNotEligible(t *testing.T) {
	svc, _, bs := reviewSvc(t)
	bs.byID[1].Status = model.BookingStatusAwaitingVerification
	_, err := svc.Create(context.Background(), 7, 1, 5, "")
	he := httperr.As(err)
	if he == nil || he.Kind != httperr.KindConflict || he.Message != "Booking is not eligible for review." {
		t.Fatalf("expected 409 not eligible, got %v", err)
	}
}

func TestReviewCreateNoTechnician(t *testing.T) {
	svc, fs, _ := reviewSvc(t)
	delete(fs.latestTech, 1)
	_, err := svc.Create(context.Background(), 7, 1, 5, "")
	he := httperr.As(err)
	if he == nil || he.Kind != httperr.KindConflict {
		t.Fatalf("expected 409 (no technician), got %v", err)
	}
}

func TestReviewCreateDuplicate(t *testing.T) {
	svc, _, _ := reviewSvc(t)
	if _, err := svc.Create(context.Background(), 7, 1, 5, "a"); err != nil {
		t.Fatalf("first: %v", err)
	}
	_, err := svc.Create(context.Background(), 7, 1, 4, "b")
	he := httperr.As(err)
	if he == nil || he.Kind != httperr.KindConflict || he.Message != "This booking already has a review." {
		t.Fatalf("expected 409 already reviewed, got %v", err)
	}
}

func TestReviewCreateNotOwner(t *testing.T) {
	svc, _, _ := reviewSvc(t)
	_, err := svc.Create(context.Background(), 99, 1, 5, "")
	if he := httperr.As(err); he == nil || he.Kind != httperr.KindForbidden {
		t.Fatalf("expected 403, got %v", err)
	}
}

func TestReviewShowNotFound(t *testing.T) {
	svc, _, _ := reviewSvc(t)
	_, err := svc.Show(context.Background(), 7, 1)
	he := httperr.As(err)
	if he == nil || he.Kind != httperr.KindNotFound {
		t.Fatalf("expected 404, got %v", err)
	}
}

func TestReviewModerate(t *testing.T) {
	svc, fs, _ := reviewSvc(t)
	if _, err := svc.Create(context.Background(), 7, 1, 5, "a"); err != nil {
		t.Fatalf("create: %v", err)
	}
	rv, err := svc.Moderate(context.Background(), fs.reviews[1].ID, model.ReviewStatusHidden)
	if err != nil || rv.Status != model.ReviewStatusHidden {
		t.Fatalf("moderate: %+v err=%v", rv, err)
	}
}

func TestReviewRepoError(t *testing.T) {
	fs := newFakeReview()
	bs := newFakeBooking()
	bs.byID[1] = &model.Booking{ID: 1, CustomerID: 7, Status: model.BookingStatusCompleted}
	fs.err = errReviewDown{}
	svc := NewReviewService(fs, bs, fakeTx{})
	if _, err := svc.AdminList(context.Background(), "", 1, 15); httperr.As(err) == nil {
		t.Fatalf("expected error, got nil")
	}
}

type errReviewDown struct{}

func (errReviewDown) Error() string { return "db down" }

func TestReviewConcurrentDuplicateSingleSuccess(t *testing.T) {
	fs := newFakeReview()
	bs := newFakeBooking()
	bs.byID[1] = &model.Booking{ID: 1, CustomerID: 7, Status: model.BookingStatusCompleted}
	fs.latestTech[1] = 9
	svc := NewReviewService(fs, bs, fakeTx{})

	var wg sync.WaitGroup
	wins := make(chan error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.Create(context.Background(), 7, 1, 5, "")
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
		t.Fatalf("expected exactly 1 review created, got %d", succ)
	}
}
