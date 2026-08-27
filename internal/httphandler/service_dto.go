package httphandler

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/service"
)

// serviceRequest decodes the Store/UpdateServiceRequest body. Price arrives as
// a JSON number (or numeric string) and is converted to integer cents; a
// non-numeric value is a decode-level error → 422 numeric message.
type serviceRequest struct {
	ServiceCategoryID uint64          `json:"service_category_id"`
	Name              string          `json:"name"`
	Slug              string          `json:"slug"`
	Description       string          `json:"description"`
	Price             json.RawMessage `json:"price"`
	Unit              string          `json:"unit"`
	EstimatedDuration *int64          `json:"estimated_duration"`
	IsActive          *boolish        `json:"is_active"`

	PriceCents int64 `json:"-"`
}

// serviceData mirrors Laravel ServiceResource.
type serviceData struct {
	ID                uint64          `json:"id"`
	Name              string          `json:"name"`
	Slug              string          `json:"slug"`
	Description       *string         `json:"description"`
	Price             string          `json:"price"`
	Unit              string          `json:"unit"`
	EstimatedDuration *int64          `json:"estimated_duration"`
	IsActive          bool            `json:"is_active"`
	Category          *categoryNested `json:"category"`
}

// categoryNested is the CategoryResource embedded in ServiceResource (services
// always render as an empty array — the category relation is loaded without
// its services on service endpoints).
type categoryNested struct {
	ID          uint64  `json:"id"`
	Name        string  `json:"name"`
	Slug        string  `json:"slug"`
	Description *string `json:"description"`
	Icon        *string `json:"icon"`
	IsActive    bool    `json:"is_active"`
	Services    []any   `json:"services"`
}

func toServiceData(s *model.Service) serviceData {
	d := serviceData{
		ID:                s.ID,
		Name:              s.Name,
		Slug:              s.Slug,
		Description:       s.Description,
		Price:             centsToString(s.PriceCents),
		Unit:              s.Unit,
		EstimatedDuration: s.EstimatedDuration,
		IsActive:          s.IsActive,
	}
	if s.Category != nil {
		d.Category = &categoryNested{
			ID:          s.Category.ID,
			Name:        s.Category.Name,
			Slug:        s.Category.Slug,
			Description: s.Category.Description,
			Icon:        s.Category.Icon,
			IsActive:    s.Category.IsActive,
			Services:    []any{},
		}
	}
	return d
}

// validateServiceRequest mirrors Store/UpdateServiceRequest. unique + category
// exists are DB-backed (service layer).
func validateServiceRequest(req serviceRequest) map[string][]string {
	errors := map[string][]string{}
	each := func(field string, msgs ...string) {
		if len(msgs) > 0 {
			errors[field] = append(errors[field], msgs...)
		}
	}

	if req.ServiceCategoryID == 0 {
		each("service_category_id", "The service category id field is required.")
	}

	if req.Name == "" {
		each("name", fmt.Sprintf(msgRequired, "name"))
	} else if len(req.Name) > 255 {
		each("name", fmt.Sprintf(msgMax, "name", 255))
	}

	if req.Slug != "" && len(req.Slug) > 255 {
		each("slug", fmt.Sprintf(msgMax, "slug", 255))
	}

	if req.Description != "" && len(req.Description) > 1000 {
		each("description", fmt.Sprintf(msgMax, "description", 1000))
	}

	switch {
	case len(req.Price) == 0:
		each("price", "The price field is required.")
	default:
		cents, err := parsePriceCents(req.Price)
		if err != nil {
			each("price", "The price field must be a number.")
		} else if cents < 0 {
			each("price", "The price field must be at least 0.")
		} else {
			req.PriceCents = cents
		}
	}

	switch {
	case req.Unit == "":
		each("unit", "The unit field is required.")
	default:
		valid := false
		for _, u := range model.ServiceUnits {
			if req.Unit == u {
				valid = true
				break
			}
		}
		if !valid {
			each("unit", "The selected unit is invalid.")
		}
	}

	if req.EstimatedDuration != nil && *req.EstimatedDuration < 1 {
		each("estimated_duration", "The estimated duration field must be at least 1.")
	}

	if req.IsActive != nil {
		_ = *req.IsActive
	}
	return errors
}

// buildServicePage mirrors Laravel's paginated ServiceResource collection.
func buildServicePage(c *gin.Context, list *service.ServiceList) gin.H {
	path := c.Request.URL.Path
	page := list.Page
	last := 0
	if list.PerPage > 0 {
		last = (list.Total + list.PerPage - 1) / list.PerPage
	}
	from := 0
	to := 0
	if list.Total > 0 {
		from = (page-1)*list.PerPage + 1
		to = from + len(list.Items) - 1
	}
	link := func(p int) any {
		if p >= 1 && p <= last {
			q := c.Request.URL.Query()
			q.Set("page", strconv.Itoa(p))
			return path + "?" + q.Encode()
		}
		return nil
	}

	data := make([]serviceData, 0, len(list.Items))
	for _, item := range list.Items {
		data = append(data, toServiceData(item))
	}
	return gin.H{
		"data": data,
		"links": gin.H{
			"first": link(1),
			"last":  link(last),
			"prev":  link(page - 1),
			"next":  link(page + 1),
		},
		"meta": gin.H{
			"current_page": page,
			"from":         from,
			"last_page":    last,
			"path":         path,
			"per_page":     list.PerPage,
			"to":           to,
			"total":        list.Total,
		},
	}
}
