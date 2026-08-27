package model

import "time"

// TokenableType mirrors Laravel's polymorphic morph name for User.
const TokenableType = "App\\Models\\User"

// DefaultTokenLifetime is Sanctum's default SANCTUM_TOKEN_EXPIRATION (min).
const DefaultTokenLifetime = 10080 * time.Minute // 7 days

// PersonalAccessToken mirrors personal_access_tokens. Token holds the SHA-256
// hash (never the raw bearer token).
type PersonalAccessToken struct {
	ID            uint64
	TokenableType string
	TokenableID   uint64
	Name          string
	Token         string // sha256 hash
	Abilities     *string
	LastUsedAt    *time.Time
	ExpiresAt     *time.Time
	CreatedAt     *time.Time
	UpdatedAt     *time.Time
}
