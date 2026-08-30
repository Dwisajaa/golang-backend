package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimit is a single-instance, fixed-window, per-client-IP limiter for
// sensitive endpoints, mirroring Laravel's ThrottleRequests parameters
// (audited: auth-login 10/min, auth-register 5/min, otp-verify 10/min,
// otp-resend 3/10min, password-reset 5/10min, booking-create 10/min,
// payment-upload 5/10min). It is race-safe (mutex) and stateless across
// instances — distributed limiting is a production-phase concern.
func RateLimit(name string, limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !take(name, c.ClientIP(), limit, window) {
			c.Header("Retry-After", itoa(int(window.Seconds())))
			c.AbortWithStatusJSON(http.StatusTooManyRequests,
				gin.H{"message": "Too many requests. Please try again later."})
			return
		}
		c.Next()
	}
}

// limiterWindow is a fixed-window bucket keyed by limiter name + IP.
type limiterWindow struct {
	mu    sync.Mutex
	items map[string]*windowEntry // "name|ip"
}

type windowEntry struct {
	start time.Time
	count int
}

var limiter = &limiterWindow{items: map[string]*windowEntry{}}

func take(name, ip string, limit int, window time.Duration) bool {
	key := name + "|" + ip
	now := time.Now()
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	// prune expired windows lazily
	for k, e := range limiter.items {
		if now.Sub(e.start) > window {
			delete(limiter.items, k)
		}
	}
	e, ok := limiter.items[key]
	if !ok {
		limiter.items[key] = &windowEntry{start: now, count: 1}
		return true
	}
	if now.Sub(e.start) >= window {
		e.start = now
		e.count = 1
		return true
	}
	e.count++
	return e.count <= limit
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	const digits = "0123456789"
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = digits[n%10]
		n /= 10
	}
	return string(b[i:])
}
