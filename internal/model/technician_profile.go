package model

import "time"

// TechnicianProfile mirrors the technician_profiles table: 1:1 with users
// (unique user_id), technician_code unique, nullable contact fields.
type TechnicianProfile struct {
	ID             uint64
	UserID         uint64
	TechnicianCode string
	Phone          *string
	Specialization *string
	Address        *string
	Bio            *string
	IsActive       bool
	CreatedAt      *time.Time
	UpdatedAt      *time.Time
}

// TechnicianCodePrefix matches Laravel generateTechnicianCode.
const TechnicianCodePrefix = "TECH-"
