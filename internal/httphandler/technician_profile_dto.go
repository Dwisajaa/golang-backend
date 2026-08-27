package httphandler

import (
	"fmt"

	"github.com/Dwisajaa/golang-backend/internal/model"
)

type updateTechnicianProfileRequest struct {
	Phone          string `json:"phone"`
	Specialization string `json:"specialization"`
	Address        string `json:"address"`
	Bio            string `json:"bio"`
}

// technicianProfileData mirrors Laravel TechnicianProfileResource exactly.
type technicianProfileData struct {
	ID             uint64  `json:"id"`
	TechnicianCode string  `json:"technician_code"`
	Phone          *string `json:"phone"`
	Specialization *string `json:"specialization"`
	Address        *string `json:"address"`
	Bio            *string `json:"bio"`
	IsActive       bool    `json:"is_active"`
}

type technicianProfileResponse struct {
	Message string                `json:"message"`
	Data    technicianProfileData `json:"data"`
}

func toTechnicianProfile(p *model.TechnicianProfile) technicianProfileData {
	return technicianProfileData{
		ID:             p.ID,
		TechnicianCode: p.TechnicianCode,
		Phone:          p.Phone,
		Specialization: p.Specialization,
		Address:        p.Address,
		Bio:            p.Bio,
		IsActive:       p.IsActive,
	}
}

// validateUpdateTechnicianProfile mirrors UpdateTechnicianProfileRequest.
func validateUpdateTechnicianProfile(req updateTechnicianProfileRequest) map[string][]string {
	errors := map[string][]string{}
	each := func(field string, msgs ...string) {
		if len(msgs) > 0 {
			errors[field] = append(errors[field], msgs...)
		}
	}
	if req.Phone != "" && len(req.Phone) > 20 {
		each("phone", fmt.Sprintf(msgMax, "phone", 20))
	}
	if req.Specialization != "" && len(req.Specialization) > 255 {
		each("specialization", fmt.Sprintf(msgMax, "specialization", 255))
	}
	if req.Address != "" && len(req.Address) > 255 {
		each("address", fmt.Sprintf(msgMax, "address", 255))
	}
	if req.Bio != "" && len(req.Bio) > 2000 {
		each("bio", fmt.Sprintf(msgMax, "bio", 2000))
	}
	return errors
}
