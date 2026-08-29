package httphandler

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Dwisajaa/golang-backend/internal/httperr"
	"github.com/Dwisajaa/golang-backend/internal/middleware"
	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/repository"
	"github.com/Dwisajaa/golang-backend/internal/service"
)

type BookingService interface {
	ListByCustomer(ctx context.Context, userID uint64, page, perPage int) (*service.BookingList, error)
	Show(ctx context.Context, userID, bookingID uint64) (*model.Booking, error)
	Create(ctx context.Context, userID uint64, in service.CreateBookingInput) (*model.Booking, error)
	Cancel(ctx context.Context, userID, bookingID uint64) (*model.Booking, error)
	AdminList(ctx context.Context, filters repository.AdminBookingFilters, page, perPage int) (*service.BookingList, error)
}

type BookingHandler struct {
	service BookingService
}

func NewBookingHandler(svc BookingService) *BookingHandler { return &BookingHandler{service: svc} }

func (h *BookingHandler) List(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		respondError(c, httperr.Unauthorized("Unauthenticated."))
		return
	}
	page, perPage := parsePagination(c)
	list, err := h.service.ListByCustomer(c.Request.Context(), u.ID, page, perPage)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, buildBookingPage(c, list))
}

func (h *BookingHandler) Show(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		respondError(c, httperr.Unauthorized("Unauthenticated."))
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	b, err := h.service.Show(c.Request.Context(), u.ID, id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": toBookingData(b)})
}

func (h *BookingHandler) Store(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		respondError(c, httperr.Unauthorized("Unauthenticated."))
		return
	}
	var req createBookingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, httperr.BadRequest("Invalid JSON payload."))
		return
	}
	if errs := validateCreateBooking(req); len(errs) > 0 {
		respondError(c, httperr.Validation(errs))
		return
	}
	b, err := h.service.Create(c.Request.Context(), u.ID, toCreateBookingInput(req))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"message": "Booking created successfully.",
		"data":    toBookingData(b),
	})
}

func (h *BookingHandler) Cancel(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		respondError(c, httperr.Unauthorized("Unauthenticated."))
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	// CancelBookingRequest: optional reason (not used by controller logic beyond
	// validation — documented as deferred field; Laravel doesn't persist it)
	b, err := h.service.Cancel(c.Request.Context(), u.ID, id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "Booking cancelled successfully.",
		"data":    toBookingData(b),
	})
}

// AdminList serves GET /api/admin/bookings.
func (h *BookingHandler) AdminList(c *gin.Context) {
	page, perPage := parsePagination(c)
	filters := repository.AdminBookingFilters{
		Search: strings.TrimSpace(c.Query("search")),
		Status: strings.TrimSpace(c.Query("status")),
	}
	list, err := h.service.AdminList(c.Request.Context(), filters, page, perPage)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, buildBookingPage(c, list))
}

func parsePagination(c *gin.Context) (int, int) {
	page := 1
	if raw := c.Query("page"); raw != "" {
		if p, err := strconv.Atoi(raw); err == nil && p >= 1 {
			page = p
		}
	}
	perPage := 15
	if raw := c.Query("per_page"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n >= 1 {
			perPage = n
			if perPage > 50 {
				perPage = 50
			}
		}
	}
	return page, perPage
}
