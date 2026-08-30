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
	"github.com/Dwisajaa/golang-backend/internal/service"
)

type ReviewService interface {
	Show(ctx context.Context, userID, bookingID uint64) (*model.Review, error)
	Create(ctx context.Context, userID, bookingID uint64, rating int, comment string) (*model.Review, error)
	AdminList(ctx context.Context, status string, page, perPage int) (*service.ReviewList, error)
	Moderate(ctx context.Context, reviewID uint64, status string) (*model.Review, error)
}

type ReviewHandler struct {
	service ReviewService
}

func NewReviewHandler(svc ReviewService) *ReviewHandler { return &ReviewHandler{service: svc} }

// Show serves GET /api/bookings/:id/review (customer).
func (h *ReviewHandler) Show(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		respondError(c, httperr.Unauthorized("Unauthenticated."))
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	rv, err := h.service.Show(c.Request.Context(), u.ID, id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": toReviewData(rv)})
}

// Store serves POST /api/bookings/:id/review (customer).
func (h *ReviewHandler) Store(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		respondError(c, httperr.Unauthorized("Unauthenticated."))
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req storeReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, httperr.BadRequest("Invalid JSON payload."))
		return
	}
	validRating := false
	for _, r := range []int{1, 2, 3, 4, 5} {
		if req.Rating == r {
			validRating = true
		}
	}
	if !validRating {
		respondError(c, httperr.Validation(map[string][]string{"rating": {"The selected rating is invalid."}}))
		return
	}
	comment := strings.TrimSpace(req.Comment)
	if len(comment) > 1000 {
		respondError(c, httperr.Validation(map[string][]string{"comment": {"The comment field must not be greater than 1000 characters."}}))
		return
	}
	rv, err := h.service.Create(c.Request.Context(), u.ID, id, req.Rating, comment)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"message": "Review submitted successfully.",
		"data":    toReviewData(rv),
	})
}

// AdminList serves GET /api/admin/reviews.
func (h *ReviewHandler) AdminList(c *gin.Context) {
	page, perPage := parsePagination(c)
	status := strings.TrimSpace(c.Query("status"))
	list, err := h.service.AdminList(c.Request.Context(), status, page, perPage)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, buildReviewPage(c, list))
}

// Moderate serves POST /api/admin/reviews/:id/moderate.
func (h *ReviewHandler) Moderate(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req moderateReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, httperr.BadRequest("Invalid JSON payload."))
		return
	}
	validStatus := false
	for _, s := range model.ReviewStatuses {
		if req.Status == s {
			validStatus = true
		}
	}
	if !validStatus {
		respondError(c, httperr.Validation(map[string][]string{"status": {"The selected status is invalid."}}))
		return
	}
	rv, err := h.service.Moderate(c.Request.Context(), id, req.Status)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "Review moderated successfully.",
		"data":    toReviewData(rv),
	})
}

type storeReviewRequest struct {
	Rating  int    `json:"rating"`
	Comment string `json:"comment"`
}

type moderateReviewRequest struct {
	Status string `json:"status"`
}

// reviewData mirrors ReviewResource.
type reviewData struct {
	ID         uint64      `json:"id"`
	BookingID  uint64      `json:"booking_id"`
	Rating     int         `json:"rating"`
	Comment    *string     `json:"comment"`
	Status     string      `json:"status"`
	CreatedAt  timeMicro   `json:"created_at"`
	Customer   *reviewUser `json:"customer"`
	Technician *reviewUser `json:"technician"`
}

type reviewUser struct {
	ID   uint64 `json:"id"`
	Name string `json:"name"`
}

func toReviewData(rv *model.Review) reviewData {
	d := reviewData{
		ID: rv.ID, BookingID: rv.BookingID, Rating: rv.Rating,
		Comment: rv.Comment, Status: rv.Status, CreatedAt: timeMicro{t: rv.CreatedAt},
	}
	if rv.Customer != nil {
		d.Customer = &reviewUser{ID: rv.Customer.ID, Name: rv.Customer.Name}
	}
	if rv.Technician != nil {
		d.Technician = &reviewUser{ID: rv.Technician.ID, Name: rv.Technician.Name}
	}
	return d
}

func buildReviewPage(c *gin.Context, list *service.ReviewList) gin.H {
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
	data := make([]reviewData, 0, len(list.Items))
	for _, rv := range list.Items {
		data = append(data, toReviewData(rv))
	}
	return gin.H{
		"data":  data,
		"links": gin.H{"first": link(1), "last": link(last), "prev": link(page - 1), "next": link(page + 1)},
		"meta":  gin.H{"current_page": page, "from": from, "last_page": last, "path": path, "per_page": list.PerPage, "to": to, "total": list.Total},
	}
}
