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

// CustomerProfileService is the surface this handler consumes.
type CustomerProfileService interface {
	GetByUserID(ctx context.Context, userID uint64) (*model.CustomerProfile, error)
	Upsert(ctx context.Context, userID uint64, in service.UpdateInput) (*model.CustomerProfile, error)
}

type CustomerProfileHandler struct {
	service CustomerProfileService
}

func NewCustomerProfileHandler(svc CustomerProfileService) *CustomerProfileHandler {
	return &CustomerProfileHandler{service: svc}
}

// Get serves GET /api/customer-profile (role:customer). A missing profile is
// {"data":null} like Laravel — not 404.
func (h *CustomerProfileHandler) Get(c *gin.Context) {
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
	if p == nil {
		c.JSON(http.StatusOK, gin.H{"data": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": toCustomerProfile(p)})
}

// Update serves PUT /api/customer-profile: create-or-update for the
// authenticated customer (Laravel updateOrCreate).
func (h *CustomerProfileHandler) Update(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		respondError(c, httperr.Unauthorized("Unauthenticated."))
		return
	}

	var req updateCustomerProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, httperr.BadRequest("Invalid JSON payload."))
		return
	}
	if errs := validateUpdateCustomerProfile(req); len(errs) > 0 {
		respondError(c, httperr.Validation(errs))
		return
	}

	p, err := h.service.Upsert(c.Request.Context(), u.ID, service.UpdateInput{
		FullName:   req.FullName,
		Phone:      req.Phone,
		Address:    req.Address,
		City:       req.City,
		PostalCode: req.PostalCode,
	})
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, customerProfileResponse{
		Message: "Customer profile updated successfully.",
		Data:    toCustomerProfile(p),
	})
}
