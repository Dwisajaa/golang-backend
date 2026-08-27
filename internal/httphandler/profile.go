package httphandler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Dwisajaa/golang-backend/internal/httperr"
	"github.com/Dwisajaa/golang-backend/internal/middleware"
	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/service"
)

// ProfileService is the surface this handler consumes.
type ProfileService interface {
	UpdateProfile(ctx context.Context, userID uint64, in service.UpdateProfileInput) (*model.User, error)
	UpdatePassword(ctx context.Context, userID uint64, currentPassword, newPassword string) error
}

type ProfileHandler struct {
	service ProfileService
}

func NewProfileHandler(svc ProfileService) *ProfileHandler { return &ProfileHandler{service: svc} }

// Get serves GET /api/profile. The authenticated user comes from the Auth
// middleware context — never from a client-supplied id.
func (h *ProfileHandler) Get(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		respondError(c, httperr.Unauthorized("Unauthenticated."))
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": toUserResponse(u)})
}

// Update serves PUT /api/profile.
func (h *ProfileHandler) Update(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		respondError(c, httperr.Unauthorized("Unauthenticated."))
		return
	}

	var req updateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, httperr.BadRequest("Invalid JSON payload."))
		return
	}
	if errs := validateUpdateProfile(req); len(errs) > 0 {
		respondError(c, httperr.Validation(errs))
		return
	}

	updated, err := h.service.UpdateProfile(c.Request.Context(), u.ID, service.UpdateProfileInput{
		Name:  req.Name,
		Email: req.Email,
	})
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, profileResponse{Message: "Profile updated successfully", User: toUserResponse(updated)})
}

// UpdatePassword serves PUT /api/profile/password.
func (h *ProfileHandler) UpdatePassword(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		respondError(c, httperr.Unauthorized("Unauthenticated."))
		return
	}

	var req updatePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, httperr.BadRequest("Invalid JSON payload."))
		return
	}
	if errs := validateUpdatePassword(req); len(errs) > 0 {
		respondError(c, httperr.Validation(errs))
		return
	}

	if err := h.service.UpdatePassword(c.Request.Context(), u.ID, req.CurrentPassword, req.Password); err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password updated successfully"})
}
