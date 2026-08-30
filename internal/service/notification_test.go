package service

import (
	"context"
	"testing"
	"time"

	"github.com/Dwisajaa/golang-backend/internal/httperr"
	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/repository"
)

type fakeNotifStore struct {
	items  []*model.Notification
	total  int
	readID string
	all    bool
	err    error
}

func (f *fakeNotifStore) CountByUser(ctx context.Context, q repository.Queryer, userID uint64) (int, error) {
	return f.total, f.err
}
func (f *fakeNotifStore) ListByUser(ctx context.Context, q repository.Queryer, userID uint64, limit, offset int) ([]*model.Notification, error) {
	return f.items, f.err
}
func (f *fakeNotifStore) FindByUserAndID(ctx context.Context, q repository.Queryer, userID uint64, id string) (*model.Notification, error) {
	if f.err != nil {
		return nil, f.err
	}
	for _, n := range f.items {
		if n.ID == id {
			return n, nil
		}
	}
	return nil, repository.ErrNotFound
}
func (f *fakeNotifStore) MarkRead(ctx context.Context, q repository.Queryer, id string, now time.Time) error {
	f.readID = id
	return f.err
}
func (f *fakeNotifStore) MarkAllRead(ctx context.Context, q repository.Queryer, userID uint64) error {
	f.all = true
	return f.err
}

func TestNotificationListReadReadAll(t *testing.T) {
	fs := &fakeNotifStore{total: 1, items: []*model.Notification{{ID: "abc", Data: model.SystemNotification{Event: "e", Title: "t", Message: "m"}}}}
	svc := NewNotificationService(fs, fakeTx{})

	list, err := svc.List(context.Background(), 7, 1, 15)
	if err != nil || list.Total != 1 || len(list.Items) != 1 {
		t.Fatalf("list: %+v err=%v", list, err)
	}
	n, err := svc.Read(context.Background(), 7, "abc")
	if err != nil || n.ID != "abc" || fs.readID != "abc" {
		t.Fatalf("read: %+v err=%v readID=%q", n, err, fs.readID)
	}
	if err := svc.ReadAll(context.Background(), 7); err != nil || !fs.all {
		t.Fatalf("read all: err=%v all=%v", err, fs.all)
	}
}

func TestNotificationReadNotFound(t *testing.T) {
	svc := NewNotificationService(&fakeNotifStore{}, fakeTx{})
	_, err := svc.Read(context.Background(), 7, "missing")
	if he := httperr.As(err); he == nil || he.Kind != httperr.KindNotFound {
		t.Fatalf("expected 404, got %v", err)
	}
}
