package httphandler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Dwisajaa/golang-backend/internal/httperr"
	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/service"
)

type stubAuthService struct {
	registerErr error
	loginRes    *service.LoginResult
	loginErr    error
}

func (s *stubAuthService) Register(ctx context.Context, in service.RegisterInput) (*model.User, error) {
	if s.registerErr != nil {
		return nil, s.registerErr
	}
	return &model.User{ID: 1, Name: in.Name, Email: in.Email, Role: model.RoleCustomer}, nil
}

func (s *stubAuthService) Login(ctx context.Context, email, password string) (*service.LoginResult, error) {
	if s.loginErr != nil {
		return nil, s.loginErr
	}
	if s.loginRes != nil {
		return s.loginRes, nil
	}
	return &service.LoginResult{User: &model.User{ID: 1, Email: email, Role: model.RoleCustomer}, RawToken: "raw-token-123"}, nil
}

func (s *stubAuthService) RevokeToken(ctx context.Context, rawToken string) error { return nil }

func authRouter(svc AuthService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewAuthHandler(svc)
	r.POST("/api/register", h.Register)
	r.POST("/api/login", h.Login)
	return r
}

func postJSON(t *testing.T, r *gin.Engine, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(rec, req)
	return rec
}

func TestRegisterValid(t *testing.T) {
	rec := postJSON(t, authRouter(&stubAuthService{}), "/api/register",
		`{"name":"Dwi","email":"dwi@example.test","password":"password123","password_confirmation":"password123"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if body["requires_verification"] != true {
		t.Fatalf("expected requires_verification=true, got %v", body["requires_verification"])
	}
	if body["message"] != "Registration successful. Verification code sent to your email." {
		t.Fatalf("message mismatch: %v", body["message"])
	}
	if _, has := body["token"]; has {
		t.Fatal("register must not return a token (Laravel parity)")
	}
}

func TestRegisterValidationErrorsMimicLaravel(t *testing.T) {
	rec := postJSON(t, authRouter(&stubAuthService{}), "/api/register", `{"name":"","email":"bad","password":"123"}`)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if body["message"] != "The given data was invalid." {
		t.Fatalf("message mismatch: %v", body["message"])
	}
	errs, ok := body["errors"].(map[string]any)
	if !ok {
		t.Fatalf("missing errors map: %v", body)
	}
	if len(errs["name"].([]any)) == 0 || len(errs["email"].([]any)) == 0 {
		t.Fatalf("expected name+email errors: %v", errs)
	}
}

func TestRegisterMalformedJSON(t *testing.T) {
	rec := postJSON(t, authRouter(&stubAuthService{}), "/api/register", `{"name":`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on malformed JSON, got %d", rec.Code)
	}
}

func TestRegisterDuplicateEmail(t *testing.T) {
	dupErr := httperr.Validation(map[string][]string{"email": {"The email has already been taken."}})
	rec := postJSON(t, authRouter(&stubAuthService{registerErr: dupErr}), "/api/register",
		`{"name":"Dwi","email":"dup@example.test","password":"password123","password_confirmation":"password123"}`)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	errs := body["errors"].(map[string]any)
	if errs["email"].([]any)[0] != "The email has already been taken." {
		t.Fatalf("expected taken-email message, got %v", errs["email"])
	}
}

func TestLoginValid(t *testing.T) {
	rec := postJSON(t, authRouter(&stubAuthService{}), "/api/login",
		`{"email":"a@example.test","password":"password123"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if body["token_type"] != "Bearer" {
		t.Fatalf("token_type mismatch: %v", body["token_type"])
	}
	if body["token"] != "raw-token-123" {
		t.Fatalf("raw token missing: %v", body["token"])
	}
	if _, has := body["token_hash"]; has {
		t.Fatal("token hash leaked")
	}
	if _, has := body["password"]; has {
		t.Fatal("password leaked")
	}
}

func TestLoginInvalidCredentials(t *testing.T) {
	rec := postJSON(t, authRouter(&stubAuthService{loginErr: service.InvalidCredentialsError{}}), "/api/login",
		`{"email":"a@example.test","password":"wrong"}`)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["message"] != "The provided credentials are incorrect." {
		t.Fatalf("message mismatch: %v", body["message"])
	}
}

func TestLoginUnverifiedEmail(t *testing.T) {
	u := &model.User{ID: 7, Name: "U", Email: "unv@example.test", Role: model.RoleCustomer}
	rec := postJSON(t, authRouter(&stubAuthService{loginErr: service.EmailUnverifiedError{User: u}}), "/api/login",
		`{"email":"unv@example.test","password":"password123"}`)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if body["message"] != "Email belum diverifikasi." {
		t.Fatalf("message mismatch: %v", body["message"])
	}
	user := body["user"].(map[string]any)
	if user["id"].(float64) != 7 {
		t.Fatalf("unverified 403 must carry the user: %v", body)
	}
}

func TestLoginValidationErrors(t *testing.T) {
	rec := postJSON(t, authRouter(&stubAuthService{}), "/api/login", `{"email":"","password":""}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
}
