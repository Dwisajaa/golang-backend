package httphandler

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Dwisajaa/golang-backend/internal/httperr"
	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/service"
)

// ServiceService is the surface the service handlers consume.
type ServiceService interface {
	List(ctx context.Context, categoryID *uint64, search string, page, perPage int) (*service.ServiceList, error)
	Get(ctx context.Context, id uint64) (*model.Service, error)
	Create(ctx context.Context, in service.ServiceInput) (*model.Service, error)
	Update(ctx context.Context, id uint64, in service.ServiceInput) (*model.Service, error)
	Delete(ctx context.Context, id uint64) error
}

type ServiceHandler struct {
	service ServiceService
}

func NewServiceHandler(svc ServiceService) *ServiceHandler { return &ServiceHandler{service: svc} }

// List serves GET /api/services (public). Mirrors perPage(1..50), category_id
// and search filters, name ordering, Laravel paginator envelope.
func (h *ServiceHandler) List(c *gin.Context) {
	page := 1
	if raw := c.Query("page"); raw != "" {
		if p, err := strconv.Atoi(raw); err == nil && p >= 1 {
			page = p
		}
	}
	perPage := model.DefaultServicePerPage
	if raw := c.Query("per_page"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n >= 1 {
			perPage = n
			if perPage > 50 {
				perPage = 50
			}
		}
	}
	var categoryID *uint64
	if raw := c.Query("category_id"); raw != "" {
		if id, err := strconv.ParseUint(raw, 10, 64); err == nil && id != 0 {
			categoryID = &id
		}
	}
	search := strings.TrimSpace(c.Query("search"))

	list, err := h.service.List(c.Request.Context(), categoryID, search, page, perPage)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, buildServicePage(c, list))
}

// Get serves GET /api/services/:id (public).
func (h *ServiceHandler) Get(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	svc, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": toServiceData(svc)})
}

// Store serves POST /api/admin/services.
func (h *ServiceHandler) Store(c *gin.Context) {
	req, ok := decodeServiceRequest(c)
	if !ok {
		return
	}
	svc, err := h.service.Create(c.Request.Context(), toServiceInput(req))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"message": "Service created successfully.",
		"data":    toServiceData(svc),
	})
}

// Update serves PUT /api/admin/services/:id.
func (h *ServiceHandler) Update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	req, ok := decodeServiceRequest(c)
	if !ok {
		return
	}
	svc, err := h.service.Update(c.Request.Context(), id, toServiceInput(req))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "Service updated successfully.",
		"data":    toServiceData(svc),
	})
}

// Destroy serves DELETE /api/admin/services/:id.
func (h *ServiceHandler) Destroy(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Service deleted successfully."})
}

func decodeServiceRequest(c *gin.Context) (serviceRequest, bool) {
	var req serviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		if errors.Is(err, errNotBoolean) {
			respondError(c, httperr.Validation(map[string][]string{
				"is_active": {"The is active field must be true or false."},
			}))
			return req, false
		}
		if errors.Is(err, errBadPrice) {
			respondError(c, httperr.Validation(map[string][]string{
				"price": {"The price field must be a number."},
			}))
			return req, false
		}
		respondError(c, httperr.BadRequest("Invalid JSON payload."))
		return req, false
	}
	if errs := validateServiceRequest(req); len(errs) > 0 {
		respondError(c, httperr.Validation(errs))
		return req, false
	}
	return req, true
}

func toServiceInput(req serviceRequest) service.ServiceInput {
	return service.ServiceInput{
		ServiceCategoryID: req.ServiceCategoryID,
		Name:              req.Name,
		Slug:              req.Slug,
		Description:       req.Description,
		PriceCents:        req.PriceCents,
		Unit:              req.Unit,
		EstimatedDuration: req.EstimatedDuration,
		IsActive:          boolVal(req.IsActive),
	}
}

var errBadPrice = errors.New("price is not numeric")

// parsePriceCents converts a JSON-numeric price into integer cents exactly
// (no float): "150", "150.00", 150, 150.5 all parse. Negative results are
// reported by the min:0 validation ("The price field must be at least 0.").
func parsePriceCents(raw json.RawMessage) (int64, error) {
	s := strings.TrimSpace(string(raw))
	if s == "" {
		return 0, errBadPrice
	}
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		return 0, errBadPrice
	}
	cents := new(big.Rat).Mul(r, big.NewRat(100, 1))
	q := new(big.Int).Quo(cents.Num(), cents.Denom())
	return q.Int64(), nil
}
