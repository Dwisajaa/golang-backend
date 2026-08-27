package model

import "time"

// Role values mirror Laravel User role constants.
const (
	RoleCustomer   = "customer"
	RoleTechnician = "technician"
	RoleAdmin      = "admin"
	RoleSuperAdmin = "super_admin"
)

// User is the 1:1 persistence shape of the users table. Password is loaded by
// the repository (needed later for login) but never serialized to JSON.
type User struct {
	ID              uint64
	Name            string
	Email           string
	Role            string
	EmailVerifiedAt *time.Time
	Password        string
	RememberToken   *string
	CreatedAt       *time.Time
	UpdatedAt       *time.Time
}
