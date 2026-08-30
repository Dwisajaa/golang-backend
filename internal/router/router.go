package router

import (
	"github.com/gin-gonic/gin"
	"log/slog"
	"time"

	"github.com/Dwisajaa/golang-backend/internal/httphandler"
	"github.com/Dwisajaa/golang-backend/internal/middleware"
	"github.com/Dwisajaa/golang-backend/internal/model"
)

// Options carries application-security tuning for the router.
type Options struct {
	AllowedOrigins map[string]bool
	MaxJSONBody    int64 // strict cap for JSON-only groups
	MaxUploadBody  int64 // cap for the customer group (multipart payment proofs)
}

const (
	limLogin     = "auth-login"
	limRegister  = "auth-register"
	limOtpVerify = "otp-verify"
	limOtpResend = "otp-resend"
	limBooking   = "booking-create"
	limPayment   = "payment-upload"
	minute       = time.Minute
)

// New assembles the Gin engine: global middleware (security + observability)
// then routes with per-group limits and Laravel-parity rate limiters.
func New(
	logger *slog.Logger,
	health *httphandler.HealthHandler,
	ready *httphandler.ReadyHandler,
	users *httphandler.UserHandler,
	auth *httphandler.AuthHandler,
	authMW gin.HandlerFunc,
	otp *httphandler.OtpHandler,
	profile *httphandler.ProfileHandler,
	customerProfile *httphandler.CustomerProfileHandler,
	techProfile *httphandler.TechnicianProfileHandler,
	categories *httphandler.ServiceCategoryHandler,
	services *httphandler.ServiceHandler,
	packages *httphandler.PackageHandler,
	bookings *httphandler.BookingHandler,
	invoices *httphandler.InvoiceHandler,
	payments *httphandler.PaymentHandler,
	assignments *httphandler.AssignmentHandler,
	notifications *httphandler.NotificationHandler,
	reviews *httphandler.ReviewHandler,
	sec Options,
) *gin.Engine {
	r := gin.New()

	// Outermost: panic → generic JSON 500, never a stack trace.
	r.Use(middleware.JSONRecovery(logger))
	r.Use(middleware.RequestID())
	r.Use(middleware.Logging(logger))
	r.Use(middleware.SecurityHeaders())
	if sec.AllowedOrigins != nil {
		r.Use(middleware.CORS(sec.AllowedOrigins))
	}

	r.GET("/health", health.Health)
	r.GET("/ready", ready.Ready)

	api := r.Group("/api")

	// Strict JSON (public auth + catalog read).
	strict := api.Group("")
	strict.Use(middleware.LimitBody(sec.MaxJSONBody))
	strict.POST("/register", middleware.RateLimit(limRegister, 5, minute), auth.Register)
	strict.POST("/login", middleware.RateLimit(limLogin, 10, minute), auth.Login)
	strict.POST("/email/verification/resend", middleware.RateLimit(limOtpResend, 3, 10*minute), otp.Resend)
	strict.POST("/email/verification/verify", middleware.RateLimit(limOtpVerify, 10, minute), otp.Verify)
	strict.GET("/users/:id", users.Get)
	strict.GET("/categories", categories.List)
	strict.GET("/services", services.List)
	strict.GET("/services/:id", services.Get)
	strict.GET("/packages", packages.List)
	strict.GET("/packages/:id", packages.Get)

	// Protected (any role): JSON only.
	protected := api.Group("")
	protected.Use(authMW)
	protected.Use(middleware.LimitBody(sec.MaxJSONBody))
	protected.POST("/logout", auth.Logout)
	protected.GET("/profile", profile.Get)
	protected.PUT("/profile", profile.Update)
	protected.PUT("/profile/password", profile.UpdatePassword)
	protected.GET("/notifications", notifications.List)
	protected.POST("/notifications/read-all", notifications.ReadAll)
	protected.POST("/notifications/:id/read", notifications.Read)

	// role:customer group (multipart payment proofs → larger cap).
	customer := api.Group("")
	customer.Use(authMW)
	customer.Use(middleware.RequireRole(model.RoleCustomer))
	customer.Use(middleware.LimitBody(sec.MaxUploadBody))
	customer.GET("/customer-profile", customerProfile.Get)
	customer.PUT("/customer-profile", customerProfile.Update)
	customer.GET("/bookings", bookings.List)
	customer.POST("/bookings", middleware.RateLimit(limBooking, 10, minute), bookings.Store)
	customer.GET("/bookings/:id", bookings.Show)
	customer.POST("/bookings/:id/cancel", bookings.Cancel)
	customer.GET("/invoices", invoices.List)
	customer.GET("/invoices/:id", invoices.Show)
	customer.POST("/invoices/:id/payment-proof", middleware.RateLimit(limPayment, 5, 10*minute), payments.StoreProof)
	customer.GET("/invoices/:id/payment-proof", payments.ShowProofByInvoice)
	customer.GET("/bookings/:id/review", reviews.Show)
	customer.POST("/bookings/:id/review", reviews.Store)

	// role:technician group (JSON only).
	technician := api.Group("")
	technician.Use(authMW)
	technician.Use(middleware.RequireRole(model.RoleTechnician))
	technician.Use(middleware.LimitBody(sec.MaxJSONBody))
	technician.GET("/technician/profile", techProfile.Get)
	technician.PUT("/technician/profile", techProfile.Update)
	technician.GET("/technician/jobs", assignments.ListJobs)
	technician.GET("/technician/jobs/:id", assignments.ShowJob)
	technician.POST("/technician/jobs/:id/accept", assignments.Accept)
	technician.POST("/technician/jobs/:id/reject", assignments.Reject)
	technician.POST("/technician/jobs/:id/start", assignments.Start)
	technician.POST("/technician/jobs/:id/complete", assignments.Complete)

	// role:admin,super_admin group (JSON only).
	admin := api.Group("")
	admin.Use(authMW)
	admin.Use(middleware.RequireRole(model.RoleAdmin, model.RoleSuperAdmin))
	admin.Use(middleware.LimitBody(sec.MaxJSONBody))
	admin.POST("/admin/categories", categories.Store)
	admin.PUT("/admin/categories/:id", categories.Update)
	admin.DELETE("/admin/categories/:id", categories.Destroy)
	admin.POST("/admin/services", services.Store)
	admin.PUT("/admin/services/:id", services.Update)
	admin.DELETE("/admin/services/:id", services.Destroy)
	admin.POST("/admin/packages", packages.Store)
	admin.PUT("/admin/packages/:id", packages.Update)
	admin.DELETE("/admin/packages/:id", packages.Destroy)
	admin.GET("/admin/bookings", bookings.AdminList)
	admin.POST("/admin/bookings/:id/verify", bookings.VerifyCompletion)
	admin.GET("/admin/payments", payments.AdminList)
	admin.GET("/admin/payments/:id/proof", payments.ShowProofByID)
	admin.POST("/admin/payments/:id/verify", payments.Verify)
	admin.POST("/admin/payments/:id/reject", payments.Reject)
	admin.GET("/admin/reviews", reviews.AdminList)
	admin.POST("/admin/reviews/:id/moderate", reviews.Moderate)
	admin.POST("/admin/bookings/:id/assign", assignments.Assign)

	return r
}
