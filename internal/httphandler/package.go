package httphandler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Dwisajaa/golang-backend/internal/httperr"
	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/service"
)

type PackageService interface {
	List(ctx context.Context, search string, page, perPage int) (*service.PackageList, error)
	Get(ctx context.Context, id uint64) (*model.Package, error)
	Create(ctx context.Context, in service.PackageInput) (*model.Package, error)
	Update(ctx context.Context, id uint64, in service.PackageInput) (*model.Package, error)
	Delete(ctx context.Context, id uint64) error
}

type PackageHandler struct{ service PackageService }

func NewPackageHandler(svc PackageService) *PackageHandler { return &PackageHandler{service: svc} }

func (h *PackageHandler) List(c *gin.Context) {
	page, perPage := 1, model.DefaultPackagePerPage
	if raw := c.Query("page"); raw != "" {
		if p, err := strconv.Atoi(raw); err == nil && p >= 1 {
			page = p
		}
	}
	if raw := c.Query("per_page"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n >= 1 {
			perPage = n
			if perPage > 50 {
				perPage = 50
			}
		}
	}
	search := strings.TrimSpace(c.Query("search"))

	list, err := h.service.List(c.Request.Context(), search, page, perPage)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, buildPackagePage(c, list))
}

func (h *PackageHandler) Get(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	p, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": toPackageData(p)})
}

func (h *PackageHandler) Store(c *gin.Context) {
	req, ok := decodePackageRequest(c)
	if !ok {
		return
	}
	p, err := h.service.Create(c.Request.Context(), toPackageInput(req))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "Package created successfully.", "data": toPackageData(p)})
}

func (h *PackageHandler) Update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	req, ok := decodePackageRequest(c)
	if !ok {
		return
	}
	p, err := h.service.Update(c.Request.Context(), id, toPackageInput(req))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Package updated successfully.", "data": toPackageData(p)})
}

func (h *PackageHandler) Destroy(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Package deleted successfully."})
}

func decodePackageRequest(c *gin.Context) (packageRequest, bool) {
	var req packageRequest
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
	if errs := validatePackageRequest(req); len(errs) > 0 {
		respondError(c, httperr.Validation(errs))
		return req, false
	}
	return req, true
}

func toPackageInput(req packageRequest) service.PackageInput {
	items := make([]model.PackageItemInput, len(req.Items))
	for i, it := range req.Items {
		items[i] = model.PackageItemInput{ServiceID: it.ServiceID, Quantity: it.Quantity}
	}
	return service.PackageInput{
		Name: req.Name, Slug: req.Slug, Description: req.Description,
		PriceCents: req.PriceCents, Duration: req.Duration,
		IsActive: boolVal(req.IsActive), IsPopular: boolVal(req.IsPopular),
		Items: items,
	}
}
