package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
)

// JSONRecovery converts panics into generic JSON 500 (no stack trace to the
// client) and logs the stack with the request id server-side. Replaces
// gin.Recovery so the 500 body matches the app's error contract and never
// leaks internals.
func JSONRecovery(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("panic_recovered",
					"request_id", c.GetString("request_id"),
					"method", c.Request.Method,
					"path", c.Request.URL.Path,
					"panic", r,
					"stack", string(debug.Stack()),
				)
				c.AbortWithStatusJSON(http.StatusInternalServerError,
					gin.H{"message": "Server error."})
			}
		}()
		c.Next()
	}
}
