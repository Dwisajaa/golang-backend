package httphandler

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Dwisajaa/golang-backend/internal/model"
	"github.com/Dwisajaa/golang-backend/internal/service"
)

type packageRequest struct {
	Name        string          `json:"name"`
	Slug        string          `json:"slug"`
	Description string          `json:"description"`
	Price       json.RawMessage `json:"price"`
	Duration    *int64          `json:"duration"`
	IsActive    *boolish        `json:"is_active"`
	IsPopular   *boolish        `json:"is_popular"`
	Items       []packageItemIn `json:"items"`

	PriceCents int64 `json:"-"`
}

type packageItemIn struct {
	ServiceID uint64 `json:"service_id"`
	Quantity  int    `json:"quantity"`
}

// packageData mirrors PackageResource.
type packageData struct {
	ID          uint64        `json:"id"`
	Name        string        `json:"name"`
	Slug        string        `json:"slug"`
	Description *string       `json:"description"`
	Price       string        `json:"price"`
	Duration    *int64        `json:"duration"`
	IsActive    bool          `json:"is_active"`
	IsPopular   bool          `json:"is_popular"`
	Items       []pkgItemData `json:"items"`
}

// pkgItemData mirrors PackageItemResource.
type pkgItemData struct {
	ID       uint64      `json:"id"`
	Quantity int         `json:"quantity"`
	Service  serviceData `json:"service"`
}

func toPackageData(p *model.Package) packageData {
	items := make([]pkgItemData, 0, len(p.Items))
	for _, it := range p.Items {
		svcData := serviceData{}
		if it.Service != nil {
			svcData = toServiceData(it.Service)
		}
		items = append(items, pkgItemData{ID: it.ID, Quantity: it.Quantity, Service: svcData})
	}
	return packageData{
		ID: p.ID, Name: p.Name, Slug: p.Slug, Description: p.Description,
		Price: centsToString(p.PriceCents), Duration: p.Duration,
		IsActive: p.IsActive, IsPopular: p.IsPopular, Items: items,
	}
}

func validatePackageRequest(req packageRequest) map[string][]string {
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

	if req.Duration != nil && *req.Duration < 1 {
		each("duration", "The duration field must be at least 1.")
	}

	if len(req.Items) == 0 {
		each("items", "The items field is required.")
	} else {
		for i, it := range req.Items {
			prefix := fmt.Sprintf("items.%d.", i)
			if it.ServiceID == 0 {
				each(prefix+"service_id", "The "+prefix+"service_id field is required.")
			}
			if it.Quantity < 1 {
				each(prefix+"quantity", "The "+prefix+"quantity field must be at least 1.")
			}
		}
	}
	return errors
}

func buildPackagePage(c *gin.Context, list *service.PackageList) gin.H {
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
			q := c.Request.URL.Query()
			q.Set("page", strconv.Itoa(p))
			return path + "?" + q.Encode()
		}
		return nil
	}
	data := make([]packageData, 0, len(list.Items))
	for _, item := range list.Items {
		data = append(data, toPackageData(item))
	}
	return gin.H{
		"data":  data,
		"links": gin.H{"first": link(1), "last": link(last), "prev": link(page - 1), "next": link(page + 1)},
		"meta":  gin.H{"current_page": page, "from": from, "last_page": last, "path": path, "per_page": list.PerPage, "to": to, "total": list.Total},
	}
}
