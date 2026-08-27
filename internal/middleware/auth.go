package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/repository"
)

// Request-scoped context keys. Values live only for the lifetime of one gin
// request; there is no global/shared authentication state.
type ctxKey int

const (
	ctxKeyUser ctxKey = iota
	ctxKeyRawToken
)

// TokenStore is the token lookup the middleware needs.
type TokenStore interface {
	FindByTokenHash(ctx context.Context, tokenHash string) (*model.PersonalAccessToken, error)
}

// UserFinder loads the user that owns a token.
type UserFinder interface {
	FindByID(ctx context.Context, id uint64) (*model.User, error)
}

// TokenHasher computes the SHA-256 of a raw token.
type TokenHasher interface {
	Hash(rawToken string) string
}

// Auth authenticates a request (Sanctum-equivalent): validates the Bearer
// header, hashes the raw token, finds the token row, checks expiry, and loads
// the owning user into the request context. It must not mix role authorization
// — this middleware only answers "who is this user?".
//
// Any failure stops the chain: handler is never reached.
func Auth(store TokenStore, users UserFinder, hasher TokenHasher) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, ok := bearerToken(c)
		if !ok {
			authFail(c, "missing or malformed Authorization header")
			return
		}

		token, err := store.FindByTokenHash(c.Request.Context(), hasher.Hash(raw))
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				authFail(c, "token not found")
				return
			}
			internalFail(c, err)
			return
		}

		if isExpired(token, time.Now().UTC()) {
			authFail(c, "token expired")
			return
		}

		user, err := users.FindByID(c.Request.Context(), token.TokenableID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				authFail(c, "token owner not found")
				return
			}
			internalFail(c, err)
			return
		}

		c.Set(ctxKeyUser, user)
		c.Set(ctxKeyRawToken, raw) // request-scoped; needed only for logout revoke
		c.Next()
	}
}

// bearerToken extracts the raw token from "Authorization: Bearer <token>".
func bearerToken(c *gin.Context) (string, bool) {
	header := c.GetHeader("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return "", false
	}
	raw := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	if raw == "" {
		return "", false
	}
	return raw, true
}

// isExpired mirrors Sanctum: a token is invalid once now is on/after expires_at.
// A NULL expires_at (never-expiring tokens) stays valid.
func isExpired(t *model.PersonalAccessToken, now time.Time) bool {
	return t.ExpiresAt != nil && !t.ExpiresAt.UTC().After(now)
}

// authFail logs a security event (category only, never the token) and replies
// with the audited Laravel contract: 401 {"message":"Unauthenticated."}.
func authFail(c *gin.Context, reason string) {
	slog.Warn("auth_failed",
		"request_id", c.GetString("request_id"),
		"method", c.Request.Method,
		"path", c.Request.URL.Path,
		"client_ip", c.ClientIP(),
		"reason", reason, // broad category; never a token value
	)
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "Unauthenticated."})
}

// internalFail logs server-side detail but answers with a generic 500.
func internalFail(c *gin.Context, err error) {
	slog.Error("auth_internal_error",
		"request_id", c.GetString("request_id"),
		"method", c.Request.Method,
		"path", c.Request.URL.Path,
		"error", err.Error(),
	)
	c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "Server error."})
}

// CurrentUser returns the user stored by Auth, or false when the request is
// unauthenticated. No database access here — values come from the context.
func CurrentUser(c *gin.Context) (*model.User, bool) {
	v, ok := c.Get(ctxKeyUser)
	if !ok {
		return nil, false
	}
	u, ok := v.(*model.User)
	return u, ok
}

// CurrentRawToken returns the request's raw bearer token (stored by Auth),
// used only to revoke the current token on logout. It is never logged.
func CurrentRawToken(c *gin.Context) (string, bool) {
	v, ok := c.Get(ctxKeyRawToken)
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}
