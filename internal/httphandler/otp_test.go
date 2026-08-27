package httphandler

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Dwisajaa/golang-backend/internal/httperr"
	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/service"
)

type stubOtpService struct {
	res *service.LoginResult
	err error
}

func (s *stubOtpService) ResendVerificationOtp(ctx context.Context, email string) error {
	return s.err
}

func (s *stubOtpService) VerifyEmail(ctx context.Context, email, otp string) (*service.LoginResult, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.res != nil {
		return s.res, nil
	}
	return &service.LoginResult{User: &model.User{ID: 1, Email: email, Role: model.RoleCustomer}, RawToken: "otp-raw-token"}, nil
}

func otpRouter(svc OtpService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewOtpHandler(svc)
	r.POST("/api/email/verification/verify", h.Verify)
	r.POST("/api/email/verification/resend", h.Resend)
	return r
}

func TestVerifyValid(t *testing.T) {
	rec := postJSON(t, otpRouter(&stubOtpService{}), "/api/email/verification/verify",
		`{"email":"u@example.test","otp":"123456"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if body["message"] != "Email berhasil diverifikasi." {
		t.Fatalf("message mismatch: %v", body["message"])
	}
	if body["token"] != "otp-raw-token" || body["token_type"] != "Bearer" {
		t.Fatalf("token payload mismatch: %v", body)
	}
}

func TestVerifyValidationShape(t *testing.T) {
	rec := postJSON(t, otpRouter(&stubOtpService{}), "/api/email/verification/verify", `{"email":"bad","otp":"12x"}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	errs := body["errors"].(map[string]any)
	if _, hasEmail := errs["email"]; !hasEmail {
		t.Fatalf("expected email error: %v", errs)
	}
	if _, hasOtp := errs["otp"]; !hasOtp {
		t.Fatalf("expected otp error: %v", errs)
	}
}

func TestVerifyMalformedJSON(t *testing.T) {
	rec := postJSON(t, otpRouter(&stubOtpService{}), "/api/email/verification/verify", `{"email":`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestVerifyCustom422Message(t *testing.T) {
	rec := postJSON(t, otpRouter(&stubOtpService{err: httperr.Unprocessable("Kode verifikasi tidak valid.")}), "/api/email/verification/verify",
		`{"email":"u@example.test","otp":"000000"}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["message"] != "Kode verifikasi tidak valid." {
		t.Fatalf("message mismatch: %v", body)
	}
}

func TestResendReturnsGenericMessage(t *testing.T) {
	rec := postJSON(t, otpRouter(&stubOtpService{}), "/api/email/verification/resend",
		`{"email":"u@example.test"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["message"] != "Jika email terdaftar dan belum diverifikasi, kode telah dikirim ulang." {
		t.Fatalf("message mismatch: %v", body)
	}
}

func TestResendErrors(t *testing.T) {
	rec := postJSON(t, otpRouter(&stubOtpService{}), "/api/email/verification/resend", `{"email":`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("malformed json: expected 400, got %d", rec.Code)
	}
	rec = postJSON(t, otpRouter(&stubOtpService{}), "/api/email/verification/resend", `{"email":"bad"}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid email: expected 422, got %d", rec.Code)
	}
}
