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

type stubTechService struct {
	profile *model.TechnicianProfile
	getErr  error
	upErr   error
}

func (s *stubTechService) GetByUserID(ctx context.Context, userID uint64) (*model.TechnicianProfile, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.profile, nil
}

func (s *stubTechService) UpdateProfile(ctx context.Context, userID uint64, in service.TechnicianProfileInput) (*model.TechnicianProfile, error) {
	if s.upErr != nil {
		return nil, s.upErr
	}
	code := "TECH-0001"
	return &model.TechnicianProfile{
		ID: 3, UserID: userID, TechnicianCode: code, IsActive: true,
	}, nil
}

func techRouter(svc TechnicianProfileService, user *model.User) (*gin.Engine, string) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	gen := auth.NewRandomTokenGenerator()
	raw := "tech-profile-token"
	exp := time.Now().UTC().Add(time.Hour)
	h := NewTechnicianProfileHandler(svc)

	api := r.Group("/api")
	g := api.Group("")
	g.Use(middleware.Auth(&fixedTokenStore{hash: gen.Hash(raw), userID: user.ID, exp: &exp}, &authUserStub{user: user}, gen))
	g.Use(middleware.RequireRole(model.RoleTechnician))
	g.GET("/technician/profile", h.Get)
	g.PUT("/technician/profile", h.Update)
	return r, raw
}

func techProfile() *model.TechnicianProfile {
	phone := "0811"
	return &model.TechnicianProfile{ID: 3, UserID: 9, TechnicianCode: "TECH-0001", Phone: &phone, IsActive: true}
}

func TestTechProfileGetFound(t *testing.T) {
	svc := &stubTechService{profile: techProfile()}
	r, raw := techRouter(svc, &model.User{ID: 9, Role: model.RoleTechnician})
	rec := doAuth(t, r, http.MethodGet, "/api/technician/profile", "", raw)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	var body map[string]map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	data := body["data"]
	if data["technician_code"] != "TECH-0001" || data["is_active"] != true {
		t.Fatalf("unexpected payload: %+v", data)
	}
}

func TestTechProfileGetMissingIs404(t *testing.T) {
	svc := &stubTechService{getErr: httperr.NotFound("Technician profile not found.")}
	r, raw := techRouter(svc, &model.User{ID: 9, Role: model.RoleTechnician})
	rec := doAuth(t, r, http.MethodGet, "/api/technician/profile", "", raw)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Technician profile not found.") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestTechProfileUnauthenticated(t *testing.T) {
	svc := &stubTechService{}
	r, _ := techRouter(svc, &model.User{ID: 9, Role: model.RoleTechnician})
	rec := doAuth(t, r, http.MethodGet, "/api/technician/profile", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestTechProfileWrongRole403(t *testing.T) {
	svc := &stubTechService{}
	r, raw := techRouter(svc, &model.User{ID: 5, Role: model.RoleCustomer})
	rec := doAuth(t, r, http.MethodGet, "/api/technician/profile", "", raw)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d (%s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Forbidden.") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestTechProfileUpdateValid(t *testing.T) {
	svc := &stubTechService{}
	r, raw := techRouter(svc, &model.User{ID: 9, Role: model.RoleTechnician})
	rec := doAuth(t, r, http.MethodPut, "/api/technician/profile",
		`{"phone":"0811","specialization":"AC","address":"Jl","bio":"x"}`, raw)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["message"] != "Technician profile updated successfully." {
		t.Fatalf("message mismatch: %v", body["message"])
	}
}

func TestTechProfileUpdateValidation(t *testing.T) {
	svc := &stubTechService{}
	r, raw := techRouter(svc, &model.User{ID: 9, Role: model.RoleTechnician})
	long := strings.Repeat("a", 21)
	rec := doAuth(t, r, http.MethodPut, "/api/technician/profile", `{"phone":"`+long+`"}`, raw)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if _, has := body["errors"].(map[string]any)["phone"]; !has {
		t.Fatalf("expected phone error: %v", body)
	}
}

func TestTechProfileMalformed(t *testing.T) {
	svc := &stubTechService{}
	r, raw := techRouter(svc, &model.User{ID: 9, Role: model.RoleTechnician})
	rec := doAuth(t, r, http.MethodPut, "/api/technician/profile", `{"phone":`, raw)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestTechProfileInternalError(t *testing.T) {
	svc := &stubTechService{getErr: httperr.Internal(ErrBooms{})}
	r, raw := techRouter(svc, &model.User{ID: 9, Role: model.RoleTechnician})
	rec := doAuth(t, r, http.MethodGet, "/api/technician/profile", "", raw)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Server error.") {
		t.Fatalf("detail leaked: %s", rec.Body.String())
	}
}
