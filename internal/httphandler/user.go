package httphandler

import (
	"context"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Dwisajaa/golang-backend/internal/httperr"
	"github.com/Dwisajaa/golang-backend/internal/model"
)

// UserService is the minimal surface this handler consumes.
type UserService interface {
	GetUserByID(ctx context.Context, id uint64) (*model.User, error)
}

type UserHandler struct {
	service UserService
}

func NewUserHandler(svc UserService) *UserHandler {
	return &UserHandler{service: svc}
}

// Get serves GET /api/users/:id.
func (h *UserHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		// Laravel route-model binding produces 404 for a non-numeric id.
		respondError(c, httperr.NotFound("Resource not found."))
		return
	}

	u, err := h.service.GetUserByID(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(200, gin.H{"data": toUserResponse(u)})
}
