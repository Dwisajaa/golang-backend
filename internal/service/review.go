package service

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Dwisajaa/golang-backend/internal/httperr"
	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/notify"
	"github.com/Dwisajaa/golang-backend/internal/repository"
)

type reviewStore interface {
	Create(ctx context.Context, q repository.Queryer, r *model.Review) error
	FindByBooking(ctx context.Context, q repository.Queryer, bookingID uint64) (*model.Review, error)
	FindByID(ctx context.Context, q repository.Queryer, id uint64) (*model.Review, error)
	ReviewExists(ctx context.Context, q repository.Queryer, bookingID uint64) (bool, error)
	LatestAssignmentTechnicianID(ctx context.Context, q repository.Queryer, bookingID uint64) (uint64, error)
	AdminCount(ctx context.Context, q repository.Queryer, status string) (int, error)
	AdminList(ctx context.Context, q repository.Queryer, status string, limit, offset int) ([]*model.Review, error)
	UpdateStatus(ctx context.Context, q repository.Queryer, id uint64, status string) error
}

// bookingLock is the minimal booking surface the review service needs (row
// lock on create review).
type bookingLock interface {
	FindByIDForUpdate(ctx context.Context, q repository.Queryer, id uint64) (*model.Booking, error)
}

// ReviewService owns the review read/create + admin list/moderate flows.
type ReviewService struct {
	reviews  reviewStore
	bookings bookingLock
	tx       txRunner
	notify   notify.Notifier
}

func NewReviewService(reviews reviewStore, bookings bookingLock, tx txRunner, notify ...notify.Notifier) *ReviewService {
	s := &ReviewService{reviews: reviews, bookings: bookings, tx: tx}
	if len(notify) > 0 {
		s.notify = notify[0]
	}
	return s
}

// Show mirrors ReviewController@show (owner customer).
func (s *ReviewService) Show(ctx context.Context, userID, bookingID uint64) (*model.Review, error) {
	var out *model.Review
	err := s.tx.Within(ctx, func(tx *sql.Tx) error {
		b, err := s.bookings.FindByIDForUpdate(ctx, tx, bookingID)
		if err != nil {
			return err
		}
		if b.CustomerID != userID {
			return htForbidden
		}
		rv, err := s.reviews.FindByBooking(ctx, tx, bookingID)
		if err != nil {
			return err
		}
		out = rv
		return nil
	})
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, httperr.NotFound("Review not found.")
		}
		return nil, mapReviewErr(err)
	}
	return out, nil
}

// Create mirrors ReviewController@store.
func (s *ReviewService) Create(ctx context.Context, userID, bookingID uint64, rating int, comment string) (*model.Review, error) {
	var out *model.Review
	err := s.tx.Within(ctx, func(tx *sql.Tx) error {
		locked, err := s.bookings.FindByIDForUpdate(ctx, tx, bookingID)
		if err != nil {
			return err
		}
		if locked.CustomerID != userID {
			return htForbidden
		}
		if locked.Status != model.BookingStatusCompleted {
			return httperr.Conflict("Booking is not eligible for review.")
		}
		techID, err := s.reviews.LatestAssignmentTechnicianID(ctx, tx, bookingID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return httperr.Conflict("Booking is not eligible for review.")
			}
			return err
		}
		exists, err := s.reviews.ReviewExists(ctx, tx, bookingID)
		if err != nil {
			return err
		}
		if exists {
			return httperr.Conflict("This booking already has a review.")
		}
		rv := &model.Review{
			BookingID: bookingID, CustomerID: userID, TechnicianID: techID,
			Rating: rating, Comment: nullIfEmpty(comment), Status: model.ReviewStatusPublished,
		}
		if err := s.reviews.Create(ctx, tx, rv); err != nil {
			return err
		}
		out = rv
		return nil
	})
	if err != nil {
		return nil, mapReviewErr(err)
	}
	// Post-commit notification to the technician (review_submitted).
	if s.notify != nil && out != nil {
		bk := bookingID
		s.notify.NotifyUser(ctx, out.TechnicianID, model.SystemNotification{
			Event: "review_submitted", Title: "New review",
			Message:   "You received a new customer review.",
			BookingID: &bk,
		})
	}
	return out, nil
}

// ReviewList is the paginated admin review list.
type ReviewList struct {
	Items   []*model.Review
	Total   int
	Page    int
	PerPage int
}

func (s *ReviewService) AdminList(ctx context.Context, status string, page, perPage int) (*ReviewList, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 15
	}
	if perPage > 50 {
		perPage = 50
	}
	var out ReviewList
	out.Page, out.PerPage = page, perPage
	err := s.tx.Within(ctx, func(tx *sql.Tx) error {
		total, err := s.reviews.AdminCount(ctx, tx, status)
		if err != nil {
			return err
		}
		items, err := s.reviews.AdminList(ctx, tx, status, perPage, (page-1)*perPage)
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

func (s *ReviewService) Moderate(ctx context.Context, reviewID uint64, status string) (*model.Review, error) {
	var out *model.Review
	err := s.tx.Within(ctx, func(tx *sql.Tx) error {
		rv, err := s.reviews.FindByID(ctx, tx, reviewID)
		if err != nil {
			return err
		}
		if err := s.reviews.UpdateStatus(ctx, tx, reviewID, status); err != nil {
			return err
		}
		rv.Status = status
		out = rv
		return nil
	})
	if err != nil {
		return nil, mapReviewErr(err)
	}
	return out, nil
}

func mapReviewErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, repository.ErrNotFound) {
		return httperr.NotFound("Resource not found.")
	}
	if errors.Is(err, repository.ErrDuplicate) {
		return httperr.Conflict("This booking already has a review.")
	}
	if he := httperr.As(err); he != nil {
		return he
	}
	return httperr.Internal(err)
}
