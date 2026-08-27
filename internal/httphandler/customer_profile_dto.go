package httphandler

import (
	"fmt"

	"github.com/Dwisajaa/golang-backend/internal/model"
)

type updateCustomerProfileRequest struct {
	FullName   string `json:"full_name"`
	Phone      string `json:"phone"`
	Address    string `json:"address"`
	City       string `json:"city"`
	PostalCode string `json:"postal_code"`
}

// customerProfileData mirrors Laravel CustomerProfileResource exactly.
type customerProfileData struct {
	ID         uint64  `json:"id"`
	UserID     uint64  `json:"user_id"`
	FullName   string  `json:"full_name"`
	Phone      string  `json:"phone"`
	Address    string  `json:"address"`
	City       string  `json:"city"`
	PostalCode *string `json:"postal_code"`
	IsComplete bool    `json:"is_complete"`
}

// customerProfileResponse mirrors the update (write) response.
type customerProfileResponse struct {
	Message string              `json:"message"`
	Data    customerProfileData `json:"data"`
}

func toCustomerProfile(p *model.CustomerProfile) customerProfileData {
	return customerProfileData{
		ID:         p.ID,
		UserID:     p.UserID,
		FullName:   p.FullName,
		Phone:      p.Phone,
		Address:    p.Address,
		City:       p.City,
		PostalCode: p.PostalCode,
		IsComplete: p.IsComplete(),
	}
}

// validateUpdateCustomerProfile mirrors Laravel UpdateCustomerProfileRequest.
func validateUpdateCustomerProfile(req updateCustomerProfileRequest) map[string][]string {
	errors := map[string][]string{}
	each := func(field string, msgs ...string) {
		if len(msgs) > 0 {
			errors[field] = append(errors[field], msgs...)
		}
	}

	switch {
	case req.FullName == "":
		each("full_name", fmt.Sprintf(msgRequired, "full name"))
	case len(req.FullName) > 255:
		each("full_name", fmt.Sprintf(msgMax, "full name", 255))
	}

	switch {
	case req.Phone == "":
		each("phone", fmt.Sprintf(msgRequired, "phone"))
	case len(req.Phone) > 20:
		each("phone", fmt.Sprintf(msgMax, "phone", 20))
	}

	switch {
	case req.Address == "":
		each("address", fmt.Sprintf(msgRequired, "address"))
	case len(req.Address) > 255:
		each("address", fmt.Sprintf(msgMax, "address", 255))
	}

	switch {
	case req.City == "":
		each("city", fmt.Sprintf(msgRequired, "city"))
	case len(req.City) > 100:
		each("city", fmt.Sprintf(msgMax, "city", 100))
	}

	if req.PostalCode != "" && len(req.PostalCode) > 10 {
		each("postal_code", fmt.Sprintf(msgMax, "postal code", 10))
	}
	return errors
}
