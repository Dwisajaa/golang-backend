package httphandler

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Dwisajaa/golang-backend/internal/auth"
	"github.com/Dwisajaa/golang-backend/internal/httperr"
	"github.com/Dwisajaa/golang-backend/internal/middleware"
	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/service"
)

func timeNowUTC() time.Time { return time.Now().UTC().Add(time.Hour) }

type stubReviewSvc struct {
	rv   *model.Review
	list *service.ReviewList
	err  error
}

func (s *stubReviewSvc) Show(ctx context.Context, uid, bid uint64) (*model.Review, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.rv, nil
}
func (s *stubReviewSvc) Create(ctx context.Context, uid, bid uint64, rating int, comment string) (*model.Review, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.rv, nil
}
func (s *stubReviewSvc) AdminList(ctx context.Context, status string, p, pp int) (*service.ReviewList, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.list != nil {
		return s.list, nil
	}
	return &service.ReviewList{Total: 0, Page: 1, PerPage: 15}, nil
}
func (s *stubReviewSvc) Moderate(ctx context.Context, reviewID uint64, status string) (*model.Review, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.rv, nil
}

func reviewRouter(svc ReviewService, user *model.User, admin bool) (*gin.Engine, string) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	raw := "rv-token"
	gen := auth.NewRandomTokenGenerator()
	exp := timeNowUTC()
	u := user
	if u == nil {
		u = &model.User{ID: 7, Role: model.RoleCustomer}
	}
	h := NewReviewHandler(svc)
	api := r.Group("/api")
	auth := api.Group("")
	auth.Use(middleware.Auth(&fixedTokenStore{hash: gen.Hash(raw), userID: u.ID, exp: &exp}, &authUserStub{user: u}, gen))
	auth.Use(middleware.RequireRole(model.RoleCustomer))
	auth.GET("/bookings/:id/review", h.Show)
	auth.POST("/bookings/:id/review", h.Store)
	if admin {
		adm := api.Group("")
		adm.Use(middleware.Auth(&fixedTokenStore{hash: gen.Hash(raw), userID: u.ID, exp: &exp}, &authUserStub{user: u}, gen))
		adm.Use(middleware.RequireRole(model.RoleAdmin, model.RoleSuperAdmin))
		adm.GET("/admin/reviews", h.AdminList)
		adm.POST("/admin/reviews/:id/moderate", h.Moderate)
	}
	return r, raw
}

func sampleReview() *model.Review {
	return &model.Review{
		ID: 3, BookingID: 1, Rating: 5, Status: model.ReviewStatusPublished,
		Customer: &model.User{ID: 7, Name: "C"}, Technician: &model.User{ID: 9, Name: "T"},
	}
}

func TestReviewCreate201(t *testing.T) {
	svc := &stubReviewSvc{rv: sampleReview()}
	r, raw := reviewRouter(svc, nil, false)
	rec := doAuth(t, r, http.MethodPost, "/api/bookings/1/review", `{"rating":5,"comment":"bagus"}`, raw)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), "Review submitted successfully.") {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
}

func TestReviewInvalidRating(t *testing.T) {
	svc := &stubReviewSvc{rv: sampleReview()}
	r, raw := reviewRouter(svc, nil, false)
	rec := doAuth(t, r, http.MethodPost, "/api/bookings/1/review", `{"rating":6}`, raw)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
}

func TestReviewShowNotFound(t *testing.T) {
	svc := &stubReviewSvc{err: httperr.NotFound("Review not found.")}
	r, raw := reviewRouter(svc, nil, false)
	rec := doAuth(t, r, http.MethodGet, "/api/bookings/1/review", "", raw)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestReviewConflict(t *testing.T) {
	svc := &stubReviewSvc{err: httperr.Conflict("This booking already has a review.")}
	r, raw := reviewRouter(svc, nil, false)
	rec := doAuth(t, r, http.MethodPost, "/api/bookings/1/review", `{"rating":4}`, raw)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
}

func TestReviewAdminModerate(t *testing.T) {
	svc := &stubReviewSvc{rv: sampleReview()}
	r, raw := reviewRouter(svc, &model.User{ID: 2, Role: model.RoleAdmin}, true)
	rec := doAuth(t, r, http.MethodPost, "/api/admin/reviews/3/moderate", `{"status":"hidden"}`, raw)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Review moderated successfully.") {
		t.Fatalf("moderate: %d %s", rec.Code, rec.Body.String())
	}
}

func TestReviewAdminModerateInvalidStatus(t *testing.T) {
	svc := &stubReviewSvc{}
	r, raw := reviewRouter(svc, &model.User{ID: 2, Role: model.RoleAdmin}, true)
	rec := doAuth(t, r, http.MethodPost, "/api/admin/reviews/3/moderate", `{"status":"weird"}`, raw)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
}

func TestReviewWrongRole(t *testing.T) {
	svc := &stubReviewSvc{}
	r, raw := reviewRouter(svc, &model.User{ID: 8, Role: model.RoleTechnician}, false)
	rec := doAuth(t, r, http.MethodPost, "/api/bookings/1/review", `{"rating":5}`, raw)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestReviewUnauthorized(t *testing.T) {
	svc := &stubReviewSvc{}
	r, _ := reviewRouter(svc, nil, false)
	rec := doAuth(t, r, http.MethodGet, "/api/bookings/1/review", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestReviewInternalError(t *testing.T) {
	svc := &stubReviewSvc{err: httperr.Internal(ErrBooms{})}
	r, raw := reviewRouter(svc, &model.User{ID: 2, Role: model.RoleAdmin}, true)
	rec := doAuth(t, r, http.MethodGet, "/api/admin/reviews", "", raw)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}
