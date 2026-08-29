package router

import (
	"github.com/gin-gonic/gin"
	"log/slog"

	"github.com/Dwisajaa/golang-backend/internal/httphandler"
	"github.com/Dwisajaa/golang-backend/internal/middleware"
	"github.com/Dwisajaa/golang-backend/internal/model"
)

// New assembles the Gin engine: global middleware then routes. It owns nothing
// persistent; every dependency is passed in so tests can build a router from a
// partial set of handlers.
func New(logger *slog.Logger, health *httphandler.HealthHandler, ready *httphandler.ReadyHandler, users *httphandler.UserHandler, auth *httphandler.AuthHandler, authMW gin.HandlerFunc, otp *httphandler.OtpHandler, profile *httphandler.ProfileHandler, customerProfile *httphandler.CustomerProfileHandler, techProfile *httphandler.TechnicianProfileHandler, categories *httphandler.ServiceCategoryHandler, services *httphandler.ServiceHandler, packages *httphandler.PackageHandler) *gin.Engine {
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
	api.POST("/email/verification/resend", otp.Resend)
	api.POST("/email/verification/verify", otp.Verify)
	api.GET("/users/:id", users.Get)
	api.GET("/categories", categories.List)
	api.GET("/services", services.List)
	api.GET("/services/:id", services.Get)
	api.GET("/packages", packages.List)
	api.GET("/packages/:id", packages.Get)

	protected := api.Group("")
	protected.Use(authMW)
	protected.POST("/logout", auth.Logout)
	protected.GET("/profile", profile.Get)
	protected.PUT("/profile", profile.Update)
	protected.PUT("/profile/password", profile.UpdatePassword)

	// role:customer group mirrors Laravel's role:customer route group.
	customer := api.Group("")
	customer.Use(authMW)
	customer.Use(middleware.RequireRole(model.RoleCustomer))
	customer.GET("/customer-profile", customerProfile.Get)
	customer.PUT("/customer-profile", customerProfile.Update)

	// role:technician group mirrors Laravel's role:technician route group.
	technician := api.Group("")
	technician.Use(authMW)
	technician.Use(middleware.RequireRole(model.RoleTechnician))
	technician.GET("/technician/profile", techProfile.Get)
	technician.PUT("/technician/profile", techProfile.Update)

	// admin group mirrors Laravel's role:admin,super_admin group (catalog).
	admin := api.Group("")
	admin.Use(authMW)
	admin.Use(middleware.RequireRole(model.RoleAdmin, model.RoleSuperAdmin))
	admin.POST("/admin/categories", categories.Store)
	admin.PUT("/admin/categories/:id", categories.Update)
	admin.DELETE("/admin/categories/:id", categories.Destroy)
	admin.POST("/admin/services", services.Store)
	admin.PUT("/admin/services/:id", services.Update)
	admin.DELETE("/admin/services/:id", services.Destroy)
	admin.POST("/admin/packages", packages.Store)
	admin.PUT("/admin/packages/:id", packages.Update)
	admin.DELETE("/admin/packages/:id", packages.Destroy)

	return r
}
