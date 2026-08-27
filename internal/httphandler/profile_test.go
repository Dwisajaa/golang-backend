package httphandler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

type stubProfileService struct {
	updateErr error
	pwdErr    error
}

func (s *stubProfileService) UpdateProfile(ctx context.Context, userID uint64, in service.UpdateProfileInput) (*model.User, error) {
	if s.updateErr != nil {
		return nil, s.updateErr
	}
	return &model.User{ID: userID, Name: in.Name, Email: in.Email, Role: model.RoleCustomer}, nil
}

func (s *stubProfileService) UpdatePassword(ctx context.Context, userID uint64, currentPassword, newPassword string) error {
	return s.pwdErr
}

type authUserStub struct{ user *model.User }

func (s *authUserStub) FindByID(ctx context.Context, id uint64) (*model.User, error) {
	if s.user != nil && s.user.ID == id {
		return s.user, nil
	}
	return nil, repository.ErrNotFound
}

func doAuth(t *testing.T, r *gin.Engine, method, path, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body != "" {
		reader = strings.NewReader(body)
	} else {
		reader = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func profileTestUser() *model.User {
	return &model.User{ID: 7, Name: "U", Email: "u@example.test", Role: model.RoleCustomer}
}
func TestGetProfileAuthenticated(t *testing.T) {
	// derive the hash for a known raw token so the middleware accepts it
	gen := auth.NewRandomTokenGenerator()
	raw := "profile-valid-token"
	exp := time.Now().UTC().Add(time.Hour)
	ts := &fixedTokenStore{hash: gen.Hash(raw), userID: 7, exp: &exp}
	uf := &authUserStub{user: profileTestUser()}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewProfileHandler(&stubProfileService{})
	authMW := middleware.Auth(ts, uf, gen)
	api := r.Group("/api")
	api.Use(authMW)
	api.GET("/profile", h.Get)
	api.PUT("/profile", h.Update)
	api.PUT("/profile/password", h.UpdatePassword)

	rec := doAuth(t, r, http.MethodGet, "/api/profile", "", raw)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	var body map[string]map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	u := body["user"]
	if u["id"].(float64) != 7 || u["email"] != "u@example.test" {
		t.Fatalf("unexpected user: %+v", u)
	}
	for _, secret := range []string{"password", "remember_token", "token"} {
		if _, has := u[secret]; has {
			t.Fatalf("secret %q leaked: %+v", secret, u)
		}
	}
}

func TestGetProfileUnauthenticated(t *testing.T) {
	gen := auth.NewRandomTokenGenerator()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewProfileHandler(&stubProfileService{})
	r.Use(middleware.Auth(&fixedTokenStore{hash: "unused", userID: 7}, &authUserStub{user: profileTestUser()}, gen))
	r.GET("/api/profile", h.Get)

	rec := doAuth(t, r, http.MethodGet, "/api/profile", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Unauthenticated") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestUpdateProfileValid(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewProfileHandler(&stubProfileService{})
	gen := auth.NewRandomTokenGenerator()
	raw := "profile-valid-token"
	exp := time.Now().UTC().Add(time.Hour)
	r.Use(middleware.Auth(&fixedTokenStore{hash: gen.Hash(raw), userID: 7, exp: &exp}, &authUserStub{user: profileTestUser()}, gen))
	r.PUT("/api/profile", h.Update)

	rec := doAuth(t, r, http.MethodPut, "/api/profile", `{"name":"New","email":"new@example.test"}`, raw)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["message"] != "Profile updated successfully" {
		t.Fatalf("message mismatch: %v", body["message"])
	}
}

func TestUpdateProfileValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewProfileHandler(&stubProfileService{})
	gen := auth.NewRandomTokenGenerator()
	raw := "profile-valid-token"
	exp := time.Now().UTC().Add(time.Hour)
	r.Use(middleware.Auth(&fixedTokenStore{hash: gen.Hash(raw), userID: 7, exp: &exp}, &authUserStub{user: profileTestUser()}, gen))
	r.PUT("/api/profile", h.Update)

	rec := doAuth(t, r, http.MethodPut, "/api/profile", `{"name":"","email":"bad"}`, raw)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
}

func TestUpdateProfileMalformed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewProfileHandler(&stubProfileService{})
	gen := auth.NewRandomTokenGenerator()
	raw := "profile-valid-token"
	exp := time.Now().UTC().Add(time.Hour)
	r.Use(middleware.Auth(&fixedTokenStore{hash: gen.Hash(raw), userID: 7, exp: &exp}, &authUserStub{user: profileTestUser()}, gen))
	r.PUT("/api/profile", h.Update)

	rec := doAuth(t, r, http.MethodPut, "/api/profile", `{"name":`, raw)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestUpdatePasswordValid(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewProfileHandler(&stubProfileService{})
	gen := auth.NewRandomTokenGenerator()
	raw := "profile-valid-token"
	exp := time.Now().UTC().Add(time.Hour)
	r.Use(middleware.Auth(&fixedTokenStore{hash: gen.Hash(raw), userID: 7, exp: &exp}, &authUserStub{user: profileTestUser()}, gen))
	r.PUT("/api/profile/password", h.UpdatePassword)

	rec := doAuth(t, r, http.MethodPut, "/api/profile/password", `{"current_password":"oldpass123","password":"newpassword123","password_confirmation":"newpassword123"}`, raw)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Password updated successfully") {
		t.Fatalf("message mismatch: %s", rec.Body.String())
	}
}

func TestUpdatePasswordWrongCurrent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	svc := &stubProfileService{pwdErr: httperr.Validation(map[string][]string{
		"current_password": {"The current password field does not match your password."},
	})}
	h := NewProfileHandler(svc)
	gen := auth.NewRandomTokenGenerator()
	raw := "profile-valid-token"
	exp := time.Now().UTC().Add(time.Hour)
	r.Use(middleware.Auth(&fixedTokenStore{hash: gen.Hash(raw), userID: 7, exp: &exp}, &authUserStub{user: profileTestUser()}, gen))
	r.PUT("/api/profile/password", h.UpdatePassword)

	rec := doAuth(t, r, http.MethodPut, "/api/profile/password", `{"current_password":"wrong","password":"newpassword123","password_confirmation":"newpassword123"}`, raw)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d (%s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "current password field does not match") {
		t.Fatalf("expected laravel-style current_password error: %s", rec.Body.String())
	}
}

