package httphandler

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/Dwisajaa/golang-backend/internal/httperr"
)

// respondError converts a typed httperr into the JSON body Laravel's handler
// produces. Internal errors are logged here (with request id) and the client
// only ever sees "Server error.".
func respondError(c *gin.Context, err error) {
	he := httperr.As(err)

	status := httperr.Status(err)
	body := gin.H{"message": "Server error."}
	if he != nil {
		body = gin.H{"message": he.Message}
	}

	if status >= 500 {
		slog.Error("request_failed",
			"request_id", c.GetString("request_id"),
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", status,
			"error", err.Error(),
		)
		c.JSON(status, body)
		return
	}

	c.JSON(status, body)
}
