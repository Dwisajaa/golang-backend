package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORS applies an allow-list of origins (never "*" for authenticated APIs).
// A preflight OPTIONS request is answered 204 with the allowed headers when the
// origin is allowed. When the origin is not allow-listed no CORS headers are
// emitted (browser blocks the call). Credentials are allowed because the API
// uses bearer tokens in Authorization headers.
func CORS(allowed map[string]bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && allowed[origin] {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID")
			c.Header("Access-Control-Expose-Headers", "X-Request-ID")
		}
		if c.Request.Method == http.MethodOptions && origin != "" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// OriginAllowlist converts a slice of origins into a membership map.
func OriginAllowlist(origins []string) map[string]bool {
	m := map[string]bool{}
	for _, o := range origins {
		o = strings.TrimSpace(o)
		if o != "" {
			m[o] = true
		}
	}
	return m
}
