package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// LimitBody caps a request body at max bytes before parsing (prevents memory
// exhaustion on JSON bodies; the payment multipart handler enforces its own
// 2MB proof limit on top of this). Hitting the cap responds 413 with a generic
// message — nothing is parsed past the limit.
func LimitBody(max int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil && max > 0 {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, max)
		}
		c.Next()
	}
}
