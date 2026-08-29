package httphandler

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Dwisajaa/golang-backend/internal/httperr"
	"github.com/Dwisajaa/golang-backend/internal/middleware"
	"github.com/Dwisajaa/golang-backend/internal/model"
)

type AssignmentService interface {
	Assign(ctx context.Context, adminID, bookingID, technicianID uint64) (*model.BookingAssignment, error)
}

type AssignmentHandler struct {
	service AssignmentService
}

func NewAssignmentHandler(svc AssignmentService) *AssignmentHandler {
	return &AssignmentHandler{service: svc}
}

// Assign serves POST /api/admin/bookings/:id/assign.
func (h *AssignmentHandler) Assign(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		respondError(c, httperr.Unauthorized("Unauthenticated."))
		return
	}
	bookingID, ok := parseID(c)
	if !ok {
		return
	}
	techIDStr := strings.TrimSpace(c.PostForm("technician_id"))
	var req assignTechnicianRequest
	if techIDStr != "" {
		id, err := strconv.ParseUint(techIDStr, 10, 64)
		if err != nil || id == 0 {
			respondError(c, httperr.Validation(map[string][]string{
				"technician_id": {"The selected technician id is invalid."},
			}))
			return
		}
		req.TechnicianID = id
	} else {
		// JSON fallback (mirrors the same request shape; Laravel binds both).
		if err := c.ShouldBindJSON(&req); err != nil {
			respondError(c, httperr.BadRequest("Invalid JSON payload."))
			return
		}
	}
	if req.TechnicianID == 0 {
		respondError(c, httperr.Validation(map[string][]string{
			"technician_id": {"The technician id field is required."},
		}))
		return
	}

	a, err := h.service.Assign(c.Request.Context(), u.ID, bookingID, req.TechnicianID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"message": "Technician assigned successfully.",
		"data":    toAssignmentData(a),
	})
}

type assignTechnicianRequest struct {
	TechnicianID uint64 `json:"technician_id"`
}
