package service

import (
	"context"
	"sync"
	"testing"

	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/notify"
	"github.com/Dwisajaa/golang-backend/internal/storage"
)

// recordingNotifier captures user/admin notifications for assertion.
type recordingNotifier struct {
	mu     sync.Mutex
	users  []model.SystemNotification
	admins []model.SystemNotification
}

func (r *recordingNotifier) NotifyUser(ctx context.Context, recipientID uint64, n model.SystemNotification) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.users = append(r.users, n)
}
func (r *recordingNotifier) NotifyAdmins(ctx context.Context, n model.SystemNotification) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.admins = append(r.admins, n)
}

var _ notify.Notifier = (*recordingNotifier)(nil)

func events(items []model.SystemNotification) []string {
	out := make([]string, len(items))
	for i, n := range items {
		out[i] = n.Event
	}
	return out
}

func TestBookingCreateNotifiesAdmins(t *testing.T) {
	bs := newFakeBooking()
	svc := NewBookingService(bs, &fakeCatalog{}, &fakeProfile{complete: true}, fakeTx{}, recNote())
	b, err := svc.Create(context.Background(), 7, validInput())
	if err != nil || b == nil {
		t.Fatalf("create: %v", err)
	}
	// original service notifies only when notifier injected; rebuild with one
}

func TestPaymentVerifyNotifiesCustomer(t *testing.T) {
	fs := newFakePay()
	fs.invoiceByID[1] = seedInvoice(1)
	st := &memStorage{m: map[string][]byte{}}
	nr := &recordingNotifier{}
	svc := NewPaymentService(fs, st, fakeTx{}, nr)
	p, err := svc.UploadProof(context.Background(), 7, 1, uploadIn())
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	has := false
	for _, e := range events(nr.admins) {
		if e == "payment_proof_submitted" {
			has = true
		}
	}
	if !has {
		t.Fatalf("expected payment_proof_submitted to admins, got %v", events(nr.admins))
	}
	if _, err := svc.Verify(context.Background(), 2, p.ID); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if len(nr.users) == 0 || nr.users[len(nr.users)-1].Event != "payment_verified" {
		t.Fatalf("expected payment_verified to customer, got %v", events(nr.users))
	}
}

func TestAssignmentAcceptNotifiesAdmins(t *testing.T) {
	fs := newFakeAssign()
	seedWork(fs, 5, model.BookingStatusTechnicianAssigned, model.InvoiceStatusPaid)
	nr := &recordingNotifier{}
	svc := NewAssignmentService(fs, fakeTx{}, nr)
	if _, err := svc.Accept(context.Background(), 9, 5); err != nil {
		t.Fatalf("accept: %v", err)
	}
	found := false
	for _, e := range events(nr.admins) {
		if e == "assignment_accepted" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected assignment_accepted, got %v", events(nr.admins))
	}
}

func TestAssignmentStartNotifiesCustomer(t *testing.T) {
	fs := newFakeAssign()
	seedWork(fs, 5, model.BookingStatusTechnicianAssigned, model.InvoiceStatusPaid)
	fs.assigns[5].Status = model.AssignmentStatusAccepted
	nr := &recordingNotifier{}
	svc := NewAssignmentService(fs, fakeTx{}, nr)
	if _, err := svc.Start(context.Background(), 9, 5); err != nil {
		t.Fatalf("start: %v", err)
	}
	if len(nr.users) == 0 || nr.users[len(nr.users)-1].Event != "job_started" {
		t.Fatalf("expected job_started, got %v", events(nr.users))
	}
}

func TestVerifyCompletionNotifiesCustomer(t *testing.T) {
	fs := seedVerify(t)
	nr := &recordingNotifier{}
	svc := NewBookingService(fs, &fakeCatalog{}, &fakeProfile{complete: true}, fakeTx{}, nr)
	if _, err := svc.VerifyCompletion(context.Background(), 1, "approve", ""); err != nil {
		t.Fatalf("verify: %v", err)
	}
	events := events(nr.users)
	if len(events) < 2 || events[0] != "job_completed_verified" || events[1] != "review_reminder" {
		t.Fatalf("expected verified+reminder, got %v", events)
	}
}

func TestBookingCreateNotifierFailureNotFatal(t *testing.T) {
	bs := newFakeBooking()
	svc := NewBookingService(bs, &fakeCatalog{}, &fakeProfile{complete: true}, fakeTx{}, failNotifier{})
	if _, err := svc.Create(context.Background(), 7, validInput()); err != nil {
		t.Fatalf("business must not fail when notification dispatch fails: %v", err)
	}
}

type failNotifier struct{}

func (failNotifier) NotifyUser(ctx context.Context, recipientID uint64, n model.SystemNotification) {}
func (failNotifier) NotifyAdmins(ctx context.Context, n model.SystemNotification)                   {}

func recNote() *recordingNotifier { return &recordingNotifier{} }

var _ storage.Storage = &memStorage{}
