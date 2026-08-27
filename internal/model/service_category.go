package model

import "time"

// ServiceCategory mirrors the service_categories table. Description/Icon are
// nullable; IsActive defaults to true at insert.
type ServiceCategory struct {
	ID          uint64
	Name        string
	Slug        string
	Description *string
	Icon        *string
	IsActive    bool
	CreatedAt   *time.Time
	UpdatedAt   *time.Time
	// Services is populated by the catalog list query (active services,
	// ordered by name) — a read-only projection until the Service domain lands.
	Services []*ServiceLite
}

// ServiceLite is the minimal service shape embedded in the category list
// response (mirrors ServiceResource's fields surfaced by categories()).
// PriceCents is DECIMAL(12,2) as integer smallest-unit, serialized by the DTO.
type ServiceLite struct {
	ID                uint64
	Name              string
	Slug              string
	Description       *string
	PriceCents        int64
	Unit              string
	EstimatedDuration *int64
	IsActive          bool
}

// DefaultCategoryPerPage mirrors Laravel's perPage() default (15, clamped
// 1..50); note categories() ignores per_page and always uses this default.
const DefaultCategoryPerPage = 15
