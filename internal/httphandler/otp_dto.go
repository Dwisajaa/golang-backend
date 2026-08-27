package httphandler

import (
	"fmt"
	"regexp"
)

const (
	msgOtpRequired = "The otp field is required."
	msgOtpDigits   = "The otp field must be 6 digits."
)

var otpDigitsRE = regexp.MustCompile(`^\d{6}$`)

// resendRequest mirrors Laravel ResendVerificationRequest.
type resendRequest struct {
	Email string `json:"email"`
}

// verifyEmailRequest mirrors Laravel VerifyEmailRequest.
type verifyEmailRequest struct {
	Email string `json:"email"`
	Otp   string `json:"otp"`
}

func validateResend(req resendRequest) map[string][]string {
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
	return errors
}

// validateVerifyEmail mirrors Laravel VerifyEmailRequest.
func validateVerifyEmail(req verifyEmailRequest) map[string][]string {
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
	switch {
	case req.Otp == "":
		each("otp", msgOtpRequired)
	case !otpDigitsRE.MatchString(req.Otp):
		each("otp", msgOtpDigits)
	}
	return errors
}
