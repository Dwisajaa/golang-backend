package httphandler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HealthHandler serves infrastructure probes only. It holds no business logic.
type HealthHandler struct{}

func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

// Health reports service liveness. Response must be stable for probes:
//
//	{"status":"ok"}
func (h *HealthHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
