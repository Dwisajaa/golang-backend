package httphandler

import (
	"fmt"
	"regexp"
)

const (
	msgRequired     = "The %s field is required."
	msgMax          = "The %s field must not be greater than %d characters."
	msgEmailValid   = "The email field must be a valid email address."
	msgPasswordMin  = "The password field must be at least 8 characters."
	msgConfirmation = "The password confirmation does not match."

	emailMaxLen = 255
	nameMaxLen  = 255
	passwordMin = 8
)

var emailRE = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

type registerRequest struct {
	Name                 string `json:"name"`
	Email                string `json:"email"`
	Password             string `json:"password"`
	PasswordConfirmation string `json:"password_confirmation"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// registerResponse mirrors AuthController@register exactly.
type registerResponse struct {
	Message              string       `json:"message"`
	RequiresVerification bool         `json:"requires_verification"`
	User                 userResponse `json:"user"`
}

// loginResponse mirrors AuthController@login success.
type loginResponse struct {
	Message   string       `json:"message"`
	User      userResponse `json:"user"`
	Token     string       `json:"token"`
	TokenType string       `json:"token_type"`
}

// unverifiedResponse mirrors the 403 body for unverified email.
type unverifiedResponse struct {
	Message              string             `json:"message"`
	RequiresVerification bool               `json:"requires_verification"`
	User                 unverifiedUserData `json:"user"`
}

type unverifiedUserData struct {
	ID              uint64    `json:"id"`
	Name            string    `json:"name"`
	Email           string    `json:"email"`
	EmailVerifiedAt timeMicro `json:"email_verified_at"`
}

// validateRegister mirrors the Laravel RegisterRequest rules. Unique-email is a
// database-backed check and is done by the service instead.
func validateRegister(req registerRequest) map[string][]string {
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

// validateLogin mirrors the Laravel LoginRequest rules.
func validateLogin(req loginRequest) map[string][]string {
	errors := map[string][]string{}
	each := func(field string, msgs ...string) {
		if len(msgs) > 0 {
			errors[field] = append(errors[field], msgs...)
		}
	}

	if req.Email == "" {
		each("email", fmt.Sprintf(msgRequired, "email"))
	} else if !emailRE.MatchString(req.Email) {
		each("email", msgEmailValid)
	}

	if req.Password == "" {
		each("password", fmt.Sprintf(msgRequired, "password"))
	}
	return errors
}
