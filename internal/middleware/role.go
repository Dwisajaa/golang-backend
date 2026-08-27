package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RequireRole is the Go equivalent of Laravel's RoleMiddleware: an
// authenticated user's role must be in the allowed list, otherwise 403
// "Forbidden." (audited from app/Http/Middleware/RoleMiddleware.php).
// It is authorization — run AFTER Auth (which establishes identity).
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		u, ok := CurrentUser(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "Unauthenticated."})
			return
		}
		for _, role := range roles {
			if u.Role == role {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"message": "Forbidden."})
	}
}
