package httphandler

import (
	"fmt"
)

type updateProfileRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type updatePasswordRequest struct {
	CurrentPassword      string `json:"current_password"`
	Password             string `json:"password"`
	PasswordConfirmation string `json:"password_confirmation"`
}

// profileResponse mirrors ProfileController@update.
type profileResponse struct {
	Message string       `json:"message"`
	User    userResponse `json:"user"`
}

// validateUpdateProfile mirrors Laravel UpdateProfileRequest. Unique-email is a
// database-backed check handled by the service (ignore-self semantics).
func validateUpdateProfile(req updateProfileRequest) map[string][]string {
	errors := map[string][]string{}
	each := func(field string, msgs ...string) {
		if len(msgs) > 0 {
			errors[field] = append(errors[field], msgs...)
		}
	}

	switch {
	case req.Name == "":
		each("name", fmt.Sprintf(msgRequired, "name"))
	case len(req.Name) > nameMaxLen:
		each("name", fmt.Sprintf(msgMax, "name", nameMaxLen))
	}

	switch {
	case req.Email == "":
		each("email", fmt.Sprintf(msgRequired, "email"))
	case !emailRE.MatchString(req.Email):
		each("email", msgEmailValid)
	case len(req.Email) > emailMaxLen:
		each("email", fmt.Sprintf(msgMax, "email", emailMaxLen))
	}
	return errors
}

// validateUpdatePassword mirrors Laravel UpdatePasswordRequest. The
// current_password match check is a business rule done in the service.
func validateUpdatePassword(req updatePasswordRequest) map[string][]string {
	errors := map[string][]string{}
	each := func(field string, msgs ...string) {
		if len(msgs) > 0 {
			errors[field] = append(errors[field], msgs...)
		}
	}

	if req.CurrentPassword == "" {
		each("current_password", "The current password field is required.")
	}

	switch {
	case req.Password == "":
		each("password", fmt.Sprintf(msgRequired, "password"))
	case len(req.Password) < passwordMin:
		each("password", msgPasswordMin)
	}

	if req.Password != "" && req.Password != req.PasswordConfirmation {
		each("password", msgConfirmation)
	}
	return errors
}
