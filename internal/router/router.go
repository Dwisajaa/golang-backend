package router

import (
	"github.com/gin-gonic/gin"
	"log/slog"

	"github.com/Dwisajaa/golang-backend/internal/httphandler"
	"github.com/Dwisajaa/golang-backend/internal/middleware"
)

// New assembles the Gin engine: global middleware then routes. It owns nothing
// persistent; every dependency is passed in so tests can build a router from a
// partial set of handlers.
func New(logger *slog.Logger, health *httphandler.HealthHandler, ready *httphandler.ReadyHandler, users *httphandler.UserHandler, auth *httphandler.AuthHandler, authMW gin.HandlerFunc) *gin.Engine {
	r := gin.New()

	r.Use(gin.Recovery())
	r.Use(middleware.RequestID())
	r.Use(middleware.Logging(logger))
	r.Use(middleware.SecurityHeaders())

	r.GET("/health", health.Health)
	r.GET("/ready", ready.Ready)

	api := r.Group("/api")
	api.POST("/register", auth.Register)
	api.POST("/login", auth.Login)
	api.GET("/users/:id", users.Get)

	protected := api.Group("")
	protected.Use(authMW)
	protected.POST("/logout", auth.Logout)

	return r
}
