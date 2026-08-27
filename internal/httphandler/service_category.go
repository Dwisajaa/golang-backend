package httphandler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Dwisajaa/golang-backend/internal/httperr"
	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/service"
)

// ServiceCategoryService is the surface the category handlers consume.
type ServiceCategoryService interface {
	ListCategories(ctx context.Context, page int) (*service.CategoryList, error)
	Create(ctx context.Context, in service.CategoryInput) (*model.ServiceCategory, error)
	Update(ctx context.Context, id uint64, in service.CategoryInput) (*model.ServiceCategory, error)
	Delete(ctx context.Context, id uint64) error
}

type ServiceCategoryHandler struct {
	service ServiceCategoryService
}

func NewServiceCategoryHandler(svc ServiceCategoryService) *ServiceCategoryHandler {
	return &ServiceCategoryHandler{service: svc}
}

// List serves GET /api/categories (public), mirroring Laravel's paginated
// CategoryResource::collection response.
func (h *ServiceCategoryHandler) List(c *gin.Context) {
	page := 1
	if raw := c.Query("page"); raw != "" {
		if p, err := strconv.Atoi(raw); err == nil && p >= 1 {
			page = p
		}
	}

	list, err := h.service.ListCategories(c.Request.Context(), page)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, buildCategoryPage(c, list))
}

// Store serves POST /api/admin/categories.
func (h *ServiceCategoryHandler) Store(c *gin.Context) {
	req, ok := decodeCategoryRequest(c)
	if !ok {
		return
	}

	cat, err := h.service.Create(c.Request.Context(), toCategoryInput(req))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"message": "Category created successfully.",
		"data":    toCategoryData(cat),
	})
}

// Update serves PUT /api/admin/categories/:id.
func (h *ServiceCategoryHandler) Update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	req, ok := decodeCategoryRequest(c)
	if !ok {
		return
	}

	cat, err := h.service.Update(c.Request.Context(), id, toCategoryInput(req))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "Category updated successfully.",
		"data":    toCategoryData(cat),
	})
}

// Destroy serves DELETE /api/admin/categories/:id.
func (h *ServiceCategoryHandler) Destroy(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Category deleted successfully."})
}

func decodeCategoryRequest(c *gin.Context) (categoryRequest, bool) {
	var req categoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		if errors.Is(err, errNotBoolean) {
			respondError(c, httperr.Validation(map[string][]string{
				"is_active": {"The is active field must be true or false."},
			}))
			return req, false
		}
		respondError(c, httperr.BadRequest("Invalid JSON payload."))
		return req, false
	}
	if errs := validateCategoryRequest(req); len(errs) > 0 {
		respondError(c, httperr.Validation(errs))
		return req, false
	}
	return req, true
}

func parseID(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		respondError(c, httperr.NotFound("Resource not found."))
		return 0, false
	}
	return id, true
}

func toCategoryInput(req categoryRequest) service.CategoryInput {
	return service.CategoryInput{
		Name:        req.Name,
		Slug:        req.Slug,
		Description: req.Description,
		Icon:        req.Icon,
		IsActive:    boolVal(req.IsActive),
	}
}
