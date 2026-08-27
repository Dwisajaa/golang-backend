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
	"github.com/Dwisajaa/golang-backend/internal/service"
)

type stubCProfileService struct {
	profile  *model.CustomerProfile
	getErr   error
	upserted *model.CustomerProfile
}

func (s *stubCProfileService) GetByUserID(ctx context.Context, userID uint64) (*model.CustomerProfile, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.profile, nil
}

func (s *stubCProfileService) Upsert(ctx context.Context, userID uint64, in service.UpdateInput) (*model.CustomerProfile, error) {
	s.upserted = &model.CustomerProfile{ID: 9, UserID: userID, FullName: in.FullName, Phone: in.Phone, Address: in.Address, City: in.City}
	return s.upserted, nil
}

func customerRouter(svc CustomerProfileService, user *model.User) (*gin.Engine, string) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	gen := auth.NewRandomTokenGenerator()
	raw := "customer-profile-token"
	exp := time.Now().UTC().Add(time.Hour)
	h := NewCustomerProfileHandler(svc)

	api := r.Group("/api")
	group := api.Group("")
	group.Use(middleware.Auth(&fixedTokenStore{hash: gen.Hash(raw), userID: user.ID, exp: &exp}, &authUserStub{user: user}, gen))
	group.Use(middleware.RequireRole(model.RoleCustomer))
	group.GET("/customer-profile", h.Get)
	group.PUT("/customer-profile", h.Update)
	return r, raw
}

func TestCustomerProfileGetFound(t *testing.T) {
	svc := &stubCProfileService{profile: &model.CustomerProfile{
		ID: 1, UserID: 7, FullName: "Dev Customer", Phone: "0812", Address: "Jl. X", City: "Jakarta",
	}}
	r, raw := customerRouter(svc, &model.User{ID: 7, Role: model.RoleCustomer})
	rec := doAuth(t, r, http.MethodGet, "/api/customer-profile", "", raw)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	var body map[string]map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	data := body["data"]
	if data["full_name"] != "Dev Customer" || data["is_complete"] != true {
		t.Fatalf("unexpected profile payload: %+v", data)
	}
}

func TestCustomerProfileGetMissingIsNull(t *testing.T) {
	svc := &stubCProfileService{profile: nil}
	r, raw := customerRouter(svc, &model.User{ID: 7, Role: model.RoleCustomer})
	rec := doAuth(t, r, http.MethodGet, "/api/customer-profile", "", raw)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with data:null, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"data":null`) {
		t.Fatalf("expected data:null, got %s", rec.Body.String())
	}
}

func TestCustomerProfileGetUnauthenticated(t *testing.T) {
	svc := &stubCProfileService{}
	r, _ := customerRouter(svc, &model.User{ID: 7, Role: model.RoleCustomer})
	rec := doAuth(t, r, http.MethodGet, "/api/customer-profile", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestCustomerProfileWrongRoleForbidden(t *testing.T) {
	// a technician is authenticated but must not reach customer profile routes
	svc := &stubCProfileService{}
	r, raw := customerRouter(svc, &model.User{ID: 8, Role: model.RoleTechnician})
	rec := doAuth(t, r, http.MethodGet, "/api/customer-profile", "", raw)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d (%s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Forbidden.") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestCustomerProfileUpdateValid(t *testing.T) {
	svc := &stubCProfileService{}
	r, raw := customerRouter(svc, &model.User{ID: 7, Role: model.RoleCustomer})
	rec := doAuth(t, r, http.MethodPut, "/api/customer-profile",
		`{"full_name":"A","phone":"0812","address":"Jl","city":"Jkt","postal_code":"12345"}`, raw)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["message"] != "Customer profile updated successfully." {
		t.Fatalf("message mismatch: %v", body["message"])
	}
}

func TestCustomerProfileUpdateValidation(t *testing.T) {
	svc := &stubCProfileService{}
	r, raw := customerRouter(svc, &model.User{ID: 7, Role: model.RoleCustomer})
	rec := doAuth(t, r, http.MethodPut, "/api/customer-profile", `{"full_name":"","phone":"","address":"","city":""}`, raw)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	errs := body["errors"].(map[string]any)
	if len(errs) != 4 {
		t.Fatalf("expected 4 field errors, got %v", errs)
	}
}

func TestCustomerProfileMalformed(t *testing.T) {
	svc := &stubCProfileService{}
	r, raw := customerRouter(svc, &model.User{ID: 7, Role: model.RoleCustomer})
	rec := doAuth(t, r, http.MethodPut, "/api/customer-profile", `{"full_name":`, raw)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestCustomerProfileInternalError(t *testing.T) {
	svc := &stubCProfileService{getErr: httperr.Internal(ErrBooms{})}
	r, raw := customerRouter(svc, &model.User{ID: 7, Role: model.RoleCustomer})
	rec := doAuth(t, r, http.MethodGet, "/api/customer-profile", "", raw)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Server error.") {
		t.Fatalf("detail leaked: %s", rec.Body.String())
	}
}
