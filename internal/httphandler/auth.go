package httphandler

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Dwisajaa/golang-backend/internal/httperr"
	"github.com/Dwisajaa/golang-backend/internal/middleware"
	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/service"
)

// AuthService is the surface this handler consumes.
type AuthService interface {
	Register(ctx context.Context, in service.RegisterInput) (*model.User, error)
	Login(ctx context.Context, email, password string) (*service.LoginResult, error)
	RevokeToken(ctx context.Context, rawToken string) error
}

type AuthHandler struct {
	service AuthService
}

func NewAuthHandler(svc AuthService) *AuthHandler {
	return &AuthHandler{service: svc}
}

// Register serves POST /api/register.
func (h *AuthHandler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, httperr.BadRequest("Invalid JSON payload."))
		return
	}
	if errs := validateRegister(req); len(errs) > 0 {
		respondError(c, httperr.Validation(errs))
		return
	}

	u, err := h.service.Register(c.Request.Context(), service.RegisterInput{
		Name:     req.Name,
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, registerResponse{
		Message:              "Registration successful. Verification code sent to your email.",
		RequiresVerification: true,
		User:                 toUserResponse(u),
	})
}

// Login serves POST /api/login.
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, httperr.BadRequest("Invalid JSON payload."))
		return
	}
	if errs := validateLogin(req); len(errs) > 0 {
		respondError(c, httperr.Validation(errs))
		return
	}

	res, err := h.service.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		var unverified service.EmailUnverifiedError
		if errors.As(err, &unverified) {
			c.JSON(http.StatusForbidden, unverifiedResponse{
				Message:              "Email belum diverifikasi.",
				RequiresVerification: true,
				User: unverifiedUserData{
					ID:              unverified.User.ID,
					Name:            unverified.User.Name,
					Email:           unverified.User.Email,
					EmailVerifiedAt: timeMicro{t: unverified.User.EmailVerifiedAt},
				},
			})
			return
		}
		if errors.Is(err, service.InvalidCredentialsError{}) {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "The provided credentials are incorrect."})
			return
		}
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, loginResponse{
		Message:   "Login successful",
		User:      toUserResponse(res.User),
		Token:     res.RawToken,
		TokenType: "Bearer",
	})
}

// Logout serves POST /api/logout (protected by the auth middleware). It revokes
// the request's current token. A missing token is impossible behind the
// middleware; service behavior keeps the Laravel parity of success-on-missing.
func (h *AuthHandler) Logout(c *gin.Context) {
	raw, ok := middleware.CurrentRawToken(c)
	if !ok {
		respondError(c, httperr.Unauthorized("Unauthenticated."))
		return
	}
	if err := h.service.RevokeToken(c.Request.Context(), raw); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Logout successful"})
}
