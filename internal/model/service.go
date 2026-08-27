package model

import "time"

// Service unit values mirror Service::UNIT_VALUES (StoreServiceRequest `in`).
var ServiceUnits = []string{"per_service", "per_room", "per_unit", "per_hour", "per_meter", "custom"}

// Service mirrors the services table. Price is stored as integer cents
// (DECIMAL(12,2) boundary); serialization happens at the DTO layer as a
// two-decimal string ("150.00") per Laravel's decimal:2 cast.
type Service struct {
	ID                uint64
	ServiceCategoryID uint64
	Name              string
	Slug              string
	Description       *string
	PriceCents        int64
	Unit              string
	EstimatedDuration *int64
	IsActive          bool
	CreatedAt         *time.Time
	UpdatedAt         *time.Time
	// Category is loaded by public service endpoints (list/detail). Its
	// Services slice stays nil (CategoryResource renders services: []).
	Category *ServiceCategory
}

// DefaultServicePerPage mirrors CatalogController perPage() default.
const DefaultServicePerPage = 15
