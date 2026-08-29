package httphandler

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Dwisajaa/golang-backend/internal/auth"
	"github.com/Dwisajaa/golang-backend/internal/httperr"
	"github.com/Dwisajaa/golang-backend/internal/middleware"
	"github.com/Dwisajaa/golang-backend/internal/model"
)

func adminRouterForBooking(h *BookingHandler, user *model.User, raw string) (*gin.Engine, string) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	gen := auth.NewRandomTokenGenerator()
	exp := time.Now().UTC().Add(time.Hour)
	api := r.Group("/api")
	adm := api.Group("")
	adm.Use(middleware.Auth(&fixedTokenStore{hash: gen.Hash(raw), userID: user.ID, exp: &exp}, &authUserStub{user: user}, gen))
	adm.Use(middleware.RequireRole(model.RoleAdmin, model.RoleSuperAdmin))
	adm.POST("/admin/bookings/:id/verify", h.VerifyCompletion)
	adm.GET("/admin/bookings", h.AdminList)
	return r, raw
}

func verifyRouter(svc BookingService, user *model.User) (*gin.Engine, string) {
	r, raw := adminRouterForBooking(NewBookingHandler(svc), user, "verify-token")
	return r, raw
}

func TestVerifyApprove(t *testing.T) {
	b := sampleBooking()
	b.Status = model.BookingStatusCompleted
	b.Assignments = []*model.BookingAssignment{{ID: 50, Status: model.AssignmentStatusCompleted, Technician: &model.User{ID: 9, Name: "T"}}}
	svc := &stubBookingSvc{b: b}
	r, raw := adminRouterForBooking(NewBookingHandler(svc), &model.User{ID: 2, Role: model.RoleAdmin}, "verify-token")
	rec := doAuth(t, r, http.MethodPost, "/api/admin/bookings/1/verify", `{"action":"approve","admin_verification_note":"ok"}`, raw)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Penyelesaian booking disetujui.") {
		t.Fatalf("message: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "latest_assignment") {
		t.Fatalf("latest_assignment missing: %s", rec.Body.String())
	}
}

func TestVerifyReject(t *testing.T) {
	b := sampleBooking()
	b.Status = model.BookingStatusInProgress
	svc := &stubBookingSvc{b: b}
	r, raw := adminRouterForBooking(NewBookingHandler(svc), &model.User{ID: 2, Role: model.RoleSuperAdmin}, "verify-token")
	rec := doAuth(t, r, http.MethodPost, "/api/admin/bookings/1/verify", `{"action":"reject","admin_verification_note":"perbaiki"}`, raw)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Penyelesaian booking ditolak dan dikembalikan ke status sedang dikerjakan.") {
		t.Fatalf("reject: %d %s", rec.Code, rec.Body.String())
	}
}

func TestVerifyRejectRequiresNote(t *testing.T) {
	svc := &stubBookingSvc{}
	r, raw := adminRouterForBooking(NewBookingHandler(svc), &model.User{ID: 2, Role: model.RoleAdmin}, "verify-token")
	rec := doAuth(t, r, http.MethodPost, "/api/admin/bookings/1/verify", `{"action":"reject"}`, raw)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d (%s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Catatan wajib diisi") {
		t.Fatalf("message: %s", rec.Body.String())
	}
}

func TestVerifyInvalidAction(t *testing.T) {
	svc := &stubBookingSvc{}
	r, raw := adminRouterForBooking(NewBookingHandler(svc), &model.User{ID: 2, Role: model.RoleAdmin}, "verify-token")
	rec := doAuth(t, r, http.MethodPost, "/api/admin/bookings/1/verify", `{"action":"nope"}`, raw)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
}

func TestVerifyBusiness422(t *testing.T) {
	svc := &stubBookingSvc{err: httperr.Validation(map[string][]string{
		"booking": {"Booking must be awaiting verification."},
	})}
	r, raw := adminRouterForBooking(NewBookingHandler(svc), &model.User{ID: 2, Role: model.RoleAdmin}, "verify-token")
	rec := doAuth(t, r, http.MethodPost, "/api/admin/bookings/1/verify", `{"action":"approve"}`, raw)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
}

func TestVerifyUnauthorized(t *testing.T) {
	svc := &stubBookingSvc{}
	r, _ := adminRouterForBooking(NewBookingHandler(svc), &model.User{ID: 2, Role: model.RoleAdmin}, "verify-token")
	rec := doAuth(t, r, http.MethodPost, "/api/admin/bookings/1/verify", `{"action":"approve"}`, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestVerifyWrongRole(t *testing.T) {
	svc := &stubBookingSvc{}
	r, raw := adminRouterForBooking(NewBookingHandler(svc), &model.User{ID: 7, Role: model.RoleCustomer}, "verify-token")
	rec := doAuth(t, r, http.MethodPost, "/api/admin/bookings/1/verify", `{"action":"approve"}`, raw)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestVerifyInternalError(t *testing.T) {
	svc := &stubBookingSvc{err: httperr.Internal(ErrBooms{})}
	r, raw := adminRouterForBooking(NewBookingHandler(svc), &model.User{ID: 2, Role: model.RoleAdmin}, "verify-token")
	rec := doAuth(t, r, http.MethodPost, "/api/admin/bookings/1/verify", `{"action":"approve"}`, raw)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}
