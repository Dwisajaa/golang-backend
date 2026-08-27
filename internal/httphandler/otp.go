package httphandler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Dwisajaa/golang-backend/internal/httperr"
	"github.com/Dwisajaa/golang-backend/internal/service"
)

// OtpService is the surface this handler consumes.
type OtpService interface {
	ResendVerificationOtp(ctx context.Context, email string) error
	VerifyEmail(ctx context.Context, email, otp string) (*service.LoginResult, error)
}

type OtpHandler struct {
	service OtpService
}

func NewOtpHandler(svc OtpService) *OtpHandler { return &OtpHandler{service: svc} }

// Resend serves POST /api/email/verification/resend.
func (h *OtpHandler) Resend(c *gin.Context) {
	var req resendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, httperr.BadRequest("Invalid JSON payload."))
		return
	}
	if errs := validateResend(req); len(errs) > 0 {
		respondError(c, httperr.Validation(errs))
		return
	}

	// Resend ignores unknown users and already-verified users (Laravel parity).
	_ = h.service.ResendVerificationOtp(c.Request.Context(), req.Email)

	c.JSON(http.StatusOK, gin.H{
		"message": "Jika email terdaftar dan belum diverifikasi, kode telah dikirim ulang.",
	})
}

// Verify serves POST /api/email/verification/verify.
func (h *OtpHandler) Verify(c *gin.Context) {
	var req verifyEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, httperr.BadRequest("Invalid JSON payload."))
		return
	}
	if errs := validateVerifyEmail(req); len(errs) > 0 {
		respondError(c, httperr.Validation(errs))
		return
	}

	res, err := h.service.VerifyEmail(c.Request.Context(), req.Email, req.Otp)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, loginResponse{
		Message:   "Email berhasil diverifikasi.",
		User:      toUserResponse(res.User),
		Token:     res.RawToken,
		TokenType: "Bearer",
	})
}
