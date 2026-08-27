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

// TechnicianProfileService is the surface this handler consumes.
type TechnicianProfileService interface {
	GetByUserID(ctx context.Context, userID uint64) (*model.TechnicianProfile, error)
	UpdateProfile(ctx context.Context, userID uint64, in service.TechnicianProfileInput) (*model.TechnicianProfile, error)
}

type TechnicianProfileHandler struct {
	service TechnicianProfileService
}

func NewTechnicianProfileHandler(svc TechnicianProfileService) *TechnicianProfileHandler {
	return &TechnicianProfileHandler{service: svc}
}

// Get serves GET /api/technician/profile (role:technician). A missing profile
// is 404 "Technician profile not found." per Laravel.
func (h *TechnicianProfileHandler) Get(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		respondError(c, httperr.Unauthorized("Unauthenticated."))
		return
	}

	p, err := h.service.GetByUserID(c.Request.Context(), u.ID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": toTechnicianProfile(p)})
}

// Update serves PUT /api/technician/profile: first-or-create then update.
func (h *TechnicianProfileHandler) Update(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		respondError(c, httperr.Unauthorized("Unauthenticated."))
		return
	}

	var req updateTechnicianProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, httperr.BadRequest("Invalid JSON payload."))
		return
	}
	if errs := validateUpdateTechnicianProfile(req); len(errs) > 0 {
		respondError(c, httperr.Validation(errs))
		return
	}

	p, err := h.service.UpdateProfile(c.Request.Context(), u.ID, service.TechnicianProfileInput{
		Phone:          req.Phone,
		Specialization: req.Specialization,
		Address:        req.Address,
		Bio:            req.Bio,
	})
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, technicianProfileResponse{
		Message: "Technician profile updated successfully.",
		Data:    toTechnicianProfile(p),
	})
}
