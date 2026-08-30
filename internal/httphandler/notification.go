package httphandler

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Dwisajaa/golang-backend/internal/httperr"
	"github.com/Dwisajaa/golang-backend/internal/middleware"
	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/service"
)

type NotificationService interface {
	List(ctx context.Context, userID uint64, page, perPage int) (*service.NotificationList, error)
	Read(ctx context.Context, userID uint64, id string) (*model.Notification, error)
	ReadAll(ctx context.Context, userID uint64) error
}

type NotificationHandler struct {
	service NotificationService
}

func NewNotificationHandler(svc NotificationService) *NotificationHandler {
	return &NotificationHandler{service: svc}
}

// List serves GET /api/notifications (any authenticated user).
func (h *NotificationHandler) List(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		respondError(c, httperr.Unauthorized("Unauthenticated."))
		return
	}
	page, perPage := parsePagination(c)
	list, err := h.service.List(c.Request.Context(), u.ID, page, perPage)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, buildNotificationPage(c, list))
}

// Read serves POST /api/notifications/:id/read.
func (h *NotificationHandler) Read(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		respondError(c, httperr.Unauthorized("Unauthenticated."))
		return
	}
	id := c.Param("id")
	if id == "" {
		respondError(c, httperr.NotFound("Resource not found."))
		return
	}
	n, err := h.service.Read(c.Request.Context(), u.ID, id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "Notification marked as read.",
		"data":    toNotificationData(n),
	})
}

// ReadAll serves POST /api/notifications/read-all.
func (h *NotificationHandler) ReadAll(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		respondError(c, httperr.Unauthorized("Unauthenticated."))
		return
	}
	if err := h.service.ReadAll(c.Request.Context(), u.ID); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "All notifications marked as read."})
}

// notificationData mirrors NotificationResource.
type notificationData struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Title     *string   `json:"title"`
	Message   *string   `json:"message"`
	Data      notifData `json:"data"`
	ReadAt    timeMicro `json:"read_at"`
	CreatedAt timeMicro `json:"created_at"`
}

type notifData struct {
	BookingID    *uint64 `json:"booking_id"`
	InvoiceID    *uint64 `json:"invoice_id"`
	PaymentID    *uint64 `json:"payment_id"`
	AssignmentID *uint64 `json:"assignment_id"`
	ActionURL    *string `json:"action_url"`
}

func toNotificationData(n *model.Notification) notificationData {
	d := notificationData{
		ID: n.ID, Type: n.Data.Event,
		Title: &n.Data.Title, Message: &n.Data.Message,
		Data: notifData{BookingID: n.Data.BookingID, InvoiceID: n.Data.InvoiceID,
			PaymentID: n.Data.PaymentID, AssignmentID: n.Data.AssignmentID, ActionURL: n.Data.ActionURL},
		ReadAt: timeMicro{t: n.ReadAt}, CreatedAt: timeMicro{t: n.CreatedAt},
	}
	if n.Data.Title == "" {
		d.Title = nil
	}
	if n.Data.Message == "" {
		d.Message = nil
	}
	return d
}

func buildNotificationPage(c *gin.Context, list *service.NotificationList) gin.H {
	path := c.Request.URL.Path
	page := list.Page
	last := 0
	if list.PerPage > 0 {
		last = (list.Total + list.PerPage - 1) / list.PerPage
	}
	from, to := 0, 0
	if list.Total > 0 {
		from = (page-1)*list.PerPage + 1
		to = from + len(list.Items) - 1
	}
	link := func(p int) any {
		if p >= 1 && p <= last {
			return path + "?page=" + strconv.Itoa(p)
		}
		return nil
	}
	data := make([]notificationData, 0, len(list.Items))
	for _, n := range list.Items {
		data = append(data, toNotificationData(n))
	}
	return gin.H{
		"data":  data,
		"links": gin.H{"first": link(1), "last": link(last), "prev": link(page - 1), "next": link(page + 1)},
		"meta":  gin.H{"current_page": page, "from": from, "last_page": last, "path": path, "per_page": list.PerPage, "to": to, "total": list.Total},
	}
}
