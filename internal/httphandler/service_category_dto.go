package httphandler

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/service"
)

type categoryRequest struct {
	Name        string   `json:"name"`
	Slug        string   `json:"slug"`
	Description string   `json:"description"`
	Icon        string   `json:"icon"`
	IsActive    *boolish `json:"is_active"`
}

// boolish accepts only JSON booleans, so a non-boolean is_active can produce
// Laravel's boolean-rule 422 message instead of a generic 400.
type boolish bool

var errNotBoolean = errors.New("value is not a boolean")

func (b *boolish) UnmarshalJSON(data []byte) error {
	switch string(data) {
	case "true":
		*b = true
		return nil
	case "false":
		*b = false
		return nil
	}
	return errNotBoolean
}

func boolVal(b *boolish) *bool {
	if b == nil {
		return nil
	}
	v := bool(*b)
	return &v
}

// categoryData mirrors Laravel CategoryResource.
type categoryData struct {
	ID          uint64            `json:"id"`
	Name        string            `json:"name"`
	Slug        string            `json:"slug"`
	Description *string           `json:"description"`
	Icon        *string           `json:"icon"`
	IsActive    bool              `json:"is_active"`
	Services    []serviceLiteData `json:"services"`
}

// serviceLiteData mirrors ServiceResource as embedded by categories();
// `category` is always null (services are loaded without the category relation).
type serviceLiteData struct {
	ID                uint64  `json:"id"`
	Name              string  `json:"name"`
	Slug              string  `json:"slug"`
	Description       *string `json:"description"`
	Price             string  `json:"price"` // DECIMAL(12,2) string (Laravel decimal:2 cast)
	Unit              string  `json:"unit"`
	EstimatedDuration *int64  `json:"estimated_duration"`
	IsActive          bool    `json:"is_active"`
	Category          *any    `json:"category"`
}

func toCategoryData(c *model.ServiceCategory) categoryData {
	services := make([]serviceLiteData, 0, len(c.Services))
	for _, svc := range c.Services {
		services = append(services, serviceLiteData{
			ID:                svc.ID,
			Name:              svc.Name,
			Slug:              svc.Slug,
			Description:       svc.Description,
			Price:             centsToString(svc.PriceCents),
			Unit:              svc.Unit,
			EstimatedDuration: svc.EstimatedDuration,
			IsActive:          svc.IsActive,
			Category:          nil,
		})
	}
	return categoryData{
		ID:          c.ID,
		Name:        c.Name,
		Slug:        c.Slug,
		Description: c.Description,
		Icon:        c.Icon,
		IsActive:    c.IsActive,
		Services:    services,
	}
}

// centsToString formats the integer-cent money as a two-decimal string,
// matching Laravel's decimal:2 cast serialization.
func centsToString(cents int64) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	return fmt.Sprintf("%s%d.%02d", sign, cents/100, cents%100)
}

// validateCategoryRequest mirrors Store/UpdateCategoryRequest (unique checks
// are DB-backed and live in the service).
func validateCategoryRequest(req categoryRequest) map[string][]string {
	errors := map[string][]string{}
	each := func(field string, msgs ...string) {
		if len(msgs) > 0 {
			errors[field] = append(errors[field], msgs...)
		}
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
	if req.Icon != "" && len(req.Icon) > 100 {
		each("icon", fmt.Sprintf(msgMax, "icon", 100))
	}
	if req.IsActive != nil {
		// type correctness guaranteed by boolish
		_ = *req.IsActive
	}
	return errors
}

// buildCategoryPage mirrors Laravel's paginated resource envelope:
//
//	{data:[...], links:{first,last,prev,next}, meta:{current_page,from,last_page,path,per_page,to,total}}
func buildCategoryPage(c *gin.Context, list *service.CategoryList) gin.H {
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
			return path + "?page=" + strconv.Itoa(p)
		}
		return nil
	}

	data := make([]categoryData, 0, len(list.Items))
	for _, item := range list.Items {
		data = append(data, toCategoryData(item))
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
