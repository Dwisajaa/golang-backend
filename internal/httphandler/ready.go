package httphandler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Pinger is the minimal dependency ReadyHandler needs to prove the database
// is reachable. Keeping it an interface lets handler tests stub the DB.
type Pinger interface {
	PingContext(ctx context.Context) error
}

// ReadyHandler reports readiness of the primary dependency (the database).
type ReadyHandler struct {
	DB Pinger
}

func NewReadyHandler(db Pinger) *ReadyHandler {
	return &ReadyHandler{DB: db}
}

// Ready is the readiness probe: 200 when the database answers a ping within a
// short timeout, 503 otherwise. Separate from /health (liveness).
func (h *ReadyHandler) Ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	if err := h.DB.PingContext(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}