func TestUpdatePasswordInternalError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewProfileHandler(&stubProfileService{pwdErr: httperr.Internal(ErrBooms{})})
	gen := auth.NewRandomTokenGenerator()
	raw := "profile-valid-token"
	exp := time.Now().UTC().Add(time.Hour)
	r.Use(middleware.Auth(&fixedTokenStore{hash: gen.Hash(raw), userID: 7, exp: &exp}, &authUserStub{user: profileTestUser()}, gen))
	r.PUT("/api/profile/password", h.UpdatePassword)

	rec := doAuth(t, r, http.MethodPut, "/api/profile/password", `{"current_password":"x","password":"newpassword123","password_confirmation":"newpassword123"}`, raw)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Server error.") {
		t.Fatalf("internal detail leaked: %s", rec.Body.String())
	}
}

type ErrBooms struct{}

func (ErrBooms) Error() string { return "db exploded" }

// fixedTokenStore resolves a single token hash to a user (for middleware).
type fixedTokenStore struct {
	hash   string
	userID uint64
	exp    *time.Time
}

func (s *fixedTokenStore) FindByTokenHash(ctx context.Context, hash string) (*model.PersonalAccessToken, error) {
	if hash != s.hash {
		return nil, repository.ErrNotFound
	}
	return &model.PersonalAccessToken{ID: 1, TokenableType: model.TokenableType, TokenableID: s.userID, Token: hash, ExpiresAt: s.exp}, nil
}
