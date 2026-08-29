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
)

type stubAssignSvc struct {
	a   *model.BookingAssignment
	err error
}

func (s *stubAssignSvc) Assign(ctx context.Context, adminID, bookingID, technicianID uint64) (*model.BookingAssignment, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.a, nil
}

func assignRouter(svc AssignmentService, user *model.User, admin bool) (*gin.Engine, string) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	raw := "asg-token"
	gen := auth.NewRandomTokenGenerator()
	exp := time.Now().UTC().Add(time.Hour)
	u := user
	if u == nil {
		u = &model.User{ID: 2, Role: model.RoleAdmin}
	}
	h := NewAssignmentHandler(svc)
	api := r.Group("/api")
	if admin {
		adm := api.Group("")
		adm.Use(middleware.Auth(&fixedTokenStore{hash: gen.Hash(raw), userID: u.ID, exp: &exp}, &authUserStub{user: u}, gen))
		adm.Use(middleware.RequireRole(model.RoleAdmin, model.RoleSuperAdmin))
		adm.POST("/admin/bookings/:id/assign", h.Assign)
	}
	return r, raw
}

func sampleAssignment() *model.BookingAssignment {
	ab := uint64(2)
	return &model.BookingAssignment{
		ID: 1, BookingID: 1, TechnicianID: 9, AssignedBy: &ab,
		Status: model.AssignmentStatusPending,
		Booking: &model.Booking{
			ID: 1, BookingCode: "BJA-1", BookingDate: "2026-12-01", BookingTime: "09:00",
			Address: "Jl", Status: model.BookingStatusTechnicianAssigned,
			Items:    []*model.BookingItem{{ID: 5, BookingID: 1, ItemType: "service", ItemName: "AC", Quantity: 1, UnitPriceCents: 10000, SubtotalCents: 10000}},
			Customer: &model.User{ID: 7, Name: "C", Email: "c@example.test"},
		},
	}
}

func TestAssign201(t *testing.T) {
	svc := &stubAssignSvc{a: sampleAssignment()}
	r, raw := assignRouter(svc, nil, true)
	rec := doAuth(t, r, http.MethodPost, "/api/admin/bookings/1/assign", `{"technician_id":9}`, raw)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Technician assigned successfully.") {
		t.Fatalf("msg: %s", rec.Body.String())
	}
}

func TestAssignValidation(t *testing.T) {
	svc := &stubAssignSvc{a: sampleAssignment()}
	r, raw := assignRouter(svc, nil, true)
	rec := doAuth(t, r, http.MethodPost, "/api/admin/bookings/1/assign", `{"technician_id":0}`, raw)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
}

func TestAssignBusiness422(t *testing.T) {
	svc := &stubAssignSvc{err: httperr.Validation(map[string][]string{
		"booking": {"Booking must be confirmed and paid before assignment."},
	})}
	r, raw := assignRouter(svc, nil, true)
	rec := doAuth(t, r, http.MethodPost, "/api/admin/bookings/1/assign", `{"technician_id":9}`, raw)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
}

func TestAssignUnauthorized(t *testing.T) {
	svc := &stubAssignSvc{a: sampleAssignment()}
	r, _ := assignRouter(svc, nil, true)
	rec := doAuth(t, r, http.MethodPost, "/api/admin/bookings/1/assign", `{"technician_id":9}`, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestAssignWrongRole(t *testing.T) {
	svc := &stubAssignSvc{a: sampleAssignment()}
	r, raw := assignRouter(svc, &model.User{ID: 7, Role: model.RoleCustomer}, true)
	rec := doAuth(t, r, http.MethodPost, "/api/admin/bookings/1/assign", `{"technician_id":9}`, raw)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestAssignNotFound(t *testing.T) {
	svc := &stubAssignSvc{err: httperr.NotFound("Resource not found.")}
	r, raw := assignRouter(svc, nil, true)
	rec := doAuth(t, r, http.MethodPost, "/api/admin/bookings/1/assign", `{"technician_id":9}`, raw)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestAssignInternalError(t *testing.T) {
	svc := &stubAssignSvc{err: httperr.Internal(ErrBooms{})}
	r, raw := assignRouter(svc, nil, true)
	rec := doAuth(t, r, http.MethodPost, "/api/admin/bookings/1/assign", `{"technician_id":9}`, raw)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}
