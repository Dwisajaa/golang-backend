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

type AssignmentService interface {
	Assign(ctx context.Context, adminID, bookingID, technicianID uint64) (*model.BookingAssignment, error)
	// Technician workflow
	ListJobs(ctx context.Context, technicianID uint64, page, perPage int) (*service.JobList, error)
	ShowJob(ctx context.Context, technicianID, assignmentID uint64) (*model.BookingAssignment, error)
	Accept(ctx context.Context, technicianID, assignmentID uint64) (*model.BookingAssignment, error)
	Reject(ctx context.Context, technicianID, assignmentID uint64, reason, detail string) (*model.BookingAssignment, error)
	Start(ctx context.Context, technicianID, assignmentID uint64) (*model.BookingAssignment, error)
	Complete(ctx context.Context, technicianID, assignmentID uint64, note string) (*model.BookingAssignment, error)
}

type AssignmentHandler struct {
	service AssignmentService
}

func NewAssignmentHandler(svc AssignmentService) *AssignmentHandler {
	return &AssignmentHandler{service: svc}
}

// Assign serves POST /api/admin/bookings/:id/assign.
func (h *AssignmentHandler) Assign(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		respondError(c, httperr.Unauthorized("Unauthenticated."))
		return
	}
	bookingID, ok := parseID(c)
	if !ok {
		return
	}
	techIDStr := strings.TrimSpace(c.PostForm("technician_id"))
	var req assignTechnicianRequest
	if techIDStr != "" {
		id, err := strconv.ParseUint(techIDStr, 10, 64)
		if err != nil || id == 0 {
			respondError(c, httperr.Validation(map[string][]string{
				"technician_id": {"The selected technician id is invalid."},
			}))
			return
		}
		req.TechnicianID = id
	} else {
		// JSON fallback (mirrors the same request shape; Laravel binds both).
		if err := c.ShouldBindJSON(&req); err != nil {
			respondError(c, httperr.BadRequest("Invalid JSON payload."))
			return
		}
	}
	if req.TechnicianID == 0 {
		respondError(c, httperr.Validation(map[string][]string{
			"technician_id": {"The technician id field is required."},
		}))
		return
	}

	a, err := h.service.Assign(c.Request.Context(), u.ID, bookingID, req.TechnicianID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"message": "Technician assigned successfully.",
		"data":    toAssignmentData(a),
	})
}

type assignTechnicianRequest struct {
	TechnicianID uint64 `json:"technician_id"`
}

func (h *AssignmentHandler) technicianID(c *gin.Context) (uint64, bool) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		respondError(c, httperr.Unauthorized("Unauthenticated."))
		return 0, false
	}
	return u.ID, true
}

// ListJobs serves GET /api/technician/jobs.
func (h *AssignmentHandler) ListJobs(c *gin.Context) {
	techID, ok := h.technicianID(c)
	if !ok {
		return
	}
	page, perPage := parsePagination(c)
	list, err := h.service.ListJobs(c.Request.Context(), techID, page, perPage)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, buildJobPage(c, list))
}

// ShowJob serves GET /api/technician/jobs/:id.
func (h *AssignmentHandler) ShowJob(c *gin.Context) {
	techID, ok := h.technicianID(c)
	if !ok {
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	a, err := h.service.ShowJob(c.Request.Context(), techID, id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": toAssignmentData(a)})
}

// Accept serves POST /api/technician/jobs/:id/accept.
func (h *AssignmentHandler) Accept(c *gin.Context) {
	techID, ok := h.technicianID(c)
	if !ok {
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	a, err := h.service.Accept(c.Request.Context(), techID, id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "Assignment accepted successfully.",
		"data":    toAssignmentData(a),
	})
}

// Reject serves POST /api/technician/jobs/:id/reject.
func (h *AssignmentHandler) Reject(c *gin.Context) {
	techID, ok := h.technicianID(c)
	if !ok {
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req rejectAssignmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, httperr.BadRequest("Invalid JSON payload."))
		return
	}
	if errs := validateRejectAssignment(req); len(errs) > 0 {
		respondError(c, httperr.Validation(errs))
		return
	}
	a, err := h.service.Reject(c.Request.Context(), techID, id, req.RejectionReason, req.RejectionReasonDetail)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "Assignment rejected successfully.",
		"data":    toAssignmentData(a),
	})
}

// Start serves POST /api/technician/jobs/:id/start.
func (h *AssignmentHandler) Start(c *gin.Context) {
	techID, ok := h.technicianID(c)
	if !ok {
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	a, err := h.service.Start(c.Request.Context(), techID, id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "Job started successfully.",
		"data":    toAssignmentData(a),
	})
}

// Complete serves POST /api/technician/jobs/:id/complete.
func (h *AssignmentHandler) Complete(c *gin.Context) {
	techID, ok := h.technicianID(c)
	if !ok {
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req completeJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, httperr.BadRequest("Invalid JSON payload."))
		return
	}
	note := strings.TrimSpace(req.TechnicianNote)
	if note == "" {
		respondError(c, httperr.Validation(map[string][]string{
			"technician_note": {"The technician note field is required."},
		}))
		return
	}
	if len(note) > 2000 {
		respondError(c, httperr.Validation(map[string][]string{
			"technician_note": {"The technician note field must not be greater than 2000 characters."},
		}))
		return
	}
	a, err := h.service.Complete(c.Request.Context(), techID, id, note)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "Job completed and awaiting verification.",
		"data":    toAssignmentData(a),
	})
}

type rejectAssignmentRequest struct {
	RejectionReason       string `json:"rejection_reason"`
	RejectionReasonDetail string `json:"rejection_reason_detail"`
}

var rejectionReasons = map[string]bool{"Tidak tersedia": true, "Jadwal bentrok": true, "Lokasi terlalu jauh": true, "Masalah teknis": true, "Lainnya": true}

func validateRejectAssignment(req rejectAssignmentRequest) map[string][]string {
	errs := map[string][]string{}
	if req.RejectionReason == "" {
		errs["rejection_reason"] = []string{"The rejection reason field is required."}
	} else if !rejectionReasons[req.RejectionReason] {
		errs["rejection_reason"] = []string{"The selected rejection reason is invalid."}
	}
	if req.RejectionReasonDetail != "" && len(req.RejectionReasonDetail) > 500 {
		errs["rejection_reason_detail"] = []string{"The rejection reason detail field must not be greater than 500 characters."}
	}
	return errs
}

type completeJobRequest struct {
	TechnicianNote string `json:"technician_note"`
}

func buildJobPage(c *gin.Context, list *service.JobList) gin.H {
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
	data := make([]assignmentData, 0, len(list.Items))
	for _, a := range list.Items {
		data = append(data, toAssignmentData(a))
	}
	return gin.H{
		"data":  data,
		"links": gin.H{"first": link(1), "last": link(last), "prev": link(page - 1), "next": link(page + 1)},
		"meta":  gin.H{"current_page": page, "from": from, "last_page": last, "path": path, "per_page": list.PerPage, "to": to, "total": list.Total},
	}
}
