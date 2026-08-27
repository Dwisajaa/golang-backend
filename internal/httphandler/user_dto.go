package httphandler

import (
	"time"

	"github.com/Dwisajaa/golang-backend/internal/model"
)

// timeMicro marshals timestamps the way Laravel's JSON serializer does:
// ISO-8601 with microseconds and UTC suffix (2026-08-27T12:34:56.123456Z).
type timeMicro struct {
	t *time.Time
}

func (m timeMicro) MarshalJSON() ([]byte, error) {
	if m.t == nil {
		return []byte("null"), nil
	}
	return []byte(`"` + m.t.UTC().Format("2006-01-02T15:04:05.000000Z07:00") + `"`), nil
}

// userResponse mirrors Laravel UserResource exactly.
type userResponse struct {
	ID              uint64    `json:"id"`
	Name            string    `json:"name"`
	Email           string    `json:"email"`
	EmailVerifiedAt timeMicro `json:"email_verified_at"`
	Role            string    `json:"role"`
}

func toUserResponse(u *model.User) userResponse {
	return userResponse{
		ID:              u.ID,
		Name:            u.Name,
		Email:           u.Email,
		EmailVerifiedAt: timeMicro{t: u.EmailVerifiedAt},
		Role:            u.Role,
	}
}
