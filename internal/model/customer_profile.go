package model

import "time"

// CustomerProfile mirrors the customer_profiles table (1:1 with users via the
// unique user_id). PostalCode is nullable.
type CustomerProfile struct {
	ID         uint64
	UserID     uint64
	FullName   string
	Phone      string
	Address    string
	City       string
	PostalCode *string
	CreatedAt  *time.Time
	UpdatedAt  *time.Time
}

// IsComplete mirrors Laravel CustomerProfile::isComplete(): the required
// profile fields are all non-empty. (The profile write path makes all four
// required, so a persisted row is complete by construction.)
func (p *CustomerProfile) IsComplete() bool {
	return p.FullName != "" && p.Phone != "" && p.Address != "" && p.City != ""
}
