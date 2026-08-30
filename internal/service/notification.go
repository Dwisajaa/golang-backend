package service

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Dwisajaa/golang-backend/internal/httperr"
	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/repository"
)

type notificationStore interface {
	CountByUser(ctx context.Context, q repository.Queryer, userID uint64) (int, error)
	ListByUser(ctx context.Context, q repository.Queryer, userID uint64, limit, offset int) ([]*model.Notification, error)
	FindByUserAndID(ctx context.Context, q repository.Queryer, userID uint64, id string) (*model.Notification, error)
	MarkRead(ctx context.Context, q repository.Queryer, id string, now time.Time) error
	MarkAllRead(ctx context.Context, q repository.Queryer, userID uint64) error
}

// NotificationService owns the in-app notification read API (list / read /
// read-all). Emitting notifications happens via notify.Notifier post-commit.
type NotificationService struct {
	store notificationStore
	tx    txRunner
}

func NewNotificationService(store notificationStore, tx txRunner) *NotificationService {
	return &NotificationService{store: store, tx: tx}
}

type NotificationList struct {
	Items   []*model.Notification
	Total   int
	Page    int
	PerPage int
}

func (s *NotificationService) List(ctx context.Context, userID uint64, page, perPage int) (*NotificationList, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 15
	}
	if perPage > 50 {
		perPage = 50
	}
	var out NotificationList
	out.Page, out.PerPage = page, perPage
	err := s.tx.Within(ctx, func(tx *sql.Tx) error {
		total, err := s.store.CountByUser(ctx, tx, userID)
		if err != nil {
			return err
		}
		items, err := s.store.ListByUser(ctx, tx, userID, perPage, (page-1)*perPage)
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

func (s *NotificationService) Read(ctx context.Context, userID uint64, id string) (*model.Notification, error) {
	var out *model.Notification
	err := s.tx.Within(ctx, func(tx *sql.Tx) error {
		n, err := s.store.FindByUserAndID(ctx, tx, userID, id)
		if err != nil {
			return err
		}
		if err := s.store.MarkRead(ctx, tx, id, time.Now().UTC()); err != nil {
			return err
		}
		now := time.Now().UTC()
		n.ReadAt = &now
		out = n
		return nil
	})
	if err != nil {
		return nil, mapNotifErr(err)
	}
	return out, nil
}

func (s *NotificationService) ReadAll(ctx context.Context, userID uint64) error {
	err := s.tx.Within(ctx, func(tx *sql.Tx) error {
		return s.store.MarkAllRead(ctx, tx, userID)
	})
	if err != nil {
		return httperr.Internal(err)
	}
	return nil
}

func mapNotifErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, repository.ErrNotFound) {
		return httperr.NotFound("Resource not found.")
	}
	return httperr.Internal(err)
}
