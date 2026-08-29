package model

import "time"

// Package mirrors the packages table. Price as integer cents (DECIMAL boundary).
type Package struct {
	ID          uint64
	Name        string
	Slug        string
	Description *string
	PriceCents  int64
	Duration    *int64
	IsActive    bool
	IsPopular   bool
	CreatedAt   *time.Time
	UpdatedAt   *time.Time
	Items       []*PackageItem
}

// PackageItem mirrors the package_items table (pivot Package ↔ Service).
type PackageItem struct {
	ID        uint64
	PackageID uint64
	ServiceID uint64
	Quantity  int
	CreatedAt *time.Time
	UpdatedAt *time.Time
	Service   *Service // loaded for API responses (category nil → JSON null)
}

// PackageItemInput is the incoming {service_id, quantity} from requests.
type PackageItemInput struct {
	ServiceID uint64
	Quantity  int
}

// DefaultPackagePerPage mirrors CatalogController perPage() default.
const DefaultPackagePerPage = 15
