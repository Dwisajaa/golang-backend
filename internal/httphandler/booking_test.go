package httphandler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Dwisajaa/golang-backend/internal/auth"
	"github.com/Dwisajaa/golang-backend/internal/httperr"
	"github.com/Dwisajaa/golang-backend/internal/middleware"
	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/repository"
	"github.com/Dwisajaa/golang-backend/internal/service"
)

type stubBookingSvc struct {
	list *service.BookingList
	b    *model.Booking
	err  error
}

func (s *stubBookingSvc) ListByCustomer(ctx context.Context, uid uint64, p, pp int) (*service.BookingList, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.list != nil {
		return s.list, nil
	}
	return &service.BookingList{Total: 0, Page: 1, PerPage: 15}, nil
}
func (s *stubBookingSvc) Show(ctx context.Context, uid, bid uint64) (*model.Booking, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.b, nil
}
func (s *stubBookingSvc) Create(ctx context.Context, uid uint64, in service.CreateBookingInput) (*model.Booking, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.b, nil
}
func (s *stubBookingSvc) Cancel(ctx context.Context, uid, bid uint64) (*model.Booking, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.b, nil
}
func (s *stubBookingSvc) AdminList(ctx context.Context, f repository.AdminBookingFilters, p, pp int) (*service.BookingList, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.list != nil {
		return s.list, nil
	}
	return &service.BookingList{Total: 0, Page: 1, PerPage: 15}, nil
}

func sampleBooking() *model.Booking {
	svcID := uint64(5)
	return &model.Booking{
		ID: 1, BookingCode: "BJA-20261201-0001", CustomerID: 7,
		BookingDate: "2026-12-01", BookingTime: "09:00", Address: "Jl Test",
		SubtotalCents: 30000, TotalPriceCents: 30000,
		Status: model.BookingStatusPendingPayment,
		Items: []*model.BookingItem{{
			ID: 10, BookingID: 1, ServiceID: &svcID, ItemType: "service",
			ItemName: "AC Service", Quantity: 2, UnitPriceCents: 15000, SubtotalCents: 30000,
		}},
		Invoice: &model.Invoice{
			ID: 20, BookingID: 1, InvoiceNumber: "INV-BJA-20261201-0001-0001",
			SubtotalCents: 30000, TotalAmountCents: 30000, Status: model.InvoiceStatusUnpaid,
		},
	}
}

func bookingRouter(svc BookingService, user *model.User) (*gin.Engine, string) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	raw := "bk-token"
	gen := auth.NewRandomTokenGenerator()
	exp := time.Now().UTC().Add(time.Hour)
	u := user
	if u == nil {
		u = &model.User{ID: 7, Role: model.RoleCustomer}
	}
	h := NewBookingHandler(svc)

	api := r.Group("/api")
	cust := api.Group("")
	cust.Use(middleware.Auth(&fixedTokenStore{hash: gen.Hash(raw), userID: u.ID, exp: &exp}, &authUserStub{user: u}, gen))
	cust.Use(middleware.RequireRole(model.RoleCustomer))
	cust.GET("/bookings", h.List)
	cust.POST("/bookings", h.Store)
	cust.GET("/bookings/:id", h.Show)
	cust.POST("/bookings/:id/cancel", h.Cancel)

	admin := api.Group("")
	admin.Use(middleware.Auth(&fixedTokenStore{hash: gen.Hash(raw), userID: u.ID, exp: &exp}, &authUserStub{user: u}, gen))
	admin.Use(middleware.RequireRole(model.RoleAdmin, model.RoleSuperAdmin))
	admin.GET("/admin/bookings", h.AdminList)

	return r, raw
}

func TestBookingListCustomer(t *testing.T) {
	svc := &stubBookingSvc{list: &service.BookingList{
		Items: []*model.Booking{sampleBooking()}, Total: 1, Page: 1, PerPage: 15,
	}}
	r, raw := bookingRouter(svc, nil)
	rec := doAuth(t, r, http.MethodGet, "/api/bookings", "", raw)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	data := body["data"].([]any)
	if len(data) != 1 {
		t.Fatalf("items: %d", len(data))
	}
	first := data[0].(map[string]any)
	if first["subtotal"] != "300.00" {
		t.Fatalf("subtotal: %v", first["subtotal"])
	}
	if first["status"] != "pending_payment" {
		t.Fatalf("status: %v", first["status"])
	}
}

func TestBookingCreate201(t *testing.T) {
	svc := &stubBookingSvc{b: sampleBooking()}
	r, raw := bookingRouter(svc, nil)
	rec := doAuth(t, r, http.MethodPost, "/api/bookings",
		`{"item_type":"service","service_id":5,"quantity":2,"booking_date":"2026-12-01","booking_time":"09:00","address":"Jl Test"}`, raw)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Booking created successfully.") {
		t.Fatalf("msg: %s", rec.Body.String())
	}
}

func TestBookingShow403(t *testing.T) {
	svc := &stubBookingSvc{err: httperr.Forbidden("Forbidden.")}
	r, raw := bookingRouter(svc, nil)
	rec := doAuth(t, r, http.MethodGet, "/api/bookings/1", "", raw)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestBookingCancel409(t *testing.T) {
	svc := &stubBookingSvc{err: httperr.Conflict("Booking cannot be cancelled in its current status.")}
	r, raw := bookingRouter(svc, nil)
	rec := doAuth(t, r, http.MethodPost, "/api/bookings/1/cancel", `{}`, raw)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
}

func TestBookingValidation422(t *testing.T) {
	svc := &stubBookingSvc{}
	r, raw := bookingRouter(svc, nil)
	rec := doAuth(t, r, http.MethodPost, "/api/bookings", `{"item_type":"bogus","quantity":0}`, raw)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
}

func TestBookingUnauthenticated(t *testing.T) {
	svc := &stubBookingSvc{}
	r, _ := bookingRouter(svc, nil)
	rec := doAuth(t, r, http.MethodGet, "/api/bookings", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestBookingWrongRole(t *testing.T) {
	svc := &stubBookingSvc{}
	r, raw := bookingRouter(svc, &model.User{ID: 9, Role: model.RoleTechnician})
	rec := doAuth(t, r, http.MethodGet, "/api/bookings", "", raw)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestBookingAdminList(t *testing.T) {
	svc := &stubBookingSvc{list: &service.BookingList{Total: 0, Page: 1, PerPage: 15}}
	r, raw := bookingRouter(svc, &model.User{ID: 9, Role: model.RoleAdmin})
	rec := doAuth(t, r, http.MethodGet, "/api/admin/bookings", "", raw)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestBookingInternalError(t *testing.T) {
	svc := &stubBookingSvc{err: httperr.Internal(ErrBooms{})}
	r, raw := bookingRouter(svc, nil)
	rec := doAuth(t, r, http.MethodGet, "/api/bookings", "", raw)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func (s *stubBookingSvc) VerifyCompletion(ctx context.Context, bookingID uint64, action, note string) (*model.Booking, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.b, nil
}
